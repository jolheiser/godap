package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strconv"
	"time"

	"github.com/Macmod/godap/utils"
	"github.com/gdamore/tcell/v2"
	"github.com/go-ldap/ldap/v3"
	"github.com/rivo/tview"
)

var explorerPage *tview.Flex
var treePanel *tview.TreeView
var attrsPanel *tview.Table
var rootDNInput *tview.InputField
var searchFilterInput *tview.InputField

func InitExplorerPage() {
	treePanel = tview.NewTreeView()

	rootNode = renderPartialTree(rootDN, searchFilter)
	treePanel.SetRoot(rootNode).SetCurrentNode(rootNode)

	attrsPanel = tview.NewTable()
	attrsPanel.SetSelectable(true, true)

	searchFilterInput = tview.NewInputField().
		SetText(searchFilter)
	searchFilterInput.SetFieldBackgroundColor(tcell.GetColor("black"))
	searchFilterInput.SetTitle("Search Filter (Single-Level)")
	searchFilterInput.SetBorder(true)

	rootDNInput = tview.NewInputField().
		SetText(rootDN)
	rootDNInput.SetFieldBackgroundColor(tcell.GetColor("black"))
	rootDNInput.SetTitle("Root DN")
	rootDNInput.SetBorder(true)

	attrsPanel.SetBorder(true)
	attrsPanel.SetTitle("Attributes")

	// Event Handlers
	searchFilterInput.SetDoneFunc(func(key tcell.Key) {
		searchFilter = searchFilterInput.GetText()
		reloadPage()
	})

	rootDNInput.SetDoneFunc(func(key tcell.Key) {
		rootDN = rootDNInput.GetText()
		reloadPage()
	})

	treeFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(
			tview.NewFlex().
				AddItem(searchFilterInput, 0, 1, false).
				AddItem(rootDNInput, 0, 1, false),
			3, 0, false,
		).
		AddItem(treePanel, 0, 8, false)

	treeFlex.SetBorder(true)
	treeFlex.SetTitle("Tree View")
	explorerPage = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(
			tview.NewFlex().
				AddItem(treeFlex, 0, 1, false).
				AddItem(attrsPanel, 0, 1, false), 0, 8, false,
		)

	explorerPage.SetInputCapture(explorerPageKeyHandler)

	attrsPanel.SetInputCapture(attrsPanelKeyHandler)

	treePanel.SetInputCapture(treePanelKeyHandler)

	treePanel.SetChangedFunc(treePanelChangeHandler)
}

func expandTreeNode(node *tview.TreeNode) {
	if !node.IsExpanded() {
		//experiment: go loadChildren(node)
		loadChildren(node)
		node.SetExpanded(true)
	}
}

func collapseTreeNode(node *tview.TreeNode) {
	node.SetExpanded(false)
	if !cacheEntries {
		unloadChildren(node)
	}
}

func reloadParentNode(node *tview.TreeNode) *tview.TreeNode {
	pathToCurrent := treePanel.GetPath(node)
	parent := pathToCurrent[len(pathToCurrent)-2]
	collapseTreeNode(parent)
	expandTreeNode(parent)

	return parent
}

func findLocationInSiblings(node *tview.TreeNode) int {
	pathToCurrent := treePanel.GetPath(node)
	parent := pathToCurrent[len(pathToCurrent)-2]
	siblings := parent.GetChildren()

	for idx, loopNode := range siblings {
		if loopNode.GetReference().(string) == node.GetReference().(string) {
			return idx
		}
	}

	return -1
}

func treePanelKeyHandler(event *tcell.EventKey) *tcell.EventKey {
	currentNode := treePanel.GetCurrentNode()
	if currentNode == nil {
		return event
	}

	switch event.Key() {
	case tcell.KeyRight:
		expandTreeNode(currentNode)
	case tcell.KeyLeft:
		if currentNode.IsExpanded() { // Collapse current node
			collapseTreeNode(currentNode)
			treePanel.SetCurrentNode(currentNode)
			return nil
		} else { // Collapse parent node
			pathToCurrent := treePanel.GetPath(currentNode)
			if len(pathToCurrent) > 1 {
				parentNode := pathToCurrent[len(pathToCurrent)-2]
				collapseTreeNode(parentNode)
				treePanel.SetCurrentNode(parentNode)
			}
			return nil
		}
	case tcell.KeyDelete:
		baseDN := currentNode.GetReference().(string)
		promptModal := tview.NewModal().
			SetText("Do you really want to delete this object?\n" + baseDN).
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				if buttonLabel == "Yes" {
					err := lc.DeleteObject(baseDN)
					if err == nil {
						delete(loadedDNs, baseDN)
						updateLog("Object deleted: "+baseDN, "green")

						idx := findLocationInSiblings(currentNode)

						parent := reloadParentNode(currentNode)
						otherNodeToSelect := parent

						if idx > 0 {
							siblings := parent.GetChildren()
							otherNodeToSelect = siblings[idx-1]
						}

						treePanel.SetCurrentNode(otherNodeToSelect)
					}
				}

				app.SetRoot(appPanel, true).SetFocus(treePanel)
			})

		app.SetRoot(promptModal, false).SetFocus(promptModal)
	case tcell.KeyCtrlN:
		baseDN := currentNode.GetReference().(string)
		createObjectForm := tview.NewForm().
			AddDropDown("Object Type", []string{"OrganizationalUnit", "Container", "User", "Group", "Computer"}, 0, nil).
			AddInputField("Object Name", "", 0, nil, nil).
			AddInputField("Parent DN", baseDN, 0, nil, nil)

		createObjectForm.
			AddButton("Create", func() {
				// Note: It should be possible to walk upwards in the tree
				//   to find the first place where it's possible to place the object
				//   but it makes sense that the user should
				//   have full control over this behavior
				//   rather than automatically detecting
				//   an appropriate DN

				// pathToCurrent := treePanel.GetPath(currentNode)
				// lastNode := len(pathToCurrent) - 1
				// for nodeInPathIdx := range pathToCurrent {
				//   currentNodeIdx := lastNode - nodeInPathIdx
				// }

				_, objectType := createObjectForm.GetFormItemByLabel("Object Type").(*tview.DropDown).GetCurrentOption()

				objectName := createObjectForm.GetFormItemByLabel("Object Name").(*tview.InputField).GetText()

				var err error = nil

				switch objectType {
				case "OrganizationalUnit":
					err = lc.AddOrganizationalUnit(objectName, baseDN)
				case "Container":
					err = lc.AddContainer(objectName, baseDN)
				case "User":
					err = lc.AddUser(objectName, baseDN)
				case "Group":
					err = lc.AddGroup(objectName, baseDN)
				case "Computer":
					err = lc.AddComputer(objectName, baseDN)
				}

				if err != nil {
					updateLog(fmt.Sprintf("%s", err), "red")
				} else {
					updateLog("Object created successfully at: "+baseDN, "green")
				}

				reloadAttributesPanel(currentNode, cacheEntries)

				// Not the best approach but for now it works :)
				collapseTreeNode(currentNode)
				expandTreeNode(currentNode)
				treePanel.SetCurrentNode(currentNode)

				app.SetRoot(appPanel, true).SetFocus(treePanel)
			}).
			AddButton("Cancel", func() {
				app.SetRoot(appPanel, true).SetFocus(treePanel)
			})

		createObjectForm.SetTitle("Object Creator").SetBorder(true)
		app.SetRoot(createObjectForm, true).SetFocus(createObjectForm)
	case tcell.KeyCtrlS:
		exportMap := make(map[string]*ldap.Entry)
		currentNode.Walk(func(node, parent *tview.TreeNode) bool {
			nodeDN := node.GetReference().(string)
			exportMap[nodeDN] = loadedDNs[nodeDN]
			return true
		})

		jsonExportMap, _ := json.MarshalIndent(exportMap, "", " ")

		unixTimestamp := time.Now().Unix()

		outputFilename := fmt.Sprintf("%d_objects.json", unixTimestamp)

		err := ioutil.WriteFile(outputFilename, jsonExportMap, 0644)

		if err != nil {
			updateLog(fmt.Sprintf("%s", err), "red")
		} else {
			updateLog("File '"+outputFilename+"' saved successfully!", "green")
		}
	case tcell.KeyCtrlA:
		baseDN := currentNode.GetReference().(string)

		updateUacForm := tview.NewForm()
		updateUacForm.SetItemPadding(0)

		var checkboxState int = 0
		if loadedDNs[baseDN] != nil {
			uacValue, err := strconv.Atoi(loadedDNs[baseDN].GetAttributeValue("userAccountControl"))
			if err == nil {
				checkboxState = uacValue
			} else {
				return nil
			}
		}

		updateUacForm.
			AddTextView("Raw UAC Value", strconv.Itoa(checkboxState), 0, 1, false, true)

		uacValues := make([]int, 0)
		for key, _ := range utils.UacFlags {
			uacValues = append(uacValues, key)
		}
		sort.Ints(uacValues)

		for _, val := range uacValues {
			uacValue := val
			updateUacForm.AddCheckbox(
				utils.UacFlags[uacValue].Present,
				checkboxState&uacValue != 0,
				func(checked bool) {
					if checked {
						checkboxState |= uacValue
					} else {
						checkboxState &^= uacValue
					}

					uacPreview := updateUacForm.GetFormItemByLabel("Raw UAC Value").(*tview.TextView)
					if uacPreview != nil {
						uacPreview.SetText(strconv.Itoa(checkboxState))
					}
				},
			)
		}

		updateUacForm.
			AddButton("Update", func() {
				strCheckboxState := strconv.Itoa(checkboxState)
				err := lc.ModifyAttribute(baseDN, "userAccountControl", []string{strCheckboxState})

				if err != nil {
					updateLog(fmt.Sprintf("%s", err), "red")
				} else {
					updateLog("Object's UAC updated to "+strCheckboxState+" at: "+baseDN, "green")
				}

				idx := findLocationInSiblings(currentNode)

				parent := reloadParentNode(currentNode)
				siblings := parent.GetChildren()

				reloadAttributesPanel(currentNode, cacheEntries)

				app.SetRoot(appPanel, true).SetFocus(treePanel)

				treePanel.SetCurrentNode(siblings[idx])
			}).
			AddButton("Cancel", func() {
				app.SetRoot(appPanel, true).SetFocus(treePanel)
			})

		updateUacForm.SetTitle("userAccountControl Editor").SetBorder(true)
		app.SetRoot(updateUacForm, true).SetFocus(updateUacForm)
	}

	return event
}

func treePanelChangeHandler(node *tview.TreeNode) {
	reloadAttributesPanel(node, cacheEntries)
}

func attrsPanelKeyHandler(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyDelete:
		currentNode := treePanel.GetCurrentNode()
		attrRow, _ := attrsPanel.GetSelection()

		baseDN := currentNode.GetReference().(string)
		attrName := attrsPanel.GetCell(attrRow, 0).Text

		promptModal := tview.NewModal().
			SetText("Do you really want to delete attribute `" + attrName + "` of this object?\n" + baseDN).
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				if buttonLabel == "Yes" {
					err := lc.DeleteAttribute(baseDN, attrName)
					if err != nil {
						updateLog(fmt.Sprint(err), "red")
					} else {
						delete(loadedDNs, baseDN)
						updateLog("Attribute deleted: "+attrName+" from "+baseDN, "green")

						reloadAttributesPanel(currentNode, cacheEntries)
					}
				}

				app.SetRoot(appPanel, true).SetFocus(treePanel)
			})

		app.SetRoot(promptModal, false).SetFocus(promptModal)
	case tcell.KeyCtrlN:
		currentNode := treePanel.GetCurrentNode()
		if currentNode == nil {
			return event
		}

		createAttrForm := tview.NewForm()

		baseDN := currentNode.GetReference().(string)

		createAttrForm.
			AddTextView("Object DN", baseDN, 0, 1, false, true).
			AddInputField("Attribute Name", "", 20, nil, nil).
			AddInputField("Attribute Value", "", 20, nil, nil).
			SetFieldBackgroundColor(tcell.GetColor("black")).
			AddButton("Create", func() {
				attrName := createAttrForm.GetFormItemByLabel("Attribute Name").(*tview.InputField).GetText()
				attrVal := createAttrForm.GetFormItemByLabel("Attribute Value").(*tview.InputField).GetText()

				err := lc.AddAttribute(baseDN, attrName, []string{attrVal})
				if err != nil {
					updateLog(fmt.Sprint(err), "red")
				} else {
					delete(loadedDNs, baseDN)
					updateLog("Attribute added: "+attrName+" to "+baseDN, "green")

					reloadAttributesPanel(currentNode, cacheEntries)
				}

				app.SetRoot(appPanel, false).SetFocus(treePanel)
			}).
			AddButton("Go Back", func() {
				app.SetRoot(appPanel, false).SetFocus(treePanel)
			}).
			SetTitle("Attribute Creator").
			SetBorder(true)

		app.SetRoot(createAttrForm, true).SetFocus(createAttrForm)
	}

	return event
}

func explorerRotateFocus() {
	currentFocus := app.GetFocus()

	switch currentFocus {
	case treePanel:
		app.SetFocus(attrsPanel)
	case attrsPanel:
		app.SetFocus(treePanel)
	}
}

func explorerPageKeyHandler(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab {
		explorerRotateFocus()
		return nil
	}

	switch event.Key() {
	case tcell.KeyCtrlE:
		currentNode := treePanel.GetCurrentNode()
		if currentNode == nil {
			return event
		}

		writeAttrValsForm := tview.NewForm()

		attrRow, _ := attrsPanel.GetSelection()
		baseDN := currentNode.GetReference().(string)
		attrName := attrsPanel.GetCell(attrRow, 0).Text

		entry := loadedDNs[baseDN]
		attrVals := entry.GetAttributeValues(attrName)
		if len(attrVals) == 0 {
			return event
		}

		// Encode attribute values to hex
		rawAttrVals := entry.GetRawAttributeValues(attrName)

		var attrValsHex []string
		for idx := range rawAttrVals {
			hexEncoded := hex.EncodeToString(rawAttrVals[idx])
			attrValsHex = append(attrValsHex, hexEncoded)
		}

		valIndices := []string{}
		for idx := range attrVals {
			valIndices = append(valIndices, strconv.Itoa(idx))
		}
		valIndices = append(valIndices, "New")

		selectedIndex := 0

		useHexEncoding := false

		writeAttrValsForm = writeAttrValsForm.
			AddTextView("Base DN", baseDN, 0, 1, false, true).
			AddTextView("Attribute Name", attrName, 0, 1, false, true).
			AddTextView("Current Value", attrVals[0], 0, 1, false, true).
			AddTextView("Current Value (HEX)", attrValsHex[0], 0, 1, false, true).
			AddDropDown("Value Index", valIndices, 0, func(option string, optionIndex int) {
				selectedIndex = optionIndex

				currentValItem := writeAttrValsForm.GetFormItemByLabel("Current Value").(*tview.TextView)
				currentValItemHex := writeAttrValsForm.GetFormItemByLabel("Current Value (HEX)").(*tview.TextView)

				if selectedIndex < len(attrVals) {
					currentValItem.SetText(attrVals[selectedIndex])
					currentValItemHex.SetText(attrValsHex[selectedIndex])
				} else {
					currentValItem.SetText("")
					currentValItemHex.SetText("")
				}
			}).
			AddInputField("New Value", "", 0, nil, nil).
			AddCheckbox("Use HEX encoding", false, func(checked bool) {
				useHexEncoding = checked
			}).
			SetFieldBackgroundColor(tcell.GetColor("black")).
			AddButton("Update", func() {
				newValue := writeAttrValsForm.GetFormItemByLabel("New Value").(*tview.InputField).GetText()
				if useHexEncoding {
					newValueBytes, err := hex.DecodeString(newValue)
					if err == nil {
						newValue = string(newValueBytes)
					} else {
						updateLog(fmt.Sprint(err), "red")
						return
					}
				}

				if selectedIndex < len(attrVals) {
					attrVals[selectedIndex] = newValue
				} else {
					attrVals = append(attrVals, newValue)
				}

				err := lc.ModifyAttribute(baseDN, attrName, attrVals)
				// TODO: Don't go back immediately so that the user can
				// change multiple values at once
				if err != nil {
					updateLog(fmt.Sprint(err), "red")
				} else {
					updateLog("Attribute updated: '"+attrName+"' from '"+baseDN+"'", "green")
				}

				idx := findLocationInSiblings(currentNode)

				parent := reloadParentNode(currentNode)
				siblings := parent.GetChildren()

				treePanel.SetCurrentNode(siblings[idx])

				reloadAttributesPanel(currentNode, cacheEntries)

				app.SetRoot(appPanel, false).SetFocus(treePanel)
			}).
			AddButton("Go Back", func() {
				app.SetRoot(appPanel, false).SetFocus(treePanel)
			})

		writeAttrValsForm.SetTitle("Attribute Editor").SetBorder(true)
		app.SetRoot(writeAttrValsForm, true).SetFocus(writeAttrValsForm)
	case tcell.KeyCtrlP:
		currentNode := treePanel.GetCurrentNode()
		if currentNode == nil {
			return event
		}

		changePasswordForm := tview.NewForm()

		baseDN := currentNode.GetReference().(string)
		changePasswordForm = changePasswordForm.
			AddTextView("Object DN", baseDN, 0, 1, false, true).
			AddPasswordField("New Password", "", 20, '*', nil).
			SetFieldBackgroundColor(tcell.GetColor("black")).
			AddButton("Update", func() {
				newPassword := changePasswordForm.GetFormItemByLabel("New Password").(*tview.InputField).GetText()

				err := lc.ResetPassword(baseDN, newPassword)
				if err != nil {
					updateLog(fmt.Sprint(err), "red")
				} else {
					updateLog("Password changed: "+baseDN, "green")
				}

				app.SetRoot(appPanel, false).SetFocus(treePanel)
			}).
			AddButton("Go Back", func() {
				app.SetRoot(appPanel, false).SetFocus(treePanel)
			})

		changePasswordForm.SetTitle("Password Editor").SetBorder(true)
		app.SetRoot(changePasswordForm, true).SetFocus(changePasswordForm)
	}

	return event
}

package main

import (

	//"strconv"
	//"strings"

	"github.com/rivo/tview"
)

var helpPage *tview.Flex
var keybindingsPanel *tview.Table

func InitHelpPage() {
	helpText := `
 _______  _______  ______   _______  _______ 
(  ____ \(  ___  )(  __  \ (  ___  )(  ____ )
| (    \/| (   ) || (  \  )| (   ) || (    )|
| |      | |   | || |   ) || (___) || (____)|
| | ____ | |   | || |   | ||  ___  ||  _____)
| | \_  )| |   | || |   ) || (   ) || (      
| (___) || (___) || (__/  )| )   ( || )      
(_______)(_______)(______/ |/     \||/       

v1.6.0
`

	keybindings := [][]string{
		{"Ctrl + J", "Global", "Next panel"},
		{"f / F", "Global", "Toggle attribute formatting"},
		{"e / E", "Global", "Toggle emojis"},
		{"c / C", "Global", "Toggle colors"},
		{"a / A", "Global", "Toggle attribute expansion for multi-value attributes"},
		{"l / L", "Global", "Change current server address & credentials"},
		{"r / R", "Global", "Reconnect to the server"},
		{"u / U", "Global", "Upgrade connection to use TLS (with StartTLS)"},
		{"Ctrl + e / E", "Attributes panel", "Edit the selected attribute of the selected object"},
		{"Ctrl + n / N", "Attributes panel", "Create a new attribute in the selected object"},
		{"Ctrl + n / N", "Explorer panel", "Create a new object under the selected object"},
		{"Ctrl + s / S", "Explorer panel", "Export all loaded nodes in the selected subtree into a JSON file"},
		{"Ctrl + p / P", "Explorer panel", "Change the password of the selected user or computer account"},
		{"Ctrl + a / A", "Explorer panel", "Update the userAccountControl of the object interactively"},
		{"Delete", "Explorer/attributes panel", "Deletes the selected object or attribute"},
		{"h / H", "Global", "Show/hide headers"},
		{"q", "Global", "Exit the program"},
	}

	// Create a table
	keybindingsPanel = tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false)

	headers := []string{"Keybinding", "Context", "Action"}
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tview.Styles.SecondaryTextColor).
			SetAlign(tview.AlignCenter).SetSelectable(false)
		keybindingsPanel.SetCell(0, col, cell)
	}

	for row, binding := range keybindings {
		for col, value := range binding {
			cell := tview.NewTableCell(value).
				SetTextColor(tview.Styles.PrimaryTextColor).
				SetAlign(tview.AlignLeft)
			keybindingsPanel.SetCell(row+1, col, cell)
		}
	}

	keybindingsPanel.Select(1, 0)

	helpTextView := tview.NewTextView().
		SetText(helpText).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	frame := tview.NewFrame(keybindingsPanel)
	frame.SetBorders(0, 0, 0, 0, 0, 0).
		SetTitleAlign(tview.AlignCenter).
		SetBorder(true).
		SetTitle(" Keybindings ")

	helpPage = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(helpTextView, 12, 0, true).
		AddItem(frame, 0, 2, false)
}

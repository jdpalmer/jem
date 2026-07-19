package editor

import (
	"github.com/jdpalmer/jem/view"
)

type menuItem struct {
	label   string
	command string
}

func menuItemLabel(ctx any, idx uint8) []byte {
	items := ctx.([]menuItem)
	if int(idx) >= len(items) {
		return nil
	}
	return []byte(items[idx].label)
}

var mainMenu = []menuItem{
	{label: "Open", command: "file_visit"},
	{label: "Save", command: "file_save"},
	{label: "Undo", command: "undo"},
	{label: "Yank", command: "yank"},
	{label: "Search", command: "isearch_forward"},
	{label: "Menu Quit", command: "quit"},
}

// CmdMenuRun opens the message-line menu and dispatches the chosen command.
func CmdMenuRun(f bool, n int) bool {
	_ = f
	_ = n
	AskChoose("Menu > ", mainMenu, menuItemLabel, uint8(len(mainMenu)), 0, func(result int16) {
		if result == -2 {
			CmdAbort(false, 1)
			return
		}
		if result < 0 {
			view.MBClear()
			return
		}
		cmd := mainMenu[result].command
		view.MBClear()
		c := commandByName(cmd)
		if c != nil && c.Fn != nil {
			_ = c.Fn(false, 1)
		}
	})
	return true
}

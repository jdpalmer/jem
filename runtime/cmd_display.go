package runtime

import (
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/window"
)

// CmdRefresh forces a full screen refresh on the next DisplayUpdate.
func CmdRefresh(f bool, n int) bool {
	_ = f
	_ = n
	display.Active.ScreenDirty = true
	return true
}

// CmdThemeToggle switches between dark and light theme palettes.
func CmdThemeToggle(f bool, n int) bool {
	_ = f
	_ = n
	theme := &display.Active.Theme
	if theme.Mode == display.ThemeDark {
		theme.Mode = display.ThemeLight
	} else {
		theme.Mode = display.ThemeDark
	}
	display.ThemeUpdate()
	display.Active.ScreenDirty = true
	for i := 0; i < int(len(window.Active.Windows)); i++ {
		win := window.Active.Windows[i]
		if win != nil {
			win.ShouldRedraw = true
			win.ShouldUpdateModeLine = true
		}
	}
	if theme.Mode == display.ThemeLight {
		display.MBWrite("[light mode]")
	} else {
		display.MBWrite("[dark mode]")
	}
	return true
}

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
			display.MBClear()
			return
		}
		cmd := mainMenu[result].command
		display.MBClear()
		c := commandByName(cmd)
		if c != nil && c.Fn != nil {
			_ = c.Fn(false, 1)
		}
	})
	return true
}

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
		wp := window.Active.Windows[i]
		if wp != nil {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
	}
	if theme.Mode == display.ThemeLight {
		display.MBWrite("[light mode]")
	} else {
		display.MBWrite("[dark mode]")
	}
	return true
}

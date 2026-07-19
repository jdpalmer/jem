package editor

import (
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/view"
)

// CmdRefresh forces a full screen refresh on the next DisplayUpdate.
func CmdRefresh(f bool, n int) bool {
	_ = f
	_ = n
	model.State.ScreenDirty = true
	return true
}

// CmdThemeToggle switches between dark and light theme palettes.
func CmdThemeToggle(f bool, n int) bool {
	_ = f
	_ = n
	theme := &model.State.Theme
	if theme.Mode == model.ThemeDark {
		theme.Mode = model.ThemeLight
	} else {
		theme.Mode = model.ThemeDark
	}
	view.ThemeUpdate()
	model.State.ScreenDirty = true
	for i := 0; i < int(len(model.State.Windows)); i++ {
		wp := model.State.Windows[i]
		if wp != nil {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
	}
	if theme.Mode == model.ThemeLight {
		view.MBWrite("[light mode]")
	} else {
		view.MBWrite("[dark mode]")
	}
	return true
}

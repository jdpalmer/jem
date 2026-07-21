package runtime

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/registers"
	"github.com/jdpalmer/jem/window"
)

// CmdCopyRegister copies the active region to a named register.
func CmdCopyRegister(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil {
		return false
	}
	var region window.Region
	if !getRegion(win, &region) {
		return false
	}
	text := buf.GetText(region.Start, region.End)
	length := len(text)
	if text == nil && length > 0 {
		display.MBWrite("[out of memory]")
		return false
	}
	AskString("Register Name: ", "", func(name string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		if name == "" {
			display.MBWrite("[register name required]")
			return
		}
		if !registers.Set(name, text) {
			return
		}
		display.MBWrite("Register '%s' copied.", name)
		win.Mark = buffer.Location{Line: 0, Offset: 0}
		win.ShouldRedraw = true
	})
	return true
}

// CmdInsertRegister inserts a named register at point.
func CmdInsertRegister(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	if win == nil {
		return false
	}
	AskString("Register Name: ", "", func(name string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		text, ok := registers.Get(name)
		if !ok {
			display.MBWrite("[register '%s' not found]", name)
			return
		}
		if err := window.InsertText(win, text); err != nil {
			return
		}
		display.MBWrite("Register '%s' inserted.", name)
	})
	return true
}

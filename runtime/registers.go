package runtime

// registers.go - Named clipboards / text registers (translation of registers.c)

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
)

var registerStore = make(map[string][]byte)

func RegisterSetText(name string, text []byte) bool {
	if name == "" {
		display.MBWrite("[register name required]")
		return false
	}
	if len(text) == 0 {
		delete(registerStore, name)
		return true
	}
	copyBuf := make([]byte, len(text))
	copy(copyBuf, text)
	registerStore[name] = copyBuf
	return true
}

func RegisterGetText(name string) ([]byte, bool) {
	val, ok := registerStore[name]
	return val, ok
}

// CmdCopyRegister copies the active region to a named register.
func CmdCopyRegister(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil {
		return false
	}
	var region window.Region
	if !getRegion(wp, &region) {
		return false
	}
	text := bp.GetText(region.Start, region.End)
	length := uint(len(text))
	if text == nil && length > 0 {
		display.MBWrite("[out of memory]")
		return false
	}
	AskString("Register Name: ", "", func(name string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		if !RegisterSetText(name, text) {
			return
		}
		display.MBWrite("Register '%s' copied.", name)
		wp.Mark = buffer.Location{Line: 0, Offset: 0}
		wp.ShouldRedraw = true
	})
	return true
}

// CmdInsertRegister inserts a named register at point.
func CmdInsertRegister(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
	if wp == nil {
		return false
	}
	AskString("Register Name: ", "", func(name string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		text, ok := RegisterGetText(name)
		if !ok {
			display.MBWrite("[register '%s' not found]", name)
			return
		}
		if !window.InsertText(wp, text) {
			return
		}
		display.MBWrite("Register '%s' inserted.", name)
	})
	return true
}

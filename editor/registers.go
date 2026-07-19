package editor

// registers.go - Named clipboards / text registers (translation of registers.c)

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/view"
)

var registerStore = make(map[string][]byte)

func RegisterSetText(name string, text []byte) bool {
	if name == "" {
		view.MBWrite("[register name required]")
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
	wp := model.State.CurrentWindow
	bp := model.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	var region model.Region
	if !getRegion(wp, &region) {
		return false
	}
	text := bp.GetText(region.Start, region.End)
	length := uint(len(text))
	if text == nil && length > 0 {
		view.MBWrite("[out of memory]")
		return false
	}
	AskString("Register Name: ", "", func(name string, pr model.PromptResult) {
		if pr != model.PromptResultYes {
			return
		}
		if !RegisterSetText(name, text) {
			return
		}
		view.MBWrite("Register '%s' copied.", name)
		wp.Mark = buffer.Location{Line: 0, Offset: 0}
		wp.ShouldRedraw = true
	})
	return true
}

// CmdInsertRegister inserts a named register at point.
func CmdInsertRegister(f bool, n int) bool {
	_ = f
	_ = n
	wp := model.State.CurrentWindow
	if wp == nil {
		return false
	}
	AskString("Register Name: ", "", func(name string, pr model.PromptResult) {
		if pr != model.PromptResultYes {
			return
		}
		text, ok := RegisterGetText(name)
		if !ok {
			view.MBWrite("[register '%s' not found]", name)
			return
		}
		if !windowInsertText(wp, text) {
			return
		}
		view.MBWrite("Register '%s' inserted.", name)
	})
	return true
}

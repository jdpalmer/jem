package mode

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/window"
)

func beginEdit() {
	win := window.Active.CurrentWindow
	if win == nil {
		return
	}
	buffer.BeginCommand(win.Cursor)
}

func endEdit() { buffer.EndCommand() }

func CurrentModeInfo() *ModeInfo {
	buf := buffer.All.Current
	if buf == nil {
		return LangModeInfo(buffer.LModeNone)
	}
	return LangModeInfo(buf.LangMode)
}

func CmdModeNewlineAndIndent(f bool, n int) bool {
	return CurrentModeInfo().NewlineAndIndent(f, n)
}

func CmdModeIndentLine(f bool, n int) bool {
	_ = f
	_ = n
	return CurrentModeInfo().IndentLine(false, 1)
}

func CmdModeCloseBrace(f bool, n int) bool {
	return CurrentModeInfo().CloseBrace(f, n)
}

func CmdModeGotoMatch(f bool, n int) bool {
	_ = f
	_ = n
	return CurrentModeInfo().GotoMatch(false, 1)
}

func CmdModeMakeComment(f bool, n int) bool {
	_ = f
	_ = n
	beginEdit()
	ok := CurrentModeInfo().MakeComment(false, 1)
	endEdit()
	return ok
}

func CmdModeTopOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	return CurrentModeInfo().TopOfFunction(false, 1)
}

func CmdModeEndOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	return CurrentModeInfo().EndOfFunction(false, 1)
}

func CmdModeMarkFunction(f bool, n int) bool {
	_ = f
	_ = n
	return CurrentModeInfo().MarkFunction(false, 1)
}

package mode

import (
	"github.com/jdpalmer/jem/buffer"
)

type Hooks struct {
	Message          func(msg string)
	DefaultGotoMatch func(f bool, n int) bool
	BeginCommand     func()
	EndCommand       func()
	SetText          func(bp *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error
}

var PackageHooks Hooks

func CurrentModeInfo() *ModeInfo {
	bp := buffer.All.Current
	if bp == nil {
		return LangModeInfo(buffer.LModeNone)
	}
	return LangModeInfo(bp.LangMode)
}

func ModeDispatch(fn func(f bool, n int) bool, f bool, n int) bool {
	if fn == nil {
		return false
	}
	return fn(f, n)
}

func CmdModeNewlineAndIndent(f bool, n int) bool {
	return ModeDispatch(CurrentModeInfo().NewlineAndIndent, f, n)
}

func CmdModeIndentLine(f bool, n int) bool {
	_ = f
	_ = n
	return ModeDispatch(CurrentModeInfo().IndentLine, false, 1)
}

func CmdModeCloseBrace(f bool, n int) bool {
	return ModeDispatch(CurrentModeInfo().CloseBrace, f, n)
}

func CmdModeGotoMatch(f bool, n int) bool {
	_ = f
	_ = n
	return ModeDispatch(CurrentModeInfo().GotoMatch, false, 1)
}

func CmdModeMakeComment(f bool, n int) bool {
	_ = f
	_ = n
	PackageHooks.BeginCommand()
	ok := ModeDispatch(CurrentModeInfo().MakeComment, false, 1)
	PackageHooks.EndCommand()
	return ok
}

func CmdModeTopOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	return ModeDispatch(CurrentModeInfo().TopOfFunction, false, 1)
}

func CmdModeEndOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	return ModeDispatch(CurrentModeInfo().EndOfFunction, false, 1)
}

func CmdModeMarkFunction(f bool, n int) bool {
	_ = f
	_ = n
	return ModeDispatch(CurrentModeInfo().MarkFunction, false, 1)
}

package modes

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/session"
)

type Hooks struct {
	UndoBeginCommand      func()
	UndoEndCommand        func()
	BufferSetText         func(bp *buffer.Buffer, begin, end buffer.Location, newText []byte, newLen uint, newEndOut *buffer.Location, kill bool) bool
	WindowInsertNewline   func(wp *session.Window) bool
	WindowInsertText      func(wp *session.Window, text []byte, length int) bool
	WindowInsertCodepoint func(wp *session.Window, c rune) bool
	WindowSetCursor       func(wp *session.Window, loc buffer.Location)
	Message               func(msg string)
	DefaultGotoMatch      func(f bool, n int) bool
}

var PackageHooks Hooks

func CurrentModeInfo() *session.ModeInfo {
	bp := session.App.CurrentBuffer
	if bp == nil {
		return LangModeInfo(session.LModeNone)
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
	if PackageHooks.UndoBeginCommand != nil {
		PackageHooks.UndoBeginCommand()
	}
	ok := ModeDispatch(CurrentModeInfo().MakeComment, false, 1)
	if PackageHooks.UndoEndCommand != nil {
		PackageHooks.UndoEndCommand()
	}
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

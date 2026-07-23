package runtime

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/window"
)

// cmd_syntax.go — editor commands that depend on windows/minibuffer.

type syntaxMatchResult int

const (
	syntaxMatchNone syntaxMatchResult = iota
	syntaxMatchFound
	syntaxMatchUnbalanced
)

func syntaxMatchTarget(win *window.Window, matchOut *buffer.Location) syntaxMatchResult {
	buf := win.Buffer
	cursor := buffer.MakeLocation(win.Cursor.Line, win.Cursor.Offset)
	if cursor.Line == 0 || cursor.Line >= buf.EOF() {
		return syntaxMatchNone
	}
	if syntaxLocationHasDelimiter(buf, cursor) {
		if syntax.FindMatchingDelimiter(buf, cursor, matchOut) {
			return syntaxMatchFound
		}
		return syntaxMatchUnbalanced
	}
	if cursor.Offset == 0 {
		return syntaxMatchNone
	}
	prior := buffer.MakeLocation(cursor.Line, cursor.Offset-1)
	if syntaxLocationHasDelimiter(buf, prior) {
		if syntax.FindMatchingDelimiter(buf, prior, matchOut) {
			return syntaxMatchFound
		}
		return syntaxMatchUnbalanced
	}
	return syntaxMatchNone
}

func syntaxLocationHasDelimiter(buf *buffer.Buffer, loc buffer.Location) bool {
	if loc.Line == 0 || loc.Line >= buf.EOF() {
		return false
	}
	line := buf.Line(loc.Line)
	if loc.Offset >= line.Len() {
		return false
	}
	ch := int(line.Byte(loc.Offset))
	if _, _, _, ok := syntax.DelimiterPair(ch); !ok {
		return false
	}
	return syntax.CharIsStructural(buf, loc.Line, loc.Offset)
}

func CmdSyntaxGotoMatch(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	var match buffer.Location
	switch syntaxMatchTarget(win, &match) {
	case syntaxMatchNone:
		display.MBWrite("[No bracket here]")
		return false
	case syntaxMatchUnbalanced:
		display.MBWrite("[No matching bracket]")
		return false
	default:
		win.SetCursor(match)
		win.DidMove = true
		return true
	}
}

package editor

import (
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/view"
)

// cmd_syntax.go — editor commands that depend on windows/minibuffer.

type syntaxMatchResult int

const (
	syntaxMatchNone syntaxMatchResult = iota
	syntaxMatchFound
	syntaxMatchUnbalanced
)

func syntaxMatchTarget(wp *model.Window, matchOut *buffer.Location) syntaxMatchResult {
	if wp == nil || wp.Buffer == nil {
		return syntaxMatchNone
	}
	bp := wp.Buffer
	cursor := buffer.MakeLocation(wp.Cursor.Line, wp.Cursor.Offset)
	if cursor.Line == 0 || cursor.Line >= bp.EOF() {
		return syntaxMatchNone
	}
	if syntaxLocationHasDelimiter(bp, cursor) {
		if syntax.FindMatchingDelimiter(bp, cursor, matchOut) {
			return syntaxMatchFound
		}
		return syntaxMatchUnbalanced
	}
	if cursor.Offset == 0 {
		return syntaxMatchNone
	}
	prior := buffer.MakeLocation(cursor.Line, cursor.Offset-1)
	if syntaxLocationHasDelimiter(bp, prior) {
		if syntax.FindMatchingDelimiter(bp, prior, matchOut) {
			return syntaxMatchFound
		}
		return syntaxMatchUnbalanced
	}
	return syntaxMatchNone
}

func syntaxLocationHasDelimiter(bp *buffer.Buffer, loc buffer.Location) bool {
	if bp == nil || loc.Line == 0 || loc.Line >= bp.EOF() {
		return false
	}
	lp := bp.Line(loc.Line)
	if lp == nil || loc.Offset >= lp.Len() {
		return false
	}
	ch := int(lp.Byte(loc.Offset))
	if _, _, _, ok := syntax.DelimiterPair(ch); !ok {
		return false
	}
	return syntax.CharIsStructural(bp, loc.Line, loc.Offset)
}

func CmdSyntaxGotoMatch(f bool, n int) bool {
	_ = f
	_ = n
	wp := model.State.CurrentWindow
	if wp == nil {
		return false
	}
	var match buffer.Location
	switch syntaxMatchTarget(wp, &match) {
	case syntaxMatchNone:
		view.MBWrite("[No bracket here]")
		return false
	case syntaxMatchUnbalanced:
		view.MBWrite("[No matching bracket]")
		return false
	default:
		wp.SetCursor(match)
		wp.DidMove = true
		return true
	}
}

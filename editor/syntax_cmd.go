package editor

import "github.com/jdpalmer/jem/syntax"

// syntax_cmd.go — editor commands that depend on windows/minibuffer.

type syntaxMatchResult int

const (
	syntaxMatchNone syntaxMatchResult = iota
	syntaxMatchFound
	syntaxMatchUnbalanced
)

func syntaxMatchTarget(wp *Window, matchOut *Location) syntaxMatchResult {
	if wp == nil || wp.Buffer == nil {
		return syntaxMatchNone
	}
	bp := wp.Buffer
	cursor := MakeLocation(wp.Cursor.Line, wp.Cursor.Offset)
	if cursor.Line == 0 || cursor.Line >= BufferEOF(bp) {
		return syntaxMatchNone
	}
	if syntaxLocationHasDelimiter(bp, cursor) {
		if syntaxFindMatchingDelimiter(bp, cursor, matchOut) {
			return syntaxMatchFound
		}
		return syntaxMatchUnbalanced
	}
	if cursor.Offset == 0 {
		return syntaxMatchNone
	}
	prior := MakeLocation(cursor.Line, cursor.Offset-1)
	if syntaxLocationHasDelimiter(bp, prior) {
		if syntaxFindMatchingDelimiter(bp, prior, matchOut) {
			return syntaxMatchFound
		}
		return syntaxMatchUnbalanced
	}
	return syntaxMatchNone
}

func syntaxLocationHasDelimiter(bp *Buffer, loc Location) bool {
	if bp == nil || loc.Line == 0 || loc.Line >= BufferEOF(bp) {
		return false
	}
	lp := BufferGetLine(bp, loc.Line)
	if lp == nil || loc.Offset >= LineLength(lp) {
		return false
	}
	ch := int(LineGetc(lp, loc.Offset))
	if _, _, _, ok := syntax.DelimiterPair(ch); !ok {
		return false
	}
	return syntax.CharIsStructural(bp, loc.Line, loc.Offset)
}

func CmdSyntaxGotoMatch(f bool, n int) bool {
	_ = f
	_ = n
	wp := session.App.CurrentWindow
	if wp == nil {
		return false
	}
	var match Location
	switch syntaxMatchTarget(wp, &match) {
	case syntaxMatchNone:
		mbWrite("[No bracket here]")
		return false
	case syntaxMatchUnbalanced:
		mbWrite("[No matching bracket]")
		return false
	default:
		windowSetCursor(wp, match)
		wp.DidMove = true
		return true
	}
}

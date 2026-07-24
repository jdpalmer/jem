package mode

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/window"
)

func ModeNewlineAndIndent(f bool, n int) bool {
	_ = f
	win := window.Active.CurrentWindow
	for i := 0; i < n; i++ {
		if err := window.InsertNewline(win); err != nil {
			return false
		}
	}
	return true
}

func ModeIndentLine(f bool, n int) bool {
	_ = f
	_ = n
	return true
}

func ModeCloseBrace(f bool, n int) bool {
	_ = f
	win := window.Active.CurrentWindow
	for i := 0; i < n; i++ {
		if err := window.InsertCodepoint(win, '}'); err != nil {
			return false
		}
	}
	return true
}

type matchResult int

const (
	matchNone matchResult = iota
	matchFound
	matchUnbalanced
)

func locationHasDelimiter(buf *buffer.Buffer, loc buffer.Location) bool {
	if loc.Line == 0 || loc.Line >= buf.EOF() {
		return false
	}
	line := buf.Line(loc.Line)
	if line == nil || loc.Offset >= line.Len() {
		return false
	}
	ch := int(line.Byte(loc.Offset))
	if _, _, _, ok := syntax.DelimiterPair(ch); !ok {
		return false
	}
	return syntax.CharIsStructural(buf, loc.Line, loc.Offset)
}

func matchTarget(win *window.Window, matchOut *buffer.Location) matchResult {
	buf := win.Buffer
	cursor := buffer.MakeLocation(win.Cursor.Line, win.Cursor.Offset)
	if cursor.Line == 0 || cursor.Line >= buf.EOF() {
		return matchNone
	}
	if locationHasDelimiter(buf, cursor) {
		if syntax.FindMatchingDelimiter(buf, cursor, matchOut) {
			return matchFound
		}
		return matchUnbalanced
	}
	if cursor.Offset == 0 {
		return matchNone
	}
	prior := buffer.MakeLocation(cursor.Line, cursor.Offset-1)
	if locationHasDelimiter(buf, prior) {
		if syntax.FindMatchingDelimiter(buf, prior, matchOut) {
			return matchFound
		}
		return matchUnbalanced
	}
	return matchNone
}

func ModeGotoMatch(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	if win == nil || win.Buffer == nil {
		return false
	}
	var match buffer.Location
	switch matchTarget(win, &match) {
	case matchNone:
		Message("[No bracket here]")
		return false
	case matchUnbalanced:
		Message("[No matching bracket]")
		return false
	default:
		win.SetCursor(match)
		win.DidMove = true
		return true
	}
}

func ModeMakeComment(f bool, n int) bool   { return false }
func ModeTopOfFunction(f bool, n int) bool { return false }
func ModeEndOfFunction(f bool, n int) bool { return false }
func ModeMarkFunction(f bool, n int) bool  { return false }

func init() {
	for i := range modeTable {
		if modeTable[i].NewlineAndIndent == nil {
			modeTable[i].NewlineAndIndent = ModeNewlineAndIndent
		}
		if modeTable[i].IndentLine == nil {
			modeTable[i].IndentLine = ModeIndentLine
		}
		if modeTable[i].CloseBrace == nil {
			modeTable[i].CloseBrace = ModeCloseBrace
		}
		if modeTable[i].GotoMatch == nil {
			modeTable[i].GotoMatch = ModeGotoMatch
		}
		if modeTable[i].MakeComment == nil {
			modeTable[i].MakeComment = ModeMakeComment
		}
		if modeTable[i].TopOfFunction == nil {
			modeTable[i].TopOfFunction = ModeTopOfFunction
		}
		if modeTable[i].EndOfFunction == nil {
			modeTable[i].EndOfFunction = ModeEndOfFunction
		}
		if modeTable[i].MarkFunction == nil {
			modeTable[i].MarkFunction = ModeMarkFunction
		}
	}
}

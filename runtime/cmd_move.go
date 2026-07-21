package runtime

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
)

// Cursor movement and navigation.

// helper: ASCII word char
func isWordChar(b byte) bool {
	if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' {
		return true
	}
	return false
}

// move forward by one word: skip non-word then skip word
func forwardWordLoc(bp *buffer.Buffer, loc buffer.Location) buffer.Location {
	if bp == nil || loc.Line == 0 {
		return loc
	}
	line := bp.Line(loc.Line)
	if line == nil {
		return loc
	}
	off := int(loc.Offset)
	// If past EOL, start at the next line; otherwise skip non-word on this line.
	if off >= len(line.Data) {
		if loc.Line >= bp.LineCount {
			return buffer.Location{Line: bp.LineCount, Offset: line.Len()}
		}
		loc.Line++
		off = 0
	} else {
		for off < len(line.Data) && !isWordChar(line.Data[off]) {
			off++
		}
	}
	// Skip non-word then word, wrapping across lines as needed.
	for loc.Line <= bp.LineCount {
		line = bp.Line(loc.Line)
		if line == nil {
			return loc
		}
		for off < len(line.Data) && !isWordChar(line.Data[off]) {
			off++
		}
		if off < len(line.Data) {
			for off < len(line.Data) && isWordChar(line.Data[off]) {
				off++
			}
			return buffer.Location{Line: loc.Line, Offset: uint(off)}
		}
		if loc.Line >= bp.LineCount {
			return buffer.Location{Line: bp.LineCount, Offset: 0}
		}
		loc.Line++
		off = 0
	}
	return buffer.Location{Line: bp.LineCount, Offset: 0}
}

// move backward by one word: go left, skip non-word, then skip word backwards
func backwardWordLoc(bp *buffer.Buffer, loc buffer.Location) buffer.Location {
	if bp == nil || loc.Line == 0 {
		return loc
	}
	if loc.Line == 1 && loc.Offset == 0 {
		return loc
	}
	line := bp.Line(loc.Line)
	if line == nil {
		return loc
	}
	off := int(loc.Offset)
	// Step left one position (if at offset 0, move to end of previous line).
	if off == 0 {
		if loc.Line > 1 {
			loc.Line--
			line = bp.Line(loc.Line)
			if line != nil {
				off = len(line.Data)
			} else {
				off = 0
			}
		} else {
			return loc
		}
	}
	// Step back to a word char, then skip non-word and word backwards.
	for off > 0 {
		offPrev := buffer.PrevOffset(line.Data, uint(off))
		if offPrev == uint(off) {
			off--
		} else {
			off = int(offPrev)
		}
		b := byte(0)
		if off < len(line.Data) {
			b = line.Data[off]
		}
		if isWordChar(b) || off == 0 {
			break
		}
	}
	for off > 0 && !isWordChar(line.Data[off-1]) {
		off--
	}
	for off > 0 && isWordChar(line.Data[off-1]) {
		off--
	}
	return buffer.Location{Line: loc.Line, Offset: uint(off)}
}

// Move forward by a single codepoint, preserving UTF-8 boundaries.
func CmdForwardChar(f bool, n int) bool {
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil {
		return false
	}

	for i := 0; i < n; i++ {
		line := bp.Line(wp.Cursor.Line)
		if line != nil && wp.Cursor.Offset < line.Len() {
			wp.Cursor.Offset = buffer.NextOffset(line.Data, wp.Cursor.Offset)
		} else if wp.Cursor.Line < bp.LineCount {
			wp.Cursor.Line++
			wp.Cursor.Offset = 0
		} else {
			break
		}
	}
	wp.DidMove = true
	return true
}

func CmdBackwardChar(f bool, n int) bool {
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil {
		return false
	}

	for i := 0; i < n; i++ {
		line := bp.Line(wp.Cursor.Line)
		if line != nil && wp.Cursor.Offset > 0 {
			wp.Cursor.Offset = buffer.PrevOffset(line.Data, wp.Cursor.Offset)
		} else if wp.Cursor.Line > 1 {
			wp.Cursor.Line--
			prevLine := bp.Line(wp.Cursor.Line)
			if prevLine != nil {
				wp.Cursor.Offset = prevLine.Len()
			} else {
				wp.Cursor.Offset = 0
			}
		} else {
			break
		}
	}
	wp.DidMove = true
	return true
}

// CmdForwardWord moves forward by words (ASCII words: letters, digits, underscore)
// CmdForwardWord moves forward by words (ASCII words: letters, digits, underscore)
func CmdForwardWord(f bool, n int) bool {
	_ = f
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil {
		return false
	}
	for i := 0; i < n; i++ {
		wp.Cursor = forwardWordLoc(bp, wp.Cursor)
	}
	wp.DidMove = true
	return true
}

// CmdBackwardWord moves backward by words
// CmdBackwardWord moves backward by words
func CmdBackwardWord(f bool, n int) bool {
	_ = f
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil {
		return false
	}
	for i := 0; i < n; i++ {
		wp.Cursor = backwardWordLoc(bp, wp.Cursor)
	}
	wp.DidMove = true
	return true
}

// delete forward word
// Page-wise movement
func CmdForwardPage(f bool, n int) bool {
	wp := window.Active.CurrentWindow
	if wp == nil {
		return false
	}
	pageLines := int(wp.Height)
	if pageLines > 2 {
		pageLines = int(wp.Height - 2)
	} else {
		pageLines = 1
	}
	return CmdForwardLine(f, pageLines*n)
}

func CmdBackwardPage(f bool, n int) bool {
	wp := window.Active.CurrentWindow
	if wp == nil {
		return false
	}
	pageLines := int(wp.Height)
	if pageLines > 2 {
		pageLines = int(wp.Height - 2)
	} else {
		pageLines = 1
	}
	return CmdBackwardLine(f, pageLines*n)
}

func CmdForwardLine(f bool, n int) bool {
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil {
		return false
	}

	if wp.Cursor.Line+uint(n) <= bp.LineCount {
		wp.Cursor.Line += uint(n)
	} else {
		wp.Cursor.Line = bp.LineCount
	}

	line := bp.Line(wp.Cursor.Line)
	if line != nil && wp.Cursor.Offset > line.Len() {
		wp.Cursor.Offset = line.Len()
	}
	wp.DidMove = true
	return true
}

func CmdBackwardLine(f bool, n int) bool {
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil {
		return false
	}

	if wp.Cursor.Line > uint(n) {
		wp.Cursor.Line -= uint(n)
	} else {
		wp.Cursor.Line = 1
	}

	line := bp.Line(wp.Cursor.Line)
	if line != nil && wp.Cursor.Offset > line.Len() {
		wp.Cursor.Offset = line.Len()
	}
	wp.DidMove = true
	return true
}

func CmdGotoBol(f bool, n int) bool {
	wp := window.Active.CurrentWindow
	if wp != nil {
		wp.Cursor.Offset = 0
		wp.DidMove = true
	}
	return true
}

func CmdGotoEol(f bool, n int) bool {
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp != nil && bp != nil {
		line := bp.Line(wp.Cursor.Line)
		if line != nil {
			wp.Cursor.Offset = line.Len()
		} else {
			wp.Cursor.Offset = 0
		}
		wp.DidMove = true
	}
	return true
}

func CmdGotoBof(f bool, n int) bool {
	wp := window.Active.CurrentWindow
	if wp != nil {
		wp.Cursor.Line = 1
		wp.Cursor.Offset = 0
		wp.DidMove = true
	}
	return true
}

func CmdGotoEOF(f bool, n int) bool {
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp != nil && bp != nil {
		wp.Cursor.Line = bp.LineCount
		line := bp.Line(wp.Cursor.Line)
		if line != nil {
			wp.Cursor.Offset = line.Len()
		} else {
			wp.Cursor.Offset = 0
		}
		wp.DidMove = true
	}
	return true
}

// CmdBackToIndentation moves point to the first non-blank character on the line.
func CmdBackToIndentation(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
	if wp == nil {
		return false
	}
	lp := wp.Buffer.Line(wp.Cursor.Line)
	if lp != nil {
		wp.Cursor.Offset = lp.FirstNonblank()
	} else {
		wp.Cursor.Offset = 0
	}
	wp.DidMove = true
	return true
}

// CmdGotoLine jumps to a specific line number.
// CmdGotoLine jumps to a specific line number.
func CmdGotoLine(f bool, n int) bool {
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	var target uint
	if f {
		if n <= 0 {
			display.MBWrite("[line number out of range]")
			return false
		}
		target = uint(n)
	} else {
		AskStringCap("Goto line: ", "", 32, func(lineStr string, pr minibuffer.PromptResult) {
			if pr != minibuffer.PromptResultYes {
				return
			}
			parsed, ok := parsePositiveLineNumber(lineStr)
			if !ok {
				display.MBWrite("[invalid line number]")
				return
			}
			gotoLineNumber(bp, wp, parsed)
		})
		return true
	}
	return gotoLineNumber(bp, wp, target)
}

func gotoLineNumber(bp *buffer.Buffer, wp *window.Window, target uint) bool {
	if target > bp.LineCount {
		display.MBWrite("[line number out of range]")
		return false
	}
	if wp.Cursor.Line != target || wp.Cursor.Offset != 0 {
		markring.PushCurrent()
	}
	wp.SetCursor(buffer.MakeLocation(target, 0))
	wp.ShouldRedraw = true
	return true
}

func parsePositiveLineNumber(s string) (uint, bool) {
	if s == "" {
		return 0, false
	}
	var n uint
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + uint(c-'0')
		if n == 0 {
			return 0, false
		}
	}
	return n, true
}

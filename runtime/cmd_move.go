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
func forwardWordLoc(buf *buffer.Buffer, loc buffer.Location) buffer.Location {
	line := buf.Line(loc.Line)
	if line == nil {
		return loc
	}
	off := int(loc.Offset)
	// If past EOL, start at the next line; otherwise skip non-word on this line.
	if off >= len(line.Data) {
		if loc.Line >= len(buf.Lines) {
			return buffer.Location{Line: len(buf.Lines), Offset: line.Len()}
		}
		loc.Line++
		off = 0
	} else {
		for off < len(line.Data) && !isWordChar(line.Data[off]) {
			off++
		}
	}
	// Skip non-word then word, wrapping across lines as needed.
	for loc.Line <= len(buf.Lines) {
		line = buf.Line(loc.Line)
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
			return buffer.Location{Line: loc.Line, Offset: off}
		}
		if loc.Line >= len(buf.Lines) {
			return buffer.Location{Line: len(buf.Lines), Offset: 0}
		}
		loc.Line++
		off = 0
	}
	return buffer.Location{Line: len(buf.Lines), Offset: 0}
}

// move backward by one word: go left, skip non-word, then skip word backwards
func backwardWordLoc(buf *buffer.Buffer, loc buffer.Location) buffer.Location {
	if loc.Line == 1 && loc.Offset == 0 {
		return loc
	}
	line := buf.Line(loc.Line)
	if line == nil {
		return loc
	}
	off := int(loc.Offset)
	// Step left one position (if at offset 0, move to end of previous line).
	if off == 0 {
		if loc.Line > 1 {
			loc.Line--
			line = buf.Line(loc.Line)
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
		offPrev := buffer.PrevOffset(line.Data, off)
		if offPrev == off {
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
	return buffer.Location{Line: loc.Line, Offset: off}
}

// Move forward by a single codepoint, preserving UTF-8 boundaries.
func CmdForwardChar(f bool, n int) bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current

	for i := 0; i < n; i++ {
		line := buf.Line(win.Cursor.Line)
		if line != nil && win.Cursor.Offset < line.Len() {
			win.Cursor.Offset = buffer.NextOffset(line.Data, win.Cursor.Offset)
		} else if win.Cursor.Line < len(buf.Lines) {
			win.Cursor.Line++
			win.Cursor.Offset = 0
		} else {
			break
		}
	}
	win.DidMove = true
	return true
}

func CmdBackwardChar(f bool, n int) bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current

	for i := 0; i < n; i++ {
		line := buf.Line(win.Cursor.Line)
		if line != nil && win.Cursor.Offset > 0 {
			win.Cursor.Offset = buffer.PrevOffset(line.Data, win.Cursor.Offset)
		} else if win.Cursor.Line > 1 {
			win.Cursor.Line--
			prevLine := buf.Line(win.Cursor.Line)
			if prevLine != nil {
				win.Cursor.Offset = prevLine.Len()
			} else {
				win.Cursor.Offset = 0
			}
		} else {
			break
		}
	}
	win.DidMove = true
	return true
}

// CmdForwardWord moves forward by words (ASCII words: letters, digits, underscore)
func CmdForwardWord(f bool, n int) bool {
	_ = f
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	for i := 0; i < n; i++ {
		win.Cursor = forwardWordLoc(buf, win.Cursor)
	}
	win.DidMove = true
	return true
}

// CmdBackwardWord moves backward by words
func CmdBackwardWord(f bool, n int) bool {
	_ = f
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	for i := 0; i < n; i++ {
		win.Cursor = backwardWordLoc(buf, win.Cursor)
	}
	win.DidMove = true
	return true
}

// Page-wise movement
func CmdForwardPage(f bool, n int) bool {
	win := window.Active.CurrentWindow
	pageLines := win.Height
	if pageLines > 2 {
		pageLines = win.Height - 2
	} else {
		pageLines = 1
	}
	return CmdForwardLine(f, pageLines*n)
}

func CmdBackwardPage(f bool, n int) bool {
	win := window.Active.CurrentWindow
	pageLines := win.Height
	if pageLines > 2 {
		pageLines = win.Height - 2
	} else {
		pageLines = 1
	}
	return CmdBackwardLine(f, pageLines*n)
}

func CmdForwardLine(f bool, n int) bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current

	if win.Cursor.Line+n <= len(buf.Lines) {
		win.Cursor.Line += n
	} else {
		win.Cursor.Line = len(buf.Lines)
	}

	line := buf.Line(win.Cursor.Line)
	if line != nil && win.Cursor.Offset > line.Len() {
		win.Cursor.Offset = line.Len()
	}
	win.DidMove = true
	return true
}

func CmdBackwardLine(f bool, n int) bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current

	if win.Cursor.Line > n {
		win.Cursor.Line -= n
	} else {
		win.Cursor.Line = 1
	}

	line := buf.Line(win.Cursor.Line)
	if line != nil && win.Cursor.Offset > line.Len() {
		win.Cursor.Offset = line.Len()
	}
	win.DidMove = true
	return true
}

func CmdGotoBol(f bool, n int) bool {
	win := window.Active.CurrentWindow
	win.Cursor.Offset = 0
	win.DidMove = true
	return true
}

func CmdGotoEol(f bool, n int) bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	line := buf.Line(win.Cursor.Line)
	if line != nil {
		win.Cursor.Offset = line.Len()
	} else {
		win.Cursor.Offset = 0
	}
	win.DidMove = true
	return true
}

func CmdGotoBof(f bool, n int) bool {
	win := window.Active.CurrentWindow
	win.Cursor.Line = 1
	win.Cursor.Offset = 0
	win.DidMove = true
	return true
}

func CmdGotoEOF(f bool, n int) bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	win.Cursor.Line = len(buf.Lines)
	line := buf.Line(win.Cursor.Line)
	if line != nil {
		win.Cursor.Offset = line.Len()
	} else {
		win.Cursor.Offset = 0
	}
	win.DidMove = true
	return true
}

// CmdBackToIndentation moves point to the first non-blank character on the line.
func CmdBackToIndentation(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	line := win.Buffer.Line(win.Cursor.Line)
	if line != nil {
		win.Cursor.Offset = line.FirstNonblank()
	} else {
		win.Cursor.Offset = 0
	}
	win.DidMove = true
	return true
}

// CmdGotoLine jumps to a specific line number.
func CmdGotoLine(f bool, n int) bool {
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	var target int
	if f {
		if n <= 0 {
			display.MBWrite("[line number out of range]")
			return false
		}
		target = n
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
			gotoLineNumber(buf, win, parsed)
		})
		return true
	}
	return gotoLineNumber(buf, win, target)
}

func gotoLineNumber(buf *buffer.Buffer, win *window.Window, target int) bool {
	if target > len(buf.Lines) {
		display.MBWrite("[line number out of range]")
		return false
	}
	if win.Cursor.Line != target || win.Cursor.Offset != 0 {
		markring.PushCurrent()
	}
	win.SetCursor(buffer.MakeLocation(target, 0))
	win.ShouldRedraw = true
	return true
}

func parsePositiveLineNumber(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	var n int
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
		if n == 0 {
			return 0, false
		}
	}
	return n, true
}

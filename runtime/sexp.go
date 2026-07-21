package runtime

// Balanced-expression (sexp) movement.

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/window"
)

func cursorAtEob(win *window.Window) bool {
	if win == nil || win.Buffer == nil {
		return true
	}
	return win.Cursor.Line >= win.Buffer.EOF()
}

func cursorChar(win *window.Window, buf *buffer.Buffer) int {
	if cursorAtEob(win) {
		return -1
	}
	loc := win.Cursor
	line := buf.Line(loc.Line)
	if line == nil {
		return -1
	}
	if loc.Offset >= line.Len() {
		return '\n'
	}
	return int(line.Byte(loc.Offset))
}

func forwardSexpOnce(win *window.Window, buf *buffer.Buffer) bool {
	for {
		ch := cursorChar(win, buf)
		if ch < 0 {
			return false
		}
		if ch != ' ' && ch != '\t' && ch != '\n' {
			break
		}
		if !CmdForwardChar(false, 1) {
			return false
		}
	}
	loc := win.Cursor
	ch := cursorChar(win, buf)
	if ch == '(' || ch == '[' || ch == '{' {
		var match buffer.Location
		if !syntax.FindMatchingDelimiter(buf, loc, &match) {
			display.MBWrite("[no matching delimiter]")
			return false
		}
		mlp := buf.Line(match.Line)
		after := match.Offset + 1
		if mlp == nil || after > mlp.Len() {
			win.SetCursor(buffer.MakeLocation(match.Line+1, 0))
		} else {
			win.SetCursor(buffer.MakeLocation(match.Line, after))
		}
		win.DidMove = true
		return true
	}
	return CmdForwardWord(false, 1)
}

func backwardSexpOnce(win *window.Window, buf *buffer.Buffer) bool {
	orig := win.Cursor
	if !CmdBackwardChar(false, 1) {
		return false
	}
	for {
		ch := cursorChar(win, buf)
		if ch < 0 || (ch != ' ' && ch != '\t' && ch != '\n') {
			break
		}
		if !CmdBackwardChar(false, 1) {
			break
		}
	}
	loc := win.Cursor
	ch := cursorChar(win, buf)
	if ch == ')' || ch == ']' || ch == '}' {
		var match buffer.Location
		if !syntax.FindMatchingDelimiter(buf, loc, &match) {
			display.MBWrite("[no matching delimiter]")
			win.SetCursor(orig)
			return false
		}
		win.SetCursor(match)
		win.DidMove = true
		return true
	}
	win.SetCursor(orig)
	return CmdBackwardWord(false, 1)
}

// CmdForwardSexp moves past the balanced expression at/after point.
func CmdForwardSexp(f bool, n int) bool {
	_ = f
	if n < 0 {
		return CmdBackwardSexp(false, -n)
	}
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil {
		return false
	}
	if n == 0 {
		return true
	}
	for i := 0; i < n; i++ {
		if !forwardSexpOnce(win, buf) {
			return false
		}
	}
	return true
}

// CmdBackwardSexp moves back past the balanced expression before point.
func CmdBackwardSexp(f bool, n int) bool {
	_ = f
	if n < 0 {
		return CmdForwardSexp(false, -n)
	}
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil {
		return false
	}
	if n == 0 {
		return true
	}
	for i := 0; i < n; i++ {
		if !backwardSexpOnce(win, buf) {
			return false
		}
	}
	return true
}

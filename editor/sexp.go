package editor

// sexp.go — balanced-expression movement (translation of cmd_forward/backward_sexp in src/cmd_move.c)

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/ui"
)

import "github.com/jdpalmer/jem/app"

func cursorAtEob(wp *app.Window) bool {
	if wp == nil || wp.Buffer == nil {
		return true
	}
	return wp.Cursor.Line >= wp.Buffer.EOF()
}

func cursorChar(wp *app.Window, bp *buffer.Buffer) int {
	if cursorAtEob(wp) {
		return -1
	}
	loc := wp.Cursor
	lp := bp.Line(loc.Line)
	if lp == nil {
		return -1
	}
	if loc.Offset >= lp.Len() {
		return '\n'
	}
	return int(lp.Byte(loc.Offset))
}

func forwardSexpOnce(wp *app.Window, bp *buffer.Buffer) bool {
	for {
		ch := cursorChar(wp, bp)
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
	loc := wp.Cursor
	ch := cursorChar(wp, bp)
	if ch == '(' || ch == '[' || ch == '{' {
		var match buffer.Location
		if !syntax.FindMatchingDelimiter(bp, loc, &match) {
			ui.MBWrite("[no matching delimiter]")
			return false
		}
		mlp := bp.Line(match.Line)
		after := match.Offset + 1
		if mlp == nil || after > mlp.Len() {
			wp.SetCursor(buffer.MakeLocation(match.Line+1, 0))
		} else {
			wp.SetCursor(buffer.MakeLocation(match.Line, after))
		}
		wp.DidMove = true
		return true
	}
	return CmdForwardWord(false, 1)
}

func backwardSexpOnce(wp *app.Window, bp *buffer.Buffer) bool {
	orig := wp.Cursor
	if !CmdBackwardChar(false, 1) {
		return false
	}
	for {
		ch := cursorChar(wp, bp)
		if ch < 0 || (ch != ' ' && ch != '\t' && ch != '\n') {
			break
		}
		if !CmdBackwardChar(false, 1) {
			break
		}
	}
	loc := wp.Cursor
	ch := cursorChar(wp, bp)
	if ch == ')' || ch == ']' || ch == '}' {
		var match buffer.Location
		if !syntax.FindMatchingDelimiter(bp, loc, &match) {
			ui.MBWrite("[no matching delimiter]")
			wp.SetCursor(orig)
			return false
		}
		wp.SetCursor(match)
		wp.DidMove = true
		return true
	}
	wp.SetCursor(orig)
	return CmdBackwardWord(false, 1)
}

// CmdForwardSexp moves past the balanced expression at/after point.
func CmdForwardSexp(f bool, n int) bool {
	_ = f
	if n < 0 {
		return CmdBackwardSexp(false, -n)
	}
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	if n == 0 {
		return true
	}
	for i := 0; i < n; i++ {
		if !forwardSexpOnce(wp, bp) {
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
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	if n == 0 {
		return true
	}
	for i := 0; i < n; i++ {
		if !backwardSexpOnce(wp, bp) {
			return false
		}
	}
	return true
}

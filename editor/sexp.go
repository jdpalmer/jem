package editor

// sexp.go — balanced-expression movement (translation of cmd_forward/backward_sexp in src/cmd_move.c)

func cursorAtEob(wp *Window) bool {
	if wp == nil || wp.Buffer == nil {
		return true
	}
	return wp.Cursor.Line >= BufferEOF(wp.Buffer)
}

func cursorChar(wp *Window, bp *Buffer) int {
	if cursorAtEob(wp) {
		return -1
	}
	loc := wp.Cursor
	lp := BufferGetLine(bp, loc.Line)
	if lp == nil {
		return -1
	}
	if loc.Offset >= LineLength(lp) {
		return '\n'
	}
	return int(LineGetc(lp, loc.Offset))
}

func forwardSexpOnce(wp *Window, bp *Buffer) bool {
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
		var match Location
		if !syntaxFindMatchingDelimiter(bp, loc, &match) {
			mbWrite("[no matching delimiter]")
			return false
		}
		mlp := BufferGetLine(bp, match.Line)
		after := match.Offset + 1
		if mlp == nil || after > LineLength(mlp) {
			windowSetCursor(wp, MakeLocation(match.Line+1, 0))
		} else {
			windowSetCursor(wp, MakeLocation(match.Line, after))
		}
		wp.DidMove = true
		return true
	}
	return CmdForwardWord(false, 1)
}

func backwardSexpOnce(wp *Window, bp *Buffer) bool {
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
		var match Location
		if !syntaxFindMatchingDelimiter(bp, loc, &match) {
			mbWrite("[no matching delimiter]")
			windowSetCursor(wp, orig)
			return false
		}
		windowSetCursor(wp, match)
		wp.DidMove = true
		return true
	}
	windowSetCursor(wp, orig)
	return CmdBackwardWord(false, 1)
}

// CmdForwardSexp moves past the balanced expression at/after point.
func CmdForwardSexp(f bool, n int) bool {
	_ = f
	if n < 0 {
		return CmdBackwardSexp(false, -n)
	}
	wp := session.App.CurrentWindow
	bp := session.App.CurrentBuffer
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
	wp := session.App.CurrentWindow
	bp := session.App.CurrentBuffer
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

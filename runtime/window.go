package runtime

// window.go - Window management and layout tiling (translation of window.c and part of display.c)

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/window"
)

func CmdWindowDelete(f bool, n int) bool {
	if len(window.Active.Windows) <= 1 {
		display.MBWrite("[cannot remove only window]")
		return false
	}

	previousWindow := window.Active.CurrentWindow
	for i := len(window.Active.Windows) - 1; i >= 0; i-- {
		swap := window.Active.Windows[i]
		window.Active.Windows[i] = previousWindow
		previousWindow = swap

		if previousWindow == window.Active.CurrentWindow {
			previousWindow.SaveState()
			window.Active.CurrentWindow = window.Active.Windows[i]
			window.Active.Windows = window.Active.Windows[:len(window.Active.Windows)-1]
			window.WindowSelect(window.Active.CurrentWindow)
			window.WindowRetile()
			break
		}
	}
	return true
}

func CmdWindowNext(f bool, n int) bool {
	if len(window.Active.Windows) <= 1 {
		return true
	}
	next := window.Active.Windows[0]
	for i := 0; i < len(window.Active.Windows); i++ {
		if window.Active.Windows[i] == window.Active.CurrentWindow {
			if i+1 < len(window.Active.Windows) {
				next = window.Active.Windows[i+1]
			}
		}
	}
	window.WindowSelect(next)
	return true
}

func CmdWindowOnly(f bool, n int) bool {
	for i := 0; i < len(window.Active.Windows); i++ {
		if window.Active.Windows[i] == window.Active.CurrentWindow {
			window.Active.Windows[i] = window.Active.Windows[0]
			window.Active.Windows[0] = window.Active.CurrentWindow
			continue
		}
		window.Active.Windows[i].SaveState()
	}
	window.Active.Windows = window.Active.Windows[:1]
	window.Active.CurrentWindow = window.Active.Windows[0]
	window.WindowSelect(window.Active.CurrentWindow)
	window.WindowRetile()
	return true
}

func CmdWindowSplit(f bool, n int) bool {
	if term.Rows() < 4*(len(window.Active.Windows)+1) {
		display.MBWrite("[window is too small to split]")
		return false
	}
	wp := window.WindowCreate()
	if wp == nil {
		display.MBWrite("[maximum number of windows has been reached]")
		return false
	}

	curr := window.Active.CurrentWindow
	wp.Buffer = buffer.All.Current
	wp.TopLine = curr.TopLine
	wp.Cursor = curr.Cursor
	wp.Mark = curr.Mark
	wp.ScreenTopRow = curr.ScreenTopRow
	wp.Height = curr.Height
	wp.HScroll = curr.HScroll

	// Insert wp next to curr in Windows slice
	for i := len(window.Active.Windows) - 1; i > 0; i-- {
		if window.Active.Windows[i-1] == curr {
			window.Active.Windows[i] = wp
			break
		}
		window.Active.Windows[i] = window.Active.Windows[i-1]
	}

	window.WindowRetile()
	return true
}

// windowInsertText inserts text at the window cursor. Returns true on success.
func windowInsertText(wp *window.Window, text []byte) bool {
	return window.InsertText(wp, text)
}

// windowInsertCodepoint inserts a Unicode codepoint at the window cursor.
func windowInsertCodepoint(wp *window.Window, cp rune) bool {
	return window.InsertCodepoint(wp, cp)
}

// windowInsertNewline inserts a single newline at the window cursor.
func windowInsertNewline(wp *window.Window) bool {
	return window.InsertNewline(wp)
}

// windowReplaceLineLeadingText replaces the leading whitespace on the current line with the given text.
func windowReplaceLineLeadingText(wp *window.Window, text []byte) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	lineNumber := wp.Cursor.Line
	lp := wp.Buffer.Line(lineNumber)
	var oldWS uint = 0
	if lp != nil {
		oldWS = lp.FirstNonblank()
	}
	if oldWS == 0 && len(text) == 0 {
		return true
	}
	begin := buffer.Location{Line: lineNumber, Offset: 0}
	end := buffer.Location{Line: lineNumber, Offset: oldWS}
	return bufferSetText(wp.Buffer, begin, end, text, nil, false)
}

// windowSetLineIndent sets the indentation of the current line using a count of
// leading tabs and spaces, mirroring the behavior of the original C helper.
func windowSetLineIndent(wp *window.Window, tabs, spaces uint) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	// Build indent bytes: tabs first then spaces.
	tot := tabs + spaces
	if tot == 0 {
		// No-op if line already has no leading whitespace; otherwise clear it.
		lp := wp.Buffer.Line(wp.Cursor.Line)
		if lp == nil {
			return false
		}
		old := lp.FirstNonblank()
		if old == 0 {
			return true
		}
		begin := buffer.MakeLocation(wp.Cursor.Line, 0)
		end := buffer.MakeLocation(wp.Cursor.Line, old)
		BeginCommand()
		ok := bufferSetText(wp.Buffer, begin, end, nil, nil, false)
		EndCommand()
		if ok {
			wp.DidEdit = true
		}
		return ok
	}
	indent := make([]byte, 0, tot)
	for i := uint(0); i < tabs; i++ {
		indent = append(indent, '\t')
	}
	for i := uint(0); i < spaces; i++ {
		indent = append(indent, ' ')
	}
	lp := wp.Buffer.Line(wp.Cursor.Line)
	if lp == nil {
		return false
	}
	old := lp.FirstNonblank()
	begin := buffer.MakeLocation(wp.Cursor.Line, 0)
	end := buffer.MakeLocation(wp.Cursor.Line, old)
	BeginCommand()
	ok := bufferSetText(wp.Buffer, begin, end, indent, nil, false)
	EndCommand()
	if ok {
		wp.DidEdit = true
	}
	return ok
}

// windowDeleteChars deletes up to count characters starting at the cursor.
func windowDeleteChars(wp *window.Window, count int) bool {
	if wp == nil || wp.Buffer == nil || count <= 0 {
		return false
	}
	bp := wp.Buffer
	BeginCommand()
	defer EndCommand()
	deleted := false
	for i := 0; i < count; i++ {
		line := bp.Line(wp.Cursor.Line)
		if line != nil && wp.Cursor.Offset < line.Len() {
			begin := buffer.Location{Line: wp.Cursor.Line, Offset: wp.Cursor.Offset}
			end := buffer.Location{Line: wp.Cursor.Line, Offset: wp.Cursor.Offset + 1}
			var newEnd buffer.Location
			if bufferSetText(bp, begin, end, nil, &newEnd, false) {
				wp.Cursor = newEnd
				deleted = true
				continue
			}
			break
		} else if wp.Cursor.Line < bp.LineCount {
			begin := buffer.Location{Line: wp.Cursor.Line, Offset: wp.Cursor.Offset}
			end := buffer.Location{Line: wp.Cursor.Line + 1, Offset: 0}
			var newEnd buffer.Location
			if bufferSetText(bp, begin, end, nil, &newEnd, false) {
				wp.Cursor = newEnd
				deleted = true
				continue
			}
			break
		} else {
			break
		}
	}
	if deleted {
		wp.DidEdit = true
	}
	return deleted
}

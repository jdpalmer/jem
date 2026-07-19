package editor

// window.go - Window management and layout tiling (translation of window.c and part of display.c)

import (
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/view"
)

func CmdWindowDelete(f bool, n int) bool {
	if len(model.State.Windows) <= 1 {
		view.MBWrite("[cannot remove only window]")
		return false
	}

	previousWindow := model.State.CurrentWindow
	for i := len(model.State.Windows) - 1; i >= 0; i-- {
		swap := model.State.Windows[i]
		model.State.Windows[i] = previousWindow
		previousWindow = swap

		if previousWindow == model.State.CurrentWindow {
			previousWindow.SaveState()
			model.State.CurrentWindow = model.State.Windows[i]
			model.State.Windows = model.State.Windows[:len(model.State.Windows)-1]
			model.WindowSelect(model.State.CurrentWindow)
			model.WindowRetile()
			break
		}
	}
	return true
}

func CmdWindowNext(f bool, n int) bool {
	if len(model.State.Windows) <= 1 {
		return true
	}
	next := model.State.Windows[0]
	for i := 0; i < len(model.State.Windows); i++ {
		if model.State.Windows[i] == model.State.CurrentWindow {
			if i+1 < len(model.State.Windows) {
				next = model.State.Windows[i+1]
			}
		}
	}
	model.WindowSelect(next)
	return true
}

func CmdWindowOnly(f bool, n int) bool {
	for i := 0; i < len(model.State.Windows); i++ {
		if model.State.Windows[i] == model.State.CurrentWindow {
			model.State.Windows[i] = model.State.Windows[0]
			model.State.Windows[0] = model.State.CurrentWindow
			continue
		}
		model.State.Windows[i].SaveState()
	}
	model.State.Windows = model.State.Windows[:1]
	model.State.CurrentWindow = model.State.Windows[0]
	model.WindowSelect(model.State.CurrentWindow)
	model.WindowRetile()
	return true
}

func CmdWindowSplit(f bool, n int) bool {
	if term.Rows() < 4*(len(model.State.Windows)+1) {
		view.MBWrite("[window is too small to split]")
		return false
	}
	wp := model.WindowCreate()
	if wp == nil {
		view.MBWrite("[maximum number of windows has been reached]")
		return false
	}

	curr := model.State.CurrentWindow
	wp.Buffer = model.State.CurrentBuffer
	wp.TopLine = curr.TopLine
	wp.Cursor = curr.Cursor
	wp.Mark = curr.Mark
	wp.ScreenTopRow = curr.ScreenTopRow
	wp.Height = curr.Height
	wp.HScroll = curr.HScroll

	// Insert wp next to curr in Windows slice
	for i := len(model.State.Windows) - 1; i > 0; i-- {
		if model.State.Windows[i-1] == curr {
			model.State.Windows[i] = wp
			break
		}
		model.State.Windows[i] = model.State.Windows[i-1]
	}

	model.WindowRetile()
	return true
}

// windowInsertText inserts text at the window cursor. Returns true on success.
func windowInsertText(wp *model.Window, text []byte) bool {
	return model.InsertText(wp, text)
}

// windowInsertCodepoint inserts a Unicode codepoint at the window cursor.
func windowInsertCodepoint(wp *model.Window, cp rune) bool {
	return model.InsertCodepoint(wp, cp)
}

// windowInsertNewline inserts a single newline at the window cursor.
func windowInsertNewline(wp *model.Window) bool {
	return model.InsertNewline(wp)
}

// windowReplaceLineLeadingText replaces the leading whitespace on the current line with the given text.
func windowReplaceLineLeadingText(wp *model.Window, text []byte) bool {
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
func windowSetLineIndent(wp *model.Window, tabs, spaces uint) bool {
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
		model.BeginCommand()
		ok := bufferSetText(wp.Buffer, begin, end, nil, nil, false)
		model.EndCommand()
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
	model.BeginCommand()
	ok := bufferSetText(wp.Buffer, begin, end, indent, nil, false)
	model.EndCommand()
	if ok {
		wp.DidEdit = true
	}
	return ok
}

// windowDeleteChars deletes up to count characters starting at the cursor.
func windowDeleteChars(wp *model.Window, count int) bool {
	if wp == nil || wp.Buffer == nil || count <= 0 {
		return false
	}
	bp := wp.Buffer
	model.BeginCommand()
	defer model.EndCommand()
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

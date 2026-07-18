package editor

// window.go - Window management and layout tiling (translation of window.c and part of display.c)

import (
	"bytes"
	"github.com/jdpalmer/jem/buffer"
	"unicode/utf8"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/term"
)

func CmdWindowDelete(f bool, n int) bool {
	if app.State.WindowCount <= 1 {
		mbWrite("[cannot remove only window]")
		return false
	}

	previousWindow := app.State.CurrentWindow
	for i := int(app.State.WindowCount) - 1; i >= 0; i-- {
		swap := app.State.WINDOWS[i]
		app.State.WINDOWS[i] = previousWindow
		previousWindow = swap

		if previousWindow == app.State.CurrentWindow {
			app.WindowSaveState(previousWindow)
			app.State.CurrentWindow = app.State.WINDOWS[i]
			app.State.WindowCount--
			app.WindowSelect(app.State.CurrentWindow)
			app.WindowRetile()
			break
		}
	}
	return true
}

func CmdWindowNext(f bool, n int) bool {
	if app.State.WindowCount <= 1 {
		return true
	}
	next := app.State.WINDOWS[0]
	for i := 0; i < int(app.State.WindowCount); i++ {
		if app.State.WINDOWS[i] == app.State.CurrentWindow {
			if i+1 < int(app.State.WindowCount) {
				next = app.State.WINDOWS[i+1]
			}
		}
	}
	app.WindowSelect(next)
	return true
}

func CmdWindowOnly(f bool, n int) bool {
	for i := 0; i < int(app.State.WindowCount); i++ {
		if app.State.WINDOWS[i] == app.State.CurrentWindow {
			app.State.WINDOWS[i] = app.State.WINDOWS[0]
			app.State.WINDOWS[0] = app.State.CurrentWindow
			continue
		}
		app.WindowSaveState(app.State.WINDOWS[i])
	}
	app.State.WindowCount = 1
	app.State.CurrentWindow = app.State.WINDOWS[0]
	app.WindowSelect(app.State.CurrentWindow)
	app.WindowRetile()
	return true
}

func CmdWindowSplit(f bool, n int) bool {
	if term.Rows() < 4*(int(app.State.WindowCount)+1) {
		mbWrite("[window is too small to split]")
		return false
	}
	wp := app.WindowCreate()
	if wp == nil {
		mbWrite("[maximum number of windows has been reached]")
		return false
	}

	curr := app.State.CurrentWindow
	wp.Buffer = app.State.CurrentBuffer
	wp.TopLine = curr.TopLine
	wp.Cursor = curr.Cursor
	wp.Mark = curr.Mark
	wp.ScreenTopRow = curr.ScreenTopRow
	wp.Height = curr.Height
	wp.HScroll = curr.HScroll

	// Insert wp next to curr in WINDOWS array
	for i := int(app.State.WindowCount) - 1; i > 0; i-- {
		if app.State.WINDOWS[i-1] == curr {
			app.State.WINDOWS[i] = wp
			break
		}
		app.State.WINDOWS[i] = app.State.WINDOWS[i-1]
	}

	app.WindowRetile()
	return true
}

// windowInsertText inserts text at the window cursor. Returns true on success.
func windowInsertText(wp *Window, text []byte, length int) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	bp := wp.Buffer
	UndoBeginCommand()
	defer UndoEndCommand()
	begin := wp.Cursor
	var newEnd Location
	if !bufferSetText(bp, begin, begin, text, uint(length), &newEnd, false) {
		return false
	}
	// Set cursor to the precise new end location returned by bufferSetText.
	wp.Cursor = newEnd
	wp.DidEdit = true
	return true
}

// windowInsertCodepoint inserts a Unicode codepoint at the window cursor.
func windowInsertCodepoint(wp *Window, cp rune) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	if cp < 0 {
		return false
	}
	// Fast path for ASCII
	if cp < 0x80 {
		return windowInsertText(wp, []byte{byte(cp)}, 1)
	}
	buf := make([]byte, utf8.RuneLen(cp))
	n := utf8.EncodeRune(buf, cp)
	return windowInsertText(wp, buf, n)
}

// windowInsertNewline inserts a single newline at the window cursor.
func windowInsertNewline(wp *Window) bool {
	return windowInsertText(wp, []byte{'\n'}, 1)
}

// windowReplaceLineLeadingText replaces the leading whitespace on the current line with the given text.
func windowReplaceLineLeadingText(wp *Window, text []byte, length int) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	lineNumber := wp.Cursor.Line
	lp := buffer.GetLine(wp.Buffer, lineNumber)
	var oldWS uint = 0
	if lp != nil {
		oldWS = buffer.LineFirstNonblank(lp)
	}
	if oldWS == 0 && length == 0 {
		return true
	}
	begin := Location{Line: lineNumber, Offset: 0}
	end := Location{Line: lineNumber, Offset: oldWS}
	return bufferSetText(wp.Buffer, begin, end, text, uint(length), nil, false)
}

// windowSetLineIndent sets the indentation of the current line using a count of
// leading tabs and spaces, mirroring the behavior of the original C helper.
func windowSetLineIndent(wp *Window, tabs, spaces uint) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	// Build indent bytes: tabs first then spaces.
	tot := tabs + spaces
	if tot == 0 {
		// No-op if line already has no leading whitespace; otherwise clear it.
		lp := buffer.GetLine(wp.Buffer, wp.Cursor.Line)
		if lp == nil {
			return false
		}
		old := buffer.LineFirstNonblank(lp)
		if old == 0 {
			return true
		}
		begin := buffer.MakeLocation(wp.Cursor.Line, 0)
		end := buffer.MakeLocation(wp.Cursor.Line, old)
		UndoBeginCommand()
		ok := bufferSetText(wp.Buffer, begin, end, nil, 0, nil, false)
		UndoEndCommand()
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
	lp := buffer.GetLine(wp.Buffer, wp.Cursor.Line)
	if lp == nil {
		return false
	}
	old := buffer.LineFirstNonblank(lp)
	begin := buffer.MakeLocation(wp.Cursor.Line, 0)
	end := buffer.MakeLocation(wp.Cursor.Line, old)
	UndoBeginCommand()
	ok := bufferSetText(wp.Buffer, begin, end, indent, uint(len(indent)), nil, false)
	UndoEndCommand()
	if ok {
		wp.DidEdit = true
	}
	return ok
}

// windowDeleteChars deletes up to count characters starting at the cursor.
func windowDeleteChars(wp *Window, count int) bool {
	if wp == nil || wp.Buffer == nil || count <= 0 {
		return false
	}
	bp := wp.Buffer
	UndoBeginCommand()
	defer UndoEndCommand()
	deleted := false
	for i := 0; i < count; i++ {
		line := buffer.GetLine(bp, wp.Cursor.Line)
		if line != nil && wp.Cursor.Offset < buffer.LineLength(line) {
			begin := Location{Line: wp.Cursor.Line, Offset: wp.Cursor.Offset}
			end := Location{Line: wp.Cursor.Line, Offset: wp.Cursor.Offset + 1}
			var newEnd Location
			if bufferSetText(bp, begin, end, nil, 0, &newEnd, false) {
				wp.Cursor = newEnd
				deleted = true
				continue
			}
			break
		} else if wp.Cursor.Line < bp.LineCount {
			begin := Location{Line: wp.Cursor.Line, Offset: wp.Cursor.Offset}
			end := Location{Line: wp.Cursor.Line + 1, Offset: 0}
			var newEnd Location
			if bufferSetText(bp, begin, end, nil, 0, &newEnd, false) {
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

// editorInsertPaste inserts pasted text into the current window, normalizing CR/CRLF to LF.
func editorInsertPaste(text []byte, length int) bool {
	wp := app.State.CurrentWindow
	if wp == nil || wp.Buffer == nil {
		return false
	}
	p := text
	if length < 0 || length > len(text) {
		length = len(text)
	}
	p = p[:length]
	// Normalize CRLF -> LF, and lone CR -> LF
	p = bytes.ReplaceAll(p, []byte("\r\n"), []byte("\n"))
	p = bytes.ReplaceAll(p, []byte("\r"), []byte("\n"))
	ok := windowInsertText(wp, p, len(p))
	return ok
}

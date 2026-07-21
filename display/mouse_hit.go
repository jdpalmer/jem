package display

// Screen geometry helpers for mouse hit-testing.

import (
	"github.com/jdpalmer/jem/window"
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
)

// WinAt returns the window that occupies screen row r, or nil.
func WinAt(r uint32) *window.Window {
	for i := 0; i < int(len(window.Active.Windows)); i++ {
		win := window.Active.Windows[i]
		if win != nil && r >= win.ScreenTopRow && r < win.ScreenTopRow+win.Height {
			return win
		}
	}
	return nil
}

// lineOffsetAtCol returns the byte offset at or before the given screen column goal.
func lineOffsetAtCol(line *buffer.Line, goal uint32) uint {
	if line == nil {
		return 0
	}
	var col uint = 0
	var dbo uint = 0
	goalCol := goal
	if goalCol > 0x7FFFFFFF {
		goalCol = 0x7FFFFFFF
	}
	for dbo < line.Len() {
		newCol := col
		var used uint = 1
		b := line.Data[dbo]
		if b < 0x80 {
			newCol = uint(lineMeasureAdvance(int(col), rune(b)))
		} else {
			r, size := utf8.DecodeRune(line.Data[dbo:])
			newCol = uint(lineMeasureAdvance(int(col), r))
			used = uint(size)
		}
		if newCol > uint(goalCol) {
			break
		}
		col = newCol
		dbo += used
	}
	return dbo
}

// MouseLocationInWindow maps screen mouse coordinates to a buffer location within win.
func MouseLocationInWindow(win *window.Window) buffer.Location {
	if win == nil {
		return buffer.Location{}
	}
	rowInWin := Active.Mouse.Row - win.ScreenTopRow
	lineNumber := win.TopLine
	for rowInWin > 0 && lineNumber < win.Buffer.LineCount {
		lineNumber++
		rowInWin--
	}

	loc := buffer.Location{Line: lineNumber, Offset: 0}
	line := win.Buffer.Line(loc.Line)
	textCol := int(Active.Mouse.Col) - int(win.GutterWidth()) + int(win.HScroll)
	if textCol < 0 {
		textCol = 0
	}
	if line != nil {
		loc.Offset = lineOffsetAtCol(line, uint32(textCol))
	}
	return loc
}

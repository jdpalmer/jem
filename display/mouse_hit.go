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
		wp := window.Active.Windows[i]
		if wp != nil && r >= wp.ScreenTopRow && r < wp.ScreenTopRow+wp.Height {
			return wp
		}
	}
	return nil
}

// lineOffsetAtCol returns the byte offset at or before the given screen column goal.
func lineOffsetAtCol(lp *buffer.Line, goal uint32) uint {
	if lp == nil {
		return 0
	}
	var col uint = 0
	var dbo uint = 0
	goalCol := goal
	if goalCol > 0x7FFFFFFF {
		goalCol = 0x7FFFFFFF
	}
	for dbo < lp.Len() {
		newCol := col
		var used uint = 1
		b := lp.Data[dbo]
		if b < 0x80 {
			newCol = uint(lineMeasureAdvance(int(col), rune(b)))
		} else {
			r, size := utf8.DecodeRune(lp.Data[dbo:])
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

// MouseLocationInWindow maps screen mouse coordinates to a buffer location within wp.
func MouseLocationInWindow(wp *window.Window) buffer.Location {
	if wp == nil {
		return buffer.Location{}
	}
	rowInWin := Active.Mouse.Row - wp.ScreenTopRow
	lineNumber := wp.TopLine
	for rowInWin > 0 && lineNumber < wp.Buffer.LineCount {
		lineNumber++
		rowInWin--
	}

	loc := buffer.Location{Line: lineNumber, Offset: 0}
	line := wp.Buffer.Line(loc.Line)
	textCol := int(Active.Mouse.Col) - int(wp.GutterWidth()) + int(wp.HScroll)
	if textCol < 0 {
		textCol = 0
	}
	if line != nil {
		loc.Offset = lineOffsetAtCol(line, uint32(textCol))
	}
	return loc
}

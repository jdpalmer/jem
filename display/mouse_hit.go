package display

// Screen geometry helpers for mouse hit-testing.

import (
	"github.com/jdpalmer/jem/window"
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
)

// WinAt returns the window that occupies screen row r, or nil.
func WinAt(r int) *window.Window {
	for i := 0; i < len(window.Active.Windows); i++ {
		win := window.Active.Windows[i]
		if win != nil && r >= win.ScreenTopRow && r < win.ScreenTopRow+win.Height {
			return win
		}
	}
	return nil
}

// lineOffsetAtCol returns the byte offset at or before the given screen column goal.
func lineOffsetAtCol(line *buffer.Line, goal int) int {
	if line == nil {
		return 0
	}
	col := 0
	dbo := 0
	for dbo < line.Len() {
		newCol := col
		used := 1
		b := line.Data[dbo]
		if b < 0x80 {
			newCol = lineMeasureAdvance(col, rune(b))
		} else {
			r, size := utf8.DecodeRune(line.Data[dbo:])
			newCol = lineMeasureAdvance(col, r)
			used = size
		}
		if newCol > goal {
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
	for rowInWin > 0 && lineNumber < len(win.Buffer.Lines) {
		lineNumber++
		rowInWin--
	}

	loc := buffer.Location{Line: lineNumber, Offset: 0}
	line := win.Buffer.Line(loc.Line)
	textCol := Active.Mouse.Col - win.GutterWidth() + win.HScroll
	if textCol < 0 {
		textCol = 0
	}
	if line != nil {
		loc.Offset = lineOffsetAtCol(line, textCol)
	}
	return loc
}

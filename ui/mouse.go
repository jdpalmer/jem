package ui

// mouse.go - Mouse command implementations (translation of mouse.c)

import (
	"unicode/utf8"

	"github.com/jdpalmer/jem/app"
)

const wheelLines = 3

var mouseAnchorWindow *Window
var mouseAnchorLine uint
var mouseAnchorOffset uint

// winAt returns the window that occupies screen row r, or nil.
func winAt(r uint32) *Window {
	for i := 0; i < int(app.State.WindowCount); i++ {
		wp := app.State.WINDOWS[i]
		if wp != nil && r >= wp.ScreenTopRow && r < wp.ScreenTopRow+wp.Height {
			return wp
		}
	}
	return nil
}

// lineOffsetAtCol returns the byte offset at or before the given screen column goal.
func lineOffsetAtCol(lp *Line, goal uint32) uint {
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

// mouseLocationInWindow maps screen mouse coordinates to a buffer location within a specific window.
func mouseLocationInWindow(wp *Window) Location {
	if wp == nil {
		return Location{}
	}
	rowInWin := app.State.Mouse.Row - wp.ScreenTopRow
	lineNumber := wp.TopLine
	for rowInWin > 0 && lineNumber < wp.Buffer.LineCount {
		lineNumber++
		rowInWin--
	}

	loc := Location{Line: lineNumber, Offset: 0}
	line := wp.Buffer.Line(loc.Line)
	textCol := int(app.State.Mouse.Col) - int(wp.GutterWidth()) + int(wp.HScroll)
	if textCol < 0 {
		textCol = 0
	}
	if line != nil {
		loc.Offset = lineOffsetAtCol(line, uint32(textCol))
	}
	return loc
}

// windowCursorIsVisible checks if the window's cursor is currently within the visible viewport.
func windowCursorIsVisible(wp *Window) bool {
	if wp == nil {
		return false
	}
	visibleLine := wp.TopLine
	for i := uint32(0); i < wp.Height; i++ {
		if visibleLine == wp.Cursor.Line {
			return true
		}
		if visibleLine > wp.Buffer.LineCount {
			break
		}
		visibleLine++
	}
	return false
}

// windowLastVisibleLine finds the last line index currently visible within the window's viewport.
func windowLastVisibleLine(wp *Window) uint {
	if wp == nil {
		return 1
	}
	visibleLine := wp.TopLine
	lastVisible := wp.TopLine
	for i := uint32(0); i < wp.Height; i++ {
		if visibleLine > wp.Buffer.LineCount {
			break
		}
		lastVisible = visibleLine
		visibleLine++
	}
	return lastVisible
}

// windowScroll scrolls the window viewport by n lines (positive for down, negative for up).
func windowScroll(wp *Window, n int) {
	if wp == nil || wp.Buffer == nil {
		return
	}
	lineNumber := wp.TopLine
	if n > 0 {
		for i := 0; i < n && lineNumber < wp.Buffer.LineCount; i++ {
			lineNumber++
		}
	} else {
		for i := 0; i > n && lineNumber > 1; i-- {
			lineNumber--
		}
	}
	if lineNumber < 1 {
		lineNumber = 1
	}
	if lineNumber > wp.Buffer.LineCount {
		lineNumber = wp.Buffer.LineCount
	}
	wp.SetTopLine(lineNumber)
	wp.ShouldRedraw = true

	if wp == app.State.CurrentWindow {
		if !windowCursorIsVisible(wp) {
			var loc Location
			if n > 0 {
				loc = Location{Line: windowLastVisibleLine(wp), Offset: 0}
			} else {
				loc = Location{Line: wp.TopLine, Offset: 0}
			}
			wp.SetCursor(loc)
		}
	}
}

// CmdMouseLeft moves point to the clicked position.
// If the click falls in a window other than the current one, that window becomes current.
func CmdMouseLeft(f bool, n int) bool {
	_ = f
	_ = n
	wp := winAt(app.State.Mouse.Row)
	if wp == nil {
		return false
	}

	if wp != app.State.CurrentWindow {
		app.WindowSelect(wp)
	}

	loc := mouseLocationInWindow(wp)
	app.State.CurrentWindow.SetCursor(loc)
	app.State.CurrentWindow.Mark.Line = 0
	app.State.CurrentWindow.Mark.Offset = 0
	mouseAnchorWindow = app.State.CurrentWindow
	mouseAnchorLine = loc.Line
	mouseAnchorOffset = loc.Offset
	app.State.CurrentWindow.ShouldRedraw = true
	app.State.CurrentWindow.ShouldUpdateModeLine = true
	return true
}

// CmdMouseDrag extends the selection by moving point to the current mouse position.
func CmdMouseDrag(f bool, n int) bool {
	_ = f
	_ = n
	wp := winAt(app.State.Mouse.Row)
	if wp == nil || wp != app.State.CurrentWindow || mouseAnchorWindow != app.State.CurrentWindow {
		return false
	}

	loc := mouseLocationInWindow(wp)

	app.State.CurrentWindow.Mark.Line = mouseAnchorLine
	app.State.CurrentWindow.Mark.Offset = mouseAnchorOffset
	app.State.CurrentWindow.SetCursor(loc)
	app.State.CurrentWindow.ShouldRedraw = true
	app.State.CurrentWindow.ShouldUpdateModeLine = true
	return true
}

// CmdMouseWheelUp scrolls the viewport toward the beginning of the buffer.
func CmdMouseWheelUp(f bool, n int) bool {
	_ = f
	_ = n
	wp := winAt(app.State.Mouse.Row)
	if wp == nil {
		wp = app.State.CurrentWindow
	}
	if wp == nil || wp.Buffer == nil {
		return false
	}

	windowScroll(wp, -wheelLines)
	return true
}

// CmdMouseWheelDown scrolls the viewport toward the end of the buffer.
func CmdMouseWheelDown(f bool, n int) bool {
	_ = f
	_ = n
	wp := winAt(app.State.Mouse.Row)
	if wp == nil {
		wp = app.State.CurrentWindow
	}
	if wp == nil || wp.Buffer == nil {
		return false
	}

	windowScroll(wp, wheelLines)
	return true
}

// applyWheelTicks scrolls the viewport by net wheel notches (positive = down).
func applyWheelTicks(net int) {
	if net == 0 {
		return
	}
	wp := winAt(app.State.Mouse.Row)
	if wp == nil {
		wp = app.State.CurrentWindow
	}
	if wp == nil || wp.Buffer == nil {
		return
	}
	windowScroll(wp, net*wheelLines)
}

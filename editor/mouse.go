package editor

// mouse.go - Mouse command implementations (translation of mouse.c)

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/view"
)

const wheelLines = 3

var mouseAnchorWindow *model.Window
var mouseAnchorLine uint
var mouseAnchorOffset uint

func windowCursorIsVisible(wp *model.Window) bool {
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

func windowLastVisibleLine(wp *model.Window) uint {
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

// windowScroll scrolls the window viewport by n lines (positive = down).
func windowScroll(wp *model.Window, n int) {
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

	if wp == model.State.CurrentWindow {
		if !windowCursorIsVisible(wp) {
			var loc buffer.Location
			if n > 0 {
				loc = buffer.Location{Line: windowLastVisibleLine(wp), Offset: 0}
			} else {
				loc = buffer.Location{Line: wp.TopLine, Offset: 0}
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
	wp := view.WinAt(model.State.Mouse.Row)
	if wp == nil {
		return false
	}

	if wp != model.State.CurrentWindow {
		model.WindowSelect(wp)
	}

	loc := view.MouseLocationInWindow(wp)
	model.State.CurrentWindow.SetCursor(loc)
	model.State.CurrentWindow.Mark.Line = 0
	model.State.CurrentWindow.Mark.Offset = 0
	mouseAnchorWindow = model.State.CurrentWindow
	mouseAnchorLine = loc.Line
	mouseAnchorOffset = loc.Offset
	model.State.CurrentWindow.ShouldRedraw = true
	model.State.CurrentWindow.ShouldUpdateModeLine = true
	return true
}

// CmdMouseDrag extends the selection by moving point to the current mouse position.
func CmdMouseDrag(f bool, n int) bool {
	_ = f
	_ = n
	wp := view.WinAt(model.State.Mouse.Row)
	if wp == nil || wp != model.State.CurrentWindow || mouseAnchorWindow != model.State.CurrentWindow {
		return false
	}

	loc := view.MouseLocationInWindow(wp)

	model.State.CurrentWindow.Mark.Line = mouseAnchorLine
	model.State.CurrentWindow.Mark.Offset = mouseAnchorOffset
	model.State.CurrentWindow.SetCursor(loc)
	model.State.CurrentWindow.ShouldRedraw = true
	model.State.CurrentWindow.ShouldUpdateModeLine = true
	return true
}

// CmdMouseWheelUp scrolls the viewport toward the beginning of the buffer.
func CmdMouseWheelUp(f bool, n int) bool {
	_ = f
	_ = n
	wp := view.WinAt(model.State.Mouse.Row)
	if wp == nil {
		wp = model.State.CurrentWindow
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
	wp := view.WinAt(model.State.Mouse.Row)
	if wp == nil {
		wp = model.State.CurrentWindow
	}
	if wp == nil || wp.Buffer == nil {
		return false
	}

	windowScroll(wp, wheelLines)
	return true
}

// ApplyWheelTicks scrolls the viewport by net wheel notches (positive = down).
func ApplyWheelTicks(net int) {
	if net == 0 {
		return
	}
	wp := view.WinAt(model.State.Mouse.Row)
	if wp == nil {
		wp = model.State.CurrentWindow
	}
	if wp == nil || wp.Buffer == nil {
		return
	}
	windowScroll(wp, net*wheelLines)
}

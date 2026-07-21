package runtime

// Mouse hit handling and click/drag commands.

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/window"
)

const wheelLines = 3

var mouseAnchorWindow *window.Window
var mouseAnchorLine uint
var mouseAnchorOffset uint

func windowCursorIsVisible(win *window.Window) bool {
	if win == nil {
		return false
	}
	visibleLine := win.TopLine
	for i := uint32(0); i < win.Height; i++ {
		if visibleLine == win.Cursor.Line {
			return true
		}
		if visibleLine > win.Buffer.LineCount {
			break
		}
		visibleLine++
	}
	return false
}

func windowLastVisibleLine(win *window.Window) uint {
	if win == nil {
		return 1
	}
	visibleLine := win.TopLine
	lastVisible := win.TopLine
	for i := uint32(0); i < win.Height; i++ {
		if visibleLine > win.Buffer.LineCount {
			break
		}
		lastVisible = visibleLine
		visibleLine++
	}
	return lastVisible
}

// windowScroll scrolls the window viewport by n lines (positive = down).
func windowScroll(win *window.Window, n int) {
	if win == nil || win.Buffer == nil {
		return
	}
	lineNumber := win.TopLine
	if n > 0 {
		for i := 0; i < n && lineNumber < win.Buffer.LineCount; i++ {
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
	if lineNumber > win.Buffer.LineCount {
		lineNumber = win.Buffer.LineCount
	}
	win.SetTopLine(lineNumber)
	win.ShouldRedraw = true

	if win == window.Active.CurrentWindow {
		if !windowCursorIsVisible(win) {
			var loc buffer.Location
			if n > 0 {
				loc = buffer.Location{Line: windowLastVisibleLine(win), Offset: 0}
			} else {
				loc = buffer.Location{Line: win.TopLine, Offset: 0}
			}
			win.SetCursor(loc)
		}
	}
}

// CmdMouseLeft moves point to the clicked position.
// If the click falls in a window other than the current one, that window becomes current.
func CmdMouseLeft(f bool, n int) bool {
	_ = f
	_ = n
	win := display.WinAt(display.Active.Mouse.Row)
	if win == nil {
		return false
	}

	if win != window.Active.CurrentWindow {
		window.WindowSelect(win)
	}

	loc := display.MouseLocationInWindow(win)
	window.Active.CurrentWindow.SetCursor(loc)
	window.Active.CurrentWindow.Mark.Line = 0
	window.Active.CurrentWindow.Mark.Offset = 0
	mouseAnchorWindow = window.Active.CurrentWindow
	mouseAnchorLine = loc.Line
	mouseAnchorOffset = loc.Offset
	window.Active.CurrentWindow.ShouldRedraw = true
	window.Active.CurrentWindow.ShouldUpdateModeLine = true
	return true
}

// CmdMouseDrag extends the selection by moving point to the current mouse position.
func CmdMouseDrag(f bool, n int) bool {
	_ = f
	_ = n
	win := display.WinAt(display.Active.Mouse.Row)
	if win == nil || win != window.Active.CurrentWindow || mouseAnchorWindow != window.Active.CurrentWindow {
		return false
	}

	loc := display.MouseLocationInWindow(win)

	window.Active.CurrentWindow.Mark.Line = mouseAnchorLine
	window.Active.CurrentWindow.Mark.Offset = mouseAnchorOffset
	window.Active.CurrentWindow.SetCursor(loc)
	window.Active.CurrentWindow.ShouldRedraw = true
	window.Active.CurrentWindow.ShouldUpdateModeLine = true
	return true
}

// CmdMouseWheelUp scrolls the viewport toward the beginning of the buffer.
func CmdMouseWheelUp(f bool, n int) bool {
	_ = f
	_ = n
	win := display.WinAt(display.Active.Mouse.Row)
	if win == nil {
		win = window.Active.CurrentWindow
	}
	if win == nil || win.Buffer == nil {
		return false
	}

	windowScroll(win, -wheelLines)
	return true
}

// CmdMouseWheelDown scrolls the viewport toward the end of the buffer.
func CmdMouseWheelDown(f bool, n int) bool {
	_ = f
	_ = n
	win := display.WinAt(display.Active.Mouse.Row)
	if win == nil {
		win = window.Active.CurrentWindow
	}
	if win == nil || win.Buffer == nil {
		return false
	}

	windowScroll(win, wheelLines)
	return true
}

// ApplyWheelTicks scrolls the viewport by net wheel notches (positive = down).
func ApplyWheelTicks(net int) {
	if net == 0 {
		return
	}
	win := display.WinAt(display.Active.Mouse.Row)
	if win == nil {
		win = window.Active.CurrentWindow
	}
	if win == nil || win.Buffer == nil {
		return
	}
	windowScroll(win, net*wheelLines)
}

package runtime

// Mouse hit handling and click/drag commands.

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/window"
)

const wheelLines = 3

var mouseAnchorWindow *window.Window
var mouseAnchorLine int
var mouseAnchorOffset int

func windowLastVisibleLine(win *window.Window) int {
	last := win.TopLine + win.Height - 1
	if last > len(win.Buffer.Lines) {
		last = len(win.Buffer.Lines)
	}
	if last < win.TopLine {
		return win.TopLine
	}
	return last
}

func windowCursorIsVisible(win *window.Window) bool {
	c := win.Cursor.Line
	return c >= win.TopLine && c <= windowLastVisibleLine(win)
}

// windowScroll scrolls the window viewport by n lines (positive = down).
func windowScroll(win *window.Window, n int) {
	lineNumber := win.TopLine + n
	if lineNumber < 1 {
		lineNumber = 1
	}
	if lineNumber > len(win.Buffer.Lines) {
		lineNumber = len(win.Buffer.Lines)
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

	if win != window.Active.CurrentWindow {
		window.WindowSelect(win)
	}

	cur := window.Active.CurrentWindow
	loc := display.MouseLocationInWindow(win)
	cur.SetCursor(loc)
	cur.Mark.Line = 0
	cur.Mark.Offset = 0
	mouseAnchorWindow = cur
	mouseAnchorLine = loc.Line
	mouseAnchorOffset = loc.Offset
	cur.ShouldRedraw = true
	cur.ShouldUpdateModeLine = true
	return true
}

// CmdMouseDrag extends the selection by moving point to the current mouse position.
func CmdMouseDrag(f bool, n int) bool {
	_ = f
	_ = n
	win := display.WinAt(display.Active.Mouse.Row)
	cur := window.Active.CurrentWindow
	if win == nil || win != cur || mouseAnchorWindow != cur {
		return false
	}

	loc := display.MouseLocationInWindow(win)

	cur.Mark.Line = mouseAnchorLine
	cur.Mark.Offset = mouseAnchorOffset
	cur.SetCursor(loc)
	cur.ShouldRedraw = true
	cur.ShouldUpdateModeLine = true
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
	windowScroll(win, net*wheelLines)
}

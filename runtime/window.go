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

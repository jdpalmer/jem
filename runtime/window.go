package runtime

// Window split/delete/next/only commands and layout retile.

import (
	"slices"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/window"
)

func CmdWindowDelete(f bool, n int) bool {
	_ = f
	_ = n
	wins := window.Active.Windows
	if len(wins) <= 1 {
		display.MBWrite("[cannot remove only window]")
		return false
	}

	cur := window.Active.CurrentWindow
	idx := slices.Index(wins, cur)
	if idx < 0 {
		return false
	}
	cur.SaveState()

	var next *window.Window
	if idx+1 < len(wins) {
		next = wins[idx+1]
	} else {
		next = wins[idx-1]
	}
	window.Active.Windows = append(wins[:idx], wins[idx+1:]...)
	window.WindowSelect(next)
	window.WindowRetile()
	return true
}

func CmdWindowNext(f bool, n int) bool {
	_ = f
	_ = n
	wins := window.Active.Windows
	if len(wins) <= 1 {
		return true
	}
	i := slices.Index(wins, window.Active.CurrentWindow)
	if i < 0 {
		i = 0
	}
	window.WindowSelect(wins[(i+1)%len(wins)])
	return true
}

func CmdWindowOnly(f bool, n int) bool {
	_ = f
	_ = n
	cur := window.Active.CurrentWindow
	for _, win := range window.Active.Windows {
		if win != cur {
			win.SaveState()
		}
	}
	window.Active.Windows = []*window.Window{cur}
	window.WindowSelect(cur)
	window.WindowRetile()
	return true
}

func CmdWindowSplit(f bool, n int) bool {
	_ = f
	_ = n
	if term.Rows() < 4*(len(window.Active.Windows)+1) {
		display.MBWrite("[window is too small to split]")
		return false
	}
	win := window.WindowCreate()
	if win == nil {
		display.MBWrite("[maximum number of windows has been reached]")
		return false
	}

	curr := window.Active.CurrentWindow
	win.Buffer = buffer.All.Current
	win.TopLine = curr.TopLine
	win.Cursor = curr.Cursor
	win.Mark = curr.Mark
	win.ScreenTopRow = curr.ScreenTopRow
	win.Height = curr.Height
	win.HScroll = curr.HScroll

	wins := window.Active.Windows
	wins = wins[:len(wins)-1] // WindowCreate appended win; reinsert beside curr
	i := slices.Index(wins, curr)
	if i < 0 {
		i = len(wins) - 1
	}
	window.Active.Windows = slices.Insert(wins, i+1, win)

	window.WindowRetile()
	return true
}

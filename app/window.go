package app

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/term"
)

func WindowSelect(wp *Window) {
	old := State.CurrentWindow
	if wp == nil {
		return
	}
	if old != nil && old != wp {
		old.ShouldUpdateModeLine = true
	}
	State.CurrentWindow = wp
	SetCurrentBuffer(wp.Buffer)
	wp.ShouldRedraw = true
	wp.ShouldUpdateModeLine = true
}

func WindowCreate() *Window {
	if len(State.WINDOWS) >= MaxWindows {
		return nil
	}
	wp := &Window{
		Buffer:               State.CurrentBuffer,
		TopLine:              1,
		Cursor:               buffer.Location{Line: 1, Offset: 0},
		Mark:                 buffer.Location{Line: 1, Offset: 0},
		ScreenTopRow:         0,
		Height:               0,
		ForceReframe:         false,
		ShouldReframe:        false,
		DidMove:              false,
		DidEdit:              false,
		ShouldRedraw:         true,
		ShouldUpdateModeLine: true,
		HScroll:              0,
	}
	State.WINDOWS = append(State.WINDOWS, wp)
	return wp
}

func (wp *Window) SaveState() {
	if wp != nil && wp.Buffer != nil {
		wp.Buffer.Cursor = wp.Cursor
		wp.Buffer.Mark = wp.Mark
	}
}

func BufferWindowCount(bp *buffer.Buffer) int {
	if bp == nil {
		return len(State.WINDOWS)
	}
	count := 0
	for _, wp := range State.WINDOWS {
		if wp != nil && wp.Buffer == bp {
			count++
		}
	}
	return count
}

func WindowRetile() {
	n := len(State.WINDOWS)
	if n == 0 {
		return
	}
	usable := term.Rows() - n
	if usable < 0 {
		usable = 0
	}
	baseRows := usable / n
	extraRows := usable % n
	top := 0

	for _, wp := range State.WINDOWS {
		if wp == nil {
			continue
		}
		rows := baseRows
		if extraRows > 0 {
			rows++
			extraRows--
		}
		wp.ScreenTopRow = uint32(top)
		wp.Height = uint32(rows)
		wp.ShouldRedraw = true
		wp.ShouldUpdateModeLine = true
		top += rows + 1
	}
}

func (wp *Window) CenterCursor() {
	if wp == nil {
		return
	}
	top := wp.Cursor.Line
	for i := wp.Height / 2; i > 0 && top > 1; i-- {
		top--
	}
	wp.SetTopLine(top)
}

func (wp *Window) SetTopLine(line uint) {
	if wp == nil {
		return
	}
	wp.TopLine = line
	wp.ShouldRedraw = true
}

func (wp *Window) GutterWidth() uint32 {
	if wp == nil || wp.Buffer == nil {
		return 3
	}
	digits := uint32(1)
	n := wp.Buffer.LineCount
	for n >= 10 {
		n /= 10
		digits++
	}
	width := digits + 2
	if width >= uint32(term.Cols()) {
		width = uint32(term.Cols()) - 1
	}
	if width < 3 {
		width = 3
	}
	return width
}

func (wp *Window) SetCursor(loc buffer.Location) {
	if wp == nil {
		return
	}
	wp.Cursor = loc
	wp.DidMove = true
	wp.ShouldRedraw = true
}

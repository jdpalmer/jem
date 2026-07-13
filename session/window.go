package session

import "github.com/jdpalmer/jem/term"

func WindowSelect(wp *Window) {
	old := App.CurrentWindow
	if wp == nil {
		return
	}
	if old != nil && old != wp {
		old.ShouldUpdateModeLine = true
	}
	App.CurrentWindow = wp
	SetCurrentBuffer(wp.Buffer)
	wp.ShouldRedraw = true
	wp.ShouldUpdateModeLine = true
}

func WindowCreate() *Window {
	if App.WindowCount >= MaxWindows {
		return nil
	}
	wp := &Window{
		Buffer:               App.CurrentBuffer,
		TopLine:              1,
		Cursor:               Location{Line: 1, Offset: 0},
		Mark:                 Location{Line: 1, Offset: 0},
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
	App.WINDOWS[App.WindowCount] = wp
	App.WindowCount++
	return wp
}

func WindowSaveState(wp *Window) {
	if wp != nil && wp.Buffer != nil {
		wp.Buffer.Cursor = wp.Cursor
		wp.Buffer.Mark = wp.Mark
	}
}

func BufferWindowCount(bp *Buffer) int {
	if bp == nil {
		return int(App.WindowCount)
	}
	count := 0
	for i := 0; i < int(App.WindowCount); i++ {
		wp := App.WINDOWS[i]
		if wp != nil && wp.Buffer == bp {
			count++
		}
	}
	return count
}

func WindowRetile() {
	if App.WindowCount == 0 {
		return
	}
	usable := term.Rows() - int(App.WindowCount)
	if usable < 0 {
		usable = 0
	}
	baseRows := usable / int(App.WindowCount)
	extraRows := usable % int(App.WindowCount)
	top := 0

	for i := 0; i < int(App.WindowCount); i++ {
		wp := App.WINDOWS[i]
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

func WindowCenterCursor(wp *Window) {
	if wp == nil {
		return
	}
	top := wp.Cursor.Line
	for i := wp.Height / 2; i > 0 && top > 1; i-- {
		top--
	}
	WindowSetTopLine(wp, top)
}

func WindowSetTopLine(wp *Window, line uint) {
	if wp == nil {
		return
	}
	wp.TopLine = line
	wp.ShouldRedraw = true
}

func WindowGutterWidth(wp *Window) uint32 {
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

func WindowSetCursor(wp *Window, loc Location) {
	if wp == nil {
		return
	}
	wp.Cursor = loc
	wp.DidMove = true
	wp.ShouldRedraw = true
}

package window

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/term"
)

func WindowSelect(win *Window) {
	old := Active.CurrentWindow
	if win == nil {
		return
	}
	if old != nil && old != win {
		old.ShouldUpdateModeLine = true
	}
	Active.CurrentWindow = win
	buffer.SetCurrent(win.Buffer)
	win.ShouldRedraw = true
	win.ShouldUpdateModeLine = true
}

func WindowCreate() *Window {
	if len(Active.Windows) >= MaxWindows {
		return nil
	}
	win := &Window{
		Buffer:               buffer.All.Current,
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
	Active.Windows = append(Active.Windows, win)
	return win
}

func (win *Window) SaveState() {
	if win != nil && win.Buffer != nil {
		win.Buffer.Cursor = win.Cursor
		win.Buffer.Mark = win.Mark
	}
}

func WindowRetile() {
	n := len(Active.Windows)
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

	for _, win := range Active.Windows {
		if win == nil {
			continue
		}
		rows := baseRows
		if extraRows > 0 {
			rows++
			extraRows--
		}
		win.ScreenTopRow = uint32(top)
		win.Height = uint32(rows)
		win.ShouldRedraw = true
		win.ShouldUpdateModeLine = true
		top += rows + 1
	}
}

func (win *Window) CenterCursor() {
	if win == nil {
		return
	}
	top := win.Cursor.Line
	for i := win.Height / 2; i > 0 && top > 1; i-- {
		top--
	}
	win.SetTopLine(top)
}

func (win *Window) SetTopLine(line uint) {
	if win == nil {
		return
	}
	win.TopLine = line
	win.ShouldRedraw = true
}

func (win *Window) GutterWidth() uint32 {
	if win == nil || win.Buffer == nil {
		return 3
	}
	digits := uint32(1)
	n := win.Buffer.LineCount
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

func (win *Window) SetCursor(loc buffer.Location) {
	if win == nil {
		return
	}
	win.Cursor = loc
	win.DidMove = true
	win.ShouldRedraw = true
}

// AdjustLocationsAfterReplace updates cursor, mark, and TopLine for every
// window showing buf after a replacement of [begin, end) ending at newEnd.
func AdjustLocationsAfterReplace(buf *buffer.Buffer, begin, end, newEnd buffer.Location) {
	AdjustWindowLocations(Active.Windows, buf, begin, end, newEnd)
}

// AdjustWindowLocations is the pure form of AdjustLocationsAfterReplace.
func AdjustWindowLocations(windows []*Window, buf *buffer.Buffer, begin, end, newEnd buffer.Location) {
	for i := 0; i < len(windows); i++ {
		win := windows[i]
		if win == nil || win.Buffer != buf {
			continue
		}
		win.Cursor.AdjustAfterReplace(begin, end, newEnd)
		win.Mark.AdjustAfterReplace(begin, end, newEnd)
		if win.TopLine >= begin.Line {
			if win.TopLine > end.Line {
				if newEnd.Line >= end.Line {
					win.TopLine += newEnd.Line - end.Line
				} else {
					removed := end.Line - newEnd.Line
					if win.TopLine >= removed {
						win.TopLine -= removed
					} else {
						win.TopLine = 1
					}
				}
			} else {
				win.TopLine = begin.Line
			}
		}
	}
}

// NoteBufferEdit marks windows showing buf for redraw / modeline after an edit.
func NoteBufferEdit(buf *buffer.Buffer, isStructural bool) {
	NoteBufferEditOnWindows(Active.Windows, buf, isStructural)
}

// NoteBufferEditOnWindows is the pure form of NoteBufferEdit.
func NoteBufferEditOnWindows(windows []*Window, buf *buffer.Buffer, isStructural bool) {
	if buf == nil {
		return
	}
	firstChange := !buf.IsChanged
	shouldRedraw := isStructural
	count := 0
	for i := 0; i < len(windows); i++ {
		win := windows[i]
		if win != nil && win.Buffer == buf {
			count++
		}
	}
	if count != 1 {
		shouldRedraw = true
	}
	for i := 0; i < len(windows); i++ {
		win := windows[i]
		if win == nil || win.Buffer != buf {
			continue
		}
		if shouldRedraw {
			win.ShouldRedraw = true
		} else {
			win.DidEdit = true
		}
		if firstChange {
			win.ShouldUpdateModeLine = true
		}
	}
}

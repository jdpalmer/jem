package window

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/term"
)

func WindowSelect(wp *Window) {
	old := Active.CurrentWindow
	if wp == nil {
		return
	}
	if old != nil && old != wp {
		old.ShouldUpdateModeLine = true
	}
	Active.CurrentWindow = wp
	buffer.SetCurrent(wp.Buffer)
	wp.ShouldRedraw = true
	wp.ShouldUpdateModeLine = true
}

func WindowCreate() *Window {
	if len(Active.Windows) >= MaxWindows {
		return nil
	}
	wp := &Window{
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
	Active.Windows = append(Active.Windows, wp)
	return wp
}

func (wp *Window) SaveState() {
	if wp != nil && wp.Buffer != nil {
		wp.Buffer.Cursor = wp.Cursor
		wp.Buffer.Mark = wp.Mark
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

	for _, wp := range Active.Windows {
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

// AdjustLocationsAfterReplace updates cursor, mark, and TopLine for every
// window showing bp after a replacement of [begin, end) ending at newEnd.
func AdjustLocationsAfterReplace(bp *buffer.Buffer, begin, end, newEnd buffer.Location) {
	AdjustWindowLocations(Active.Windows, bp, begin, end, newEnd)
}

// AdjustWindowLocations is the pure form of AdjustLocationsAfterReplace.
func AdjustWindowLocations(windows []*Window, bp *buffer.Buffer, begin, end, newEnd buffer.Location) {
	for i := 0; i < len(windows); i++ {
		wp := windows[i]
		if wp == nil || wp.Buffer != bp {
			continue
		}
		wp.Cursor.AdjustAfterReplace(begin, end, newEnd)
		wp.Mark.AdjustAfterReplace(begin, end, newEnd)
		if wp.TopLine >= begin.Line {
			if wp.TopLine > end.Line {
				if newEnd.Line >= end.Line {
					wp.TopLine += newEnd.Line - end.Line
				} else {
					removed := end.Line - newEnd.Line
					if wp.TopLine >= removed {
						wp.TopLine -= removed
					} else {
						wp.TopLine = 1
					}
				}
			} else {
				wp.TopLine = begin.Line
			}
		}
	}
}

// NoteBufferEdit marks windows showing bp for redraw / modeline after an edit.
func NoteBufferEdit(bp *buffer.Buffer, isStructural bool) {
	NoteBufferEditOnWindows(Active.Windows, bp, isStructural)
}

// NoteBufferEditOnWindows is the pure form of NoteBufferEdit.
func NoteBufferEditOnWindows(windows []*Window, bp *buffer.Buffer, isStructural bool) {
	if bp == nil {
		return
	}
	firstChange := !bp.IsChanged
	shouldRedraw := isStructural
	count := 0
	for i := 0; i < len(windows); i++ {
		wp := windows[i]
		if wp != nil && wp.Buffer == bp {
			count++
		}
	}
	if count != 1 {
		shouldRedraw = true
	}
	for i := 0; i < len(windows); i++ {
		wp := windows[i]
		if wp == nil || wp.Buffer != bp {
			continue
		}
		if shouldRedraw {
			wp.ShouldRedraw = true
		} else {
			wp.DidEdit = true
		}
		if firstChange {
			wp.ShouldUpdateModeLine = true
		}
	}
}

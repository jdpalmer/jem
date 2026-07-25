package window

import (
	"strconv"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/term"
)

// WindowSelect sets the active window and updates its buffer, mode line, and redraw state.
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

// WindowCreate creates and appends a new window to the active windows list, returning nil if at capacity.
func WindowCreate() *Window {
	if len(Active.Windows) >= MaxWindows {
		return nil
	}
	win := &Window{
		Buffer:               buffer.All.Current,
		TopLine:              1,
		Cursor:               buffer.Location{Line: 1, Offset: 0},
		Mark:                 buffer.Location{Line: 1, Offset: 0},
		ShouldRedraw:         true,
		ShouldUpdateModeLine: true,
	}
	Active.Windows = append(Active.Windows, win)
	return win
}

// ContentRowOffset returns how many blank rows to leave at the top of the
// viewport when BottomAlign is set and the buffer does not fill the window.
func (win *Window) ContentRowOffset() int {
	if win == nil || !win.BottomAlign || win.Buffer == nil || win.Height <= 0 {
		return 0
	}
	visible := len(win.Buffer.Lines) - win.TopLine + 1
	if visible < 0 {
		visible = 0
	}
	if visible >= win.Height {
		return 0
	}
	return win.Height - visible
}

// SaveState persists the window's cursor and mark positions into the buffer.
func (win *Window) SaveState() {
	if win != nil && win.Buffer != nil {
		win.Buffer.Cursor = win.Cursor
		win.Buffer.Mark = win.Mark
	}
}

// WindowRetile redistributes terminal rows among all active windows and marks them for redraw.
func WindowRetile() {
	n := len(Active.Windows)
	if n == 0 {
		return
	}
	modeLines := 0
	for _, win := range Active.Windows {
		if win != nil && !win.NoModeLine {
			modeLines++
		}
	}
	usable := term.Rows() - modeLines
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
		win.ScreenTopRow = top
		win.Height = rows
		win.ShouldRedraw = true
		win.ShouldUpdateModeLine = true
		top += rows
		if !win.NoModeLine {
			top++
		}
	}
}

// CenterCursor scrolls the window so the cursor appears in the middle of the visible area.
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

// SetTopLine sets the first visible line of the window and marks it for redraw.
func (win *Window) SetTopLine(line int) {
	if win == nil {
		return
	}
	win.TopLine = line
	win.ShouldRedraw = true
}

// GutterWidth returns the width of the line-number gutter, based on the total number of buffer lines.
func (win *Window) GutterWidth() int {
	if win == nil || win.Buffer == nil {
		return 3
	}
	digits := len(strconv.Itoa(len(win.Buffer.Lines)))
	width := digits + 2
	if width >= term.Cols() {
		width = term.Cols() - 1
	}
	if width < 3 {
		width = 3
	}
	return width
}

// SetCursor updates the window cursor position and marks the window for redraw and as moved.
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

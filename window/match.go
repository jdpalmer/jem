package window

import (
	"github.com/jdpalmer/jem/buffer"
)

const matchBufferName = "*match*"

// MatchWindow returns the window showing the *match* buffer, if any.
func MatchWindow() *Window {
	mbp := buffer.Find(matchBufferName)
	if mbp == nil {
		return nil
	}
	for i := 0; i < len(Active.Windows); i++ {
		win := Active.Windows[i]
		if win != nil && win.Buffer == mbp {
			return win
		}
	}
	return nil
}

// ShowMatchWindow ensures a window displays the *match* buffer.
func ShowMatchWindow() {
	mbp := buffer.Find(matchBufferName)
	if mbp == nil {
		return
	}
	if MatchWindow() != nil {
		return
	}
	win := WindowCreate()
	if win == nil {
		return
	}
	win.Buffer = mbp
	win.TopLine = 1
	win.Cursor = buffer.Location{Line: 1, Offset: 0}
	win.Mark = buffer.Location{Line: 0, Offset: 0}
	win.ShouldRedraw = true
	win.ShouldUpdateModeLine = true
	WindowRetile()
}

// HideMatchWindow removes the *match* window if more than one window exists.
func HideMatchWindow() {
	mw := MatchWindow()
	if mw == nil || len(Active.Windows) <= 1 {
		return
	}
	idx := -1
	for i := 0; i < len(Active.Windows); i++ {
		if Active.Windows[i] == mw {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	if Active.CurrentWindow == mw {
		newCur := Active.Windows[0]
		if idx == 0 && len(Active.Windows) > 1 {
			newCur = Active.Windows[1]
		}
		WindowSelect(newCur)
	}
	for i := idx; i < len(Active.Windows)-1; i++ {
		Active.Windows[i] = Active.Windows[i+1]
	}
	Active.Windows[len(Active.Windows)-1] = nil
	Active.Windows = Active.Windows[:len(Active.Windows)-1]
	WindowRetile()
}

// ScrollMatchToSelection scrolls the *match* window so selected (0-based) is visible.
func ScrollMatchToSelection(selected int) {
	mw := MatchWindow()
	if mw == nil || selected == 0 {
		return
	}
	line := int(selected + 1)
	if line < mw.TopLine {
		mw.TopLine = line
		mw.ShouldRedraw = true
		return
	}
	if mw.Height == 0 {
		return
	}
	lastVisible := mw.TopLine + mw.Height - 1
	if line > lastVisible {
		mw.TopLine = line - mw.Height + 1
		if mw.TopLine < 1 {
			mw.TopLine = 1
		}
		mw.ShouldRedraw = true
	}
}

func ensureMatchBuffer() *buffer.Buffer {
	mbp := buffer.Find(matchBufferName)
	if mbp != nil {
		return mbp
	}
	mbp = buffer.Create()
	if mbp == nil {
		return nil
	}
	mbp.Name = matchBufferName
	mbp.LangMode = buffer.LModeNone
	return mbp
}

// SetMatchBufferText replaces *match* buffer contents and shows/scrolls its window.
// If text is empty, hides the match window instead.
func SetMatchBufferText(text []byte, selected int) {
	if len(text) == 0 {
		if buffer.Find(matchBufferName) != nil {
			HideMatchWindow()
		}
		return
	}

	mbp := ensureMatchBuffer()
	if mbp == nil {
		return
	}

	prevRO := mbp.IsReadonly
	mbp.IsReadonly = false
	eof := buffer.MakeLocation(mbp.EOF(), 0)
	_ = mbp.SetText(buffer.MakeLocation(1, 0), eof, text, nil)
	mbp.IsReadonly = prevRO
	mbp.IsReadonly = true

	ShowMatchWindow()
	ScrollMatchToSelection(selected)
}

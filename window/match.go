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
	win.BottomAlign = true
	win.NoModeLine = true
	win.ShouldRedraw = true
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

// DiscardMatchBuffer hides the *match* window and releases the *match* buffer.
func DiscardMatchBuffer() {
	HideMatchWindow()
	if mbp := buffer.Find(matchBufferName); mbp != nil {
		ReleaseBuffer(mbp)
	}
}

// ScrollMatchToSelection scrolls the *match* window so selected (0-based) is visible.
// Also moves the window cursor onto the selected line so DisplayUpdate reframing
// does not yank TopLine back to line 1.
func ScrollMatchToSelection(selected int) {
	mw := MatchWindow()
	if mw == nil {
		return
	}
	if selected < 0 {
		selected = 0
	}
	line := selected + 1
	if mw.Buffer != nil {
		n := len(mw.Buffer.Lines)
		if n > 0 && line > n {
			line = n
		}
	}
	mw.Cursor = buffer.Location{Line: line, Offset: 0}

	if line < mw.TopLine {
		mw.TopLine = line
	} else if mw.Height > 0 {
		lastVisible := mw.TopLine + mw.Height - 1
		if line > lastVisible {
			mw.TopLine = line - mw.Height + 1
			if mw.TopLine < 1 {
				mw.TopLine = 1
			}
		}
	}
	mw.ShouldRedraw = true
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
		DiscardMatchBuffer()
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
	if mw := MatchWindow(); mw != nil {
		// buffer.SetText does not mark windows dirty; force a full redraw so the
		// "> " selection marker and picker highlight colors update.
		mw.ShouldRedraw = true
	}
	ScrollMatchToSelection(selected)
}

package model

import (
	"github.com/jdpalmer/jem/buffer"
)

const matchBufferName = "*match*"

// MatchWindow returns the window showing the *match* buffer, if any.
func MatchWindow() *Window {
	mbp := BufferFind(matchBufferName)
	if mbp == nil {
		return nil
	}
	for i := 0; i < int(len(State.Windows)); i++ {
		wp := State.Windows[i]
		if wp != nil && wp.Buffer == mbp {
			return wp
		}
	}
	return nil
}

// ShowMatchWindow ensures a window displays the *match* buffer.
func ShowMatchWindow() {
	mbp := BufferFind(matchBufferName)
	if mbp == nil {
		return
	}
	if MatchWindow() != nil {
		return
	}
	wp := WindowCreate()
	if wp == nil {
		return
	}
	wp.Buffer = mbp
	wp.TopLine = 1
	wp.Cursor = buffer.Location{Line: 1, Offset: 0}
	wp.Mark = buffer.Location{Line: 0, Offset: 0}
	wp.ShouldRedraw = true
	wp.ShouldUpdateModeLine = true
	WindowRetile()
}

// HideMatchWindow removes the *match* window if more than one window exists.
func HideMatchWindow() {
	mw := MatchWindow()
	if mw == nil || len(State.Windows) <= 1 {
		return
	}
	idx := -1
	for i := 0; i < int(len(State.Windows)); i++ {
		if State.Windows[i] == mw {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	if State.CurrentWindow == mw {
		newCur := State.Windows[0]
		if idx == 0 && len(State.Windows) > 1 {
			newCur = State.Windows[1]
		}
		WindowSelect(newCur)
	}
	for i := idx; i < len(State.Windows)-1; i++ {
		State.Windows[i] = State.Windows[i+1]
	}
	State.Windows[len(State.Windows)-1] = nil
	State.Windows = State.Windows[:len(State.Windows)-1]
	WindowRetile()
}

// ScrollMatchToSelection scrolls the *match* window so selected (0-based) is visible.
func ScrollMatchToSelection(selected uint) {
	mw := MatchWindow()
	if mw == nil || selected == 0 {
		return
	}
	line := uint(selected + 1)
	if line < mw.TopLine {
		mw.TopLine = line
		mw.ShouldRedraw = true
		return
	}
	if mw.Height == 0 {
		return
	}
	lastVisible := mw.TopLine + uint(mw.Height) - 1
	if line > lastVisible {
		mw.TopLine = line - uint(mw.Height) + 1
		if mw.TopLine < 1 {
			mw.TopLine = 1
		}
		mw.ShouldRedraw = true
	}
}

// ensureMatchBuffer returns the *match* buffer, creating it if needed.
func ensureMatchBuffer() *buffer.Buffer {
	mbp := BufferFind(matchBufferName)
	if mbp != nil {
		return mbp
	}
	mbp = BufferCreate(&State.EditorRuntimeState)
	if mbp == nil {
		return nil
	}
	mbp.Name = matchBufferName
	mbp.LangMode = buffer.LModeNone
	return mbp
}

// SetMatchBufferText replaces *match* buffer contents and shows/scrolls its window.
// If text is empty, hides the match window instead.
func SetMatchBufferText(text []byte, selected uint) {
	if len(text) == 0 {
		if BufferFind(matchBufferName) != nil {
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
	_ = SetText(mbp, buffer.MakeLocation(1, 0), eof, text, nil)
	mbp.IsReadonly = prevRO
	mbp.IsReadonly = true

	ShowMatchWindow()
	ScrollMatchToSelection(selected)
}

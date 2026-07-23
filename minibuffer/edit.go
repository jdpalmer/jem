package minibuffer

// Package minibuffer holds prompt edit state (display only paints).

import (
	"bytes"
	"unicode"
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/killring"
	"github.com/jdpalmer/jem/term"
)

var minibufferHistory []string

const minibufferHistorySlots = 16

// MinibufferHistoryAdd appends text to the global minibuffer history, dropping
// duplicates of the most-recent entry and trimming to the ring size.
func MinibufferHistoryAdd(text string) {
	if text == "" {
		return
	}
	if len(minibufferHistory) > 0 && minibufferHistory[len(minibufferHistory)-1] == text {
		return
	}
	minibufferHistory = append(minibufferHistory, text)
	if len(minibufferHistory) > minibufferHistorySlots {
		minibufferHistory = minibufferHistory[len(minibufferHistory)-minibufferHistorySlots:]
	}
}



func isMinibufWordRune(r rune) bool {
	if r >= 0x80 {
		return true
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) ||
		r == '_' || r == '-' || r == '.' || r == '~'
}

// SetText replaces the entire contents of state.Text with text, placing the
// cursor at the end. text is silently truncated to Nbuf-1 bytes if necessary.
func (state *MinibufferState) SetText(text []byte) {
	if state == nil {
		return
	}
	n := len(text)
	if n >= state.Nbuf {
		n = state.Nbuf - 1
		if n < 0 {
			n = 0
		}
	}
	state.Text = state.Text[:0]
	state.Text = append(state.Text, text[:n]...)
	state.CursorPos = len(state.Text)
}

// InsertChar inserts rune r at the current cursor position.
func (state *MinibufferState) InsertChar(r rune) bool {
	if state == nil {
		return false
	}
	var enc [utf8.UTFMax]byte
	n := utf8.EncodeRune(enc[:], r)
	if len(state.Text)+n >= state.Nbuf {
		return false
	}
	cpos := state.CursorPos
	oldLen := len(state.Text)
	if cap(state.Text) >= oldLen+n {
		state.Text = state.Text[:oldLen+n]
		copy(state.Text[cpos+n:], state.Text[cpos:oldLen])
		copy(state.Text[cpos:], enc[:n])
	} else {
		out := make([]byte, 0, oldLen+n)
		out = append(out, state.Text[:cpos]...)
		out = append(out, enc[:n]...)
		out = append(out, state.Text[cpos:]...)
		state.Text = out
	}
	state.CursorPos += n
	return true
}

// DeleteBackward deletes the rune immediately before the cursor.
func (state *MinibufferState) DeleteBackward() bool {
	if state == nil || state.CursorPos == 0 {
		return false
	}
	cpos := state.CursorPos
	prev := buffer.PrevOffset(state.Text, cpos)
	n := cpos - prev
	copy(state.Text[prev:], state.Text[cpos:])
	state.Text = state.Text[:len(state.Text)-n]
	state.CursorPos = prev
	return true
}

// DeleteForward deletes the rune at the cursor position.
func (state *MinibufferState) DeleteForward() bool {
	if state == nil || state.CursorPos >= len(state.Text) {
		return false
	}
	cpos := state.CursorPos
	next := buffer.NextOffset(state.Text, cpos)
	n := next - cpos
	copy(state.Text[cpos:], state.Text[next:])
	state.Text = state.Text[:len(state.Text)-n]
	return true
}

// ClearText erases all text and resets the cursor to position 0.
func (state *MinibufferState) ClearText() bool {
	if state == nil || len(state.Text) == 0 {
		return false
	}
	state.Text = state.Text[:0]
	state.CursorPos = 0
	return true
}

// KillRange removes bytes [start, end) from state.Text and adds them to the
// kill ring. The cursor is left at start.
func (state *MinibufferState) KillRange(start, end int) bool {
	if state == nil || start == end {
		return false
	}
	killed := make([]byte, end-start)
	copy(killed, state.Text[start:end])
	killring.KillBegin()
	if !killring.KillAppend(killed) {
		return false
	}
	killring.KillWriteClipboard()
	copy(state.Text[start:], state.Text[end:])
	state.Text = state.Text[:len(state.Text)-(end-start)]
	state.CursorPos = start
	return true
}

// BackwardChar moves the cursor one rune to the left.
func (state *MinibufferState) BackwardChar() bool {
	if state == nil || state.CursorPos == 0 {
		return false
	}
	state.CursorPos = buffer.PrevOffset(state.Text, state.CursorPos)
	return true
}

// ForwardChar moves the cursor one rune to the right.
func (state *MinibufferState) ForwardChar() bool {
	if state == nil || state.CursorPos >= len(state.Text) {
		return false
	}
	state.CursorPos = buffer.NextOffset(state.Text, state.CursorPos)
	return true
}

// GotoBol moves the cursor to the start of the text.
func (state *MinibufferState) GotoBol() bool {
	if state == nil || state.CursorPos == 0 {
		return false
	}
	state.CursorPos = 0
	return true
}

// GotoEol moves the cursor to the end of the text.
func (state *MinibufferState) GotoEol() bool {
	if state == nil {
		return false
	}
	end := len(state.Text)
	if state.CursorPos == end {
		return false
	}
	state.CursorPos = end
	return true
}

// BackwardWord moves the cursor backward over one word.
func (state *MinibufferState) BackwardWord() bool {
	if state == nil {
		return false
	}
	pos := state.CursorPos
	for pos > 0 {
		prev := buffer.PrevOffset(state.Text, pos)
		r, _ := utf8.DecodeRune(state.Text[prev:])
		if isMinibufWordRune(r) {
			break
		}
		pos = prev
	}
	for pos > 0 {
		prev := buffer.PrevOffset(state.Text, pos)
		r, _ := utf8.DecodeRune(state.Text[prev:])
		if !isMinibufWordRune(r) {
			break
		}
		pos = prev
	}
	if pos == state.CursorPos {
		return false
	}
	state.CursorPos = pos
	return true
}

// ForwardWord moves the cursor forward over one word.
func (state *MinibufferState) ForwardWord() bool {
	if state == nil {
		return false
	}
	pos := state.CursorPos
	textLen := len(state.Text)
	for pos < textLen {
		r, sz := utf8.DecodeRune(state.Text[pos:])
		if isMinibufWordRune(r) {
			break
		}
		pos += sz
	}
	for pos < textLen {
		r, sz := utf8.DecodeRune(state.Text[pos:])
		if !isMinibufWordRune(r) {
			break
		}
		pos += sz
	}
	if pos == state.CursorPos {
		return false
	}
	state.CursorPos = pos
	return true
}

// DeleteWordBackward kills from the start of the previous word to the cursor.
func (state *MinibufferState) DeleteWordBackward() bool {
	if state == nil {
		return false
	}
	oldPos := state.CursorPos
	if !state.BackwardWord() {
		return false
	}
	return state.KillRange(state.CursorPos, oldPos)
}

// DeleteWordForward kills from the cursor to the end of the next word.
func (state *MinibufferState) DeleteWordForward() bool {
	if state == nil {
		return false
	}
	startPos := state.CursorPos
	if !state.ForwardWord() {
		return false
	}
	endPos := state.CursorPos
	state.CursorPos = startPos
	return state.KillRange(startPos, endPos)
}

// Kill kills from the cursor to the end of the text (C-k).
func (state *MinibufferState) Kill() bool {
	if state == nil {
		return false
	}
	end := len(state.Text)
	cpos := state.CursorPos
	if cpos >= end {
		return false
	}
	return state.KillRange(cpos, end)
}

// Yank inserts the kill-ring contents at the cursor (C-y).
// Rejects pastes that contain newlines.
func (state *MinibufferState) Yank() bool {
	if state == nil {
		return false
	}
	killring.KillReadClipboard()
	k := killring.KillBytes()
	klen := len(k)
	if klen == 0 {
		return false
	}
	if bytes.ContainsAny(k, "\n\r") {
		return false
	}
	if len(state.Text)+klen >= state.Nbuf {
		return false
	}
	cpos := state.CursorPos
	oldLen := len(state.Text)
	state.Text = state.Text[:oldLen+klen]
	copy(state.Text[cpos+klen:], state.Text[cpos:oldLen])
	copy(state.Text[cpos:], k)
	state.CursorPos += klen
	return true
}

// StepHistory navigates the global history ring.
// dir < 0 moves backward (older), dir > 0 moves forward (newer).
func (state *MinibufferState) StepHistory(dir int) bool {
	if state == nil {
		return false
	}
	histLen := len(minibufferHistory)
	if histLen == 0 {
		return false
	}
	if dir < 0 { // backward — older entry
		if state.HistoryPos+1 >= histLen {
			return false
		}
		if state.HistoryPos < 0 {
			state.SavedEdit = make([]byte, len(state.Text))
			copy(state.SavedEdit, state.Text)
			state.HaveSavedEdit = true
		}
		state.HistoryPos++
		idx := histLen - 1 - state.HistoryPos
		state.SetText([]byte(minibufferHistory[idx]))
		return true
	}
	// forward — newer / back to current edit
	if state.HistoryPos < 0 {
		return false
	}
	if state.HistoryPos == 0 {
		if state.HaveSavedEdit {
			state.SetText(state.SavedEdit)
		} else {
			state.Text = state.Text[:0]
			state.CursorPos = 0
		}
		state.HistoryPos = -1
		state.HaveSavedEdit = false
		return true
	}
	state.HistoryPos--
	idx := histLen - 1 - state.HistoryPos
	state.SetText([]byte(minibufferHistory[idx]))
	return true
}

// EditKeyHistory applies one editing keystroke to a raw prompt buffer with C-p/C-n history.
func EditKeyHistory(buf []byte, cpos *int, nbuf int, historyPos *int, haveSavedEdit *bool, savedEdit []byte, k uint32) MinibufferEditResult {
	if cpos == nil || historyPos == nil || haveSavedEdit == nil || nbuf <= 0 {
		return MinibufEditUnhandled
	}
	end := 0
	for end < nbuf && buf[end] != 0 {
		end++
	}
	state := MinibufferState{
		Text:          append(make([]byte, 0, nbuf), buf[:end]...),
		Nbuf:          nbuf,
		CursorPos:     *cpos,
		HistoryPos:    *historyPos,
		HaveSavedEdit: *haveSavedEdit,
	}
	if *haveSavedEdit {
		state.SavedEdit = append([]byte(nil), savedEdit...)
	}
	if k == term.KeyEnter || k == '\r' || k == '\n' || k == (term.CTL|'M') || k == (term.CTL|'J') {
		return MinibufEditUnhandled
	}
	killring.Tick()
	switch k {
	case term.CTL | 'P', term.KeyUp:
		if !state.StepHistory(-1) {
			return MinibufEditNoChange
		}
	case term.CTL | 'N', term.KeyDown:
		if !state.StepHistory(1) {
			return MinibufEditNoChange
		}
	case 0x7F, term.CTL | 'H':
		if !state.DeleteBackward() {
			return MinibufEditNoChange
		}
	case term.CTL | 'U':
		if !state.ClearText() {
			return MinibufEditNoChange
		}
	default:
		if (k&term.KeyMask) != 0 || k < 0x20 {
			return MinibufEditUnhandled
		}
		if !state.InsertChar(rune(k)) {
			return MinibufEditNoChange
		}
	}
	copy(buf, state.Text)
	if len(state.Text) < nbuf {
		buf[len(state.Text)] = 0
	}
	*cpos = state.CursorPos
	*historyPos = state.HistoryPos
	*haveSavedEdit = state.HaveSavedEdit
	if state.HaveSavedEdit {
		copy(savedEdit, state.SavedEdit)
	}
	return MinibufEditChanged
}

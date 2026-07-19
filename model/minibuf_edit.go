package model

// Minibuffer text editing and history (fat model; view only presents).

import (
	"unicode"
	"unicode/utf8"

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

func prevUtf8Offset(buf []byte, pos int) int {
	if pos <= 0 {
		return 0
	}
	start := pos - 1
	for start >= 0 && start > pos-4 {
		if (buf[start] & 0xC0) != 0x80 {
			return start
		}
		start--
	}
	if start < 0 {
		return 0
	}
	return start
}

func nextUtf8Offset(buf []byte, pos int) int {
	if pos >= len(buf) || pos < 0 {
		return pos
	}
	_, size := utf8.DecodeRune(buf[pos:])
	if size <= 0 {
		return pos + 1
	}
	return pos + size
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
	if uint(n) >= state.Nbuf {
		n = int(state.Nbuf) - 1
		if n < 0 {
			n = 0
		}
	}
	state.Text = state.Text[:0]
	state.Text = append(state.Text, text[:n]...)
	state.CursorPos = uint(len(state.Text))
}

// InsertChar inserts rune r at the current cursor position.
func (state *MinibufferState) InsertChar(r rune) bool {
	if state == nil {
		return false
	}
	var enc [utf8.UTFMax]byte
	n := utf8.EncodeRune(enc[:], r)
	if uint(len(state.Text)+n) >= state.Nbuf {
		return false
	}
	cpos := int(state.CursorPos)
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
	state.CursorPos += uint(n)
	return true
}

// DeleteBackward deletes the rune immediately before the cursor.
func (state *MinibufferState) DeleteBackward() bool {
	if state == nil || state.CursorPos == 0 {
		return false
	}
	cpos := int(state.CursorPos)
	prev := prevUtf8Offset(state.Text, cpos)
	n := cpos - prev
	copy(state.Text[prev:], state.Text[cpos:])
	state.Text = state.Text[:len(state.Text)-n]
	state.CursorPos = uint(prev)
	return true
}

// DeleteForward deletes the rune at the cursor position.
func (state *MinibufferState) DeleteForward() bool {
	if state == nil || int(state.CursorPos) >= len(state.Text) {
		return false
	}
	cpos := int(state.CursorPos)
	next := nextUtf8Offset(state.Text, cpos)
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
	KillBegin()
	if !KillAppend(killed) {
		return false
	}
	KillWriteClipboard()
	copy(state.Text[start:], state.Text[end:])
	state.Text = state.Text[:len(state.Text)-(end-start)]
	state.CursorPos = uint(start)
	return true
}

// BackwardChar moves the cursor one rune to the left.
func (state *MinibufferState) BackwardChar() bool {
	if state == nil || state.CursorPos == 0 {
		return false
	}
	state.CursorPos = uint(prevUtf8Offset(state.Text, int(state.CursorPos)))
	return true
}

// ForwardChar moves the cursor one rune to the right.
func (state *MinibufferState) ForwardChar() bool {
	if state == nil || int(state.CursorPos) >= len(state.Text) {
		return false
	}
	state.CursorPos = uint(nextUtf8Offset(state.Text, int(state.CursorPos)))
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
	end := uint(len(state.Text))
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
	pos := int(state.CursorPos)
	for pos > 0 {
		prev := prevUtf8Offset(state.Text, pos)
		r, _ := utf8.DecodeRune(state.Text[prev:])
		if isMinibufWordRune(r) {
			break
		}
		pos = prev
	}
	for pos > 0 {
		prev := prevUtf8Offset(state.Text, pos)
		r, _ := utf8.DecodeRune(state.Text[prev:])
		if !isMinibufWordRune(r) {
			break
		}
		pos = prev
	}
	if pos == int(state.CursorPos) {
		return false
	}
	state.CursorPos = uint(pos)
	return true
}

// ForwardWord moves the cursor forward over one word.
func (state *MinibufferState) ForwardWord() bool {
	if state == nil {
		return false
	}
	pos := int(state.CursorPos)
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
	if pos == int(state.CursorPos) {
		return false
	}
	state.CursorPos = uint(pos)
	return true
}

// DeleteWordBackward kills from the start of the previous word to the cursor.
func (state *MinibufferState) DeleteWordBackward() bool {
	if state == nil {
		return false
	}
	oldPos := int(state.CursorPos)
	if !state.BackwardWord() {
		return false
	}
	return state.KillRange(int(state.CursorPos), oldPos)
}

// DeleteWordForward kills from the cursor to the end of the next word.
func (state *MinibufferState) DeleteWordForward() bool {
	if state == nil {
		return false
	}
	startPos := int(state.CursorPos)
	if !state.ForwardWord() {
		return false
	}
	endPos := int(state.CursorPos)
	state.CursorPos = uint(startPos)
	return state.KillRange(startPos, endPos)
}

// Kill kills from the cursor to the end of the text (C-k).
func (state *MinibufferState) Kill() bool {
	if state == nil {
		return false
	}
	end := len(state.Text)
	cpos := int(state.CursorPos)
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
	KillReadClipboard()
	k := KillBytes()
	klen := uint(len(k))
	if klen == 0 {
		return false
	}
	for _, b := range k {
		if b == '\n' || b == '\r' {
			return false
		}
	}
	if uint(len(state.Text))+klen >= state.Nbuf {
		return false
	}
	cpos := int(state.CursorPos)
	oldLen := len(state.Text)
	state.Text = state.Text[:oldLen+int(klen)]
	copy(state.Text[cpos+int(klen):], state.Text[cpos:oldLen])
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
		if int(state.HistoryPos)+1 >= histLen {
			return false
		}
		if state.HistoryPos < 0 {
			state.SavedEdit = make([]byte, len(state.Text))
			copy(state.SavedEdit, state.Text)
			state.HaveSavedEdit = true
		}
		state.HistoryPos++
		idx := histLen - 1 - int(state.HistoryPos)
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
	idx := histLen - 1 - int(state.HistoryPos)
	state.SetText([]byte(minibufferHistory[idx]))
	return true
}

func tickKillState() {
	if State.KillState != CmdStateNone {
		State.KillState--
	}
}

// EditKey applies one editing keystroke to a raw prompt buffer (used by isearch).
func EditKey(buf []byte, cpos *int, nbuf int, k uint32) MinibufferEditResult {
	if cpos == nil || nbuf <= 0 {
		return MinibufEditUnhandled
	}
	end := 0
	for end < nbuf && buf[end] != 0 {
		end++
	}
	state := MinibufferState{
		Text:       append(make([]byte, 0, nbuf), buf[:end]...),
		Nbuf:       uint(nbuf),
		CursorPos:  uint(*cpos),
		HistoryPos: -1,
	}
	if k == term.KeyEnter || k == '\r' || k == '\n' || k == (term.CTL|'M') || k == (term.CTL|'J') {
		return MinibufEditUnhandled
	}
	tickKillState()
	switch k {
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
	*cpos = int(state.CursorPos)
	return MinibufEditChanged
}

// EditKeyHistory is like EditKey but supports C-p/C-n history navigation.
func EditKeyHistory(buf []byte, cpos *int, nbuf int, initial []byte, historyPos *int16, haveSavedEdit *bool, savedEdit []byte, k uint32) MinibufferEditResult {
	if cpos == nil || historyPos == nil || haveSavedEdit == nil || nbuf <= 0 {
		return MinibufEditUnhandled
	}
	end := 0
	for end < nbuf && buf[end] != 0 {
		end++
	}
	state := MinibufferState{
		Text:          append(make([]byte, 0, nbuf), buf[:end]...),
		Nbuf:          uint(nbuf),
		CursorPos:     uint(*cpos),
		HistoryPos:    *historyPos,
		HaveSavedEdit: *haveSavedEdit,
	}
	if *haveSavedEdit {
		state.SavedEdit = append([]byte(nil), savedEdit...)
	}
	_ = initial
	if k == term.KeyEnter || k == '\r' || k == '\n' || k == (term.CTL|'M') || k == (term.CTL|'J') {
		return MinibufEditUnhandled
	}
	tickKillState()
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
	*cpos = int(state.CursorPos)
	*historyPos = state.HistoryPos
	*haveSavedEdit = state.HaveSavedEdit
	if state.HaveSavedEdit {
		copy(savedEdit, state.SavedEdit)
	}
	return MinibufEditChanged
}

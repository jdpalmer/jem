package ui

// minibuf.go - Minibuffer input prompts and feedback (Go port of src/minibuffer.c)

import (
	"fmt"
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/fileio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jdpalmer/jem/term"
)

// ---- History ------------------------------------------------------------------

// Simple circular history (keeps recent entries).
var minibufferHistory []string

const minibufferHistorySlots = 16

// mbHistoryAdd appends text to the global minibuffer history, dropping
// duplicates of the most-recent entry and trimming to the ring size.
func mbHistoryAdd(text string) {
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

// ---- Message line rendering ---------------------------------------------------

// mlBegin starts rendering on the message line (resets gutter clip like C ml_begin).
func mlBegin(style TextStyle) {
	clipLeftCol = 0
	screenMove(term.Rows(), 0)
	screenSetStyle(style)
}

// mlFinish ends message-line rendering: erase trailing cells, flush, set cursor.
func mlFinish(cursorCol int, messagePresent bool) {
	screenEraseEol()
	screenFlushRow(term.Rows(), cursorCol)
	app.State.MessagePresent = messagePresent
}

func mbWrite(format string, args ...interface{}) {
	var msg string
	if len(args) == 0 {
		msg = format
	} else {
		msg = fmt.Sprintf(format, args...)
	}
	mlBegin(app.State.Theme.NormalStyle)
	screenPutBytes([]byte(msg))
	mlFinish(0, len(msg) > 0)
}

func mbClear() {
	mlBegin(app.State.Theme.NormalStyle)
	mlFinish(0, false)
}

// displayWidthBytes returns the display column width of the first endOff bytes
// of text (treats each rune as width 1 — sufficient for minibuffer prompts).
func displayWidthBytes(text []byte, endOff int) int {
	if endOff > len(text) {
		endOff = len(text)
	}
	count := 0
	for o := 0; o < endOff; {
		r, size := utf8.DecodeRune(text[o:endOff])
		if r == utf8.RuneError && size == 1 {
			o++
			count++
			continue
		}
		o += size
		count++
	}
	return count
}

// mbWritePromptStyle renders prompt+text on the message line with the cursor
// placed at the column corresponding to cpos (byte offset into text).
func mbWritePromptStyle(prompt string, text []byte, cpos int, style TextStyle) {
	mlBegin(style)
	screenPutBytes([]byte(prompt))
	cursorCol := displayWidthBytes([]byte(prompt), len(prompt)) + displayWidthBytes(text, cpos)
	screenPutBytes(text)
	if cursorCol < 0 {
		cursorCol = 0
	}
	mlFinish(cursorCol, true)
}

func mbWritePrompt(prompt string, text []byte, cpos int) {
	mbWritePromptStyle(prompt, text, cpos, app.State.Theme.NormalStyle)
}

// ---- UTF-8 byte-offset helpers -----------------------------------------------

// prevUtf8Offset returns the byte offset of the rune that starts just before
// pos in buf (walks back up to 4 bytes to find the rune boundary).
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

// nextUtf8Offset returns the byte offset of the rune that starts just after
// pos in buf, or pos if already at the end.
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

// ---- MinibufferState text-editing primitives ----------------------------------
//
// All editing functions operate on MinibufferState.Text (the current content
// as a variable-length []byte) and MinibufferState.CursorPos (byte offset of
// the cursor within Text).  Text is always kept within Nbuf bytes.

// isMinibufWordRune returns true if r is treated as a word character for
// minibuffer word-navigation (mirrors minibuffer_is_word_codepoint in C).
func isMinibufWordRune(r rune) bool {
	if r >= 0x80 {
		return true
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) ||
		r == '_' || r == '-' || r == '.' || r == '~'
}

// mbSetText replaces the entire contents of state.Text with text, placing the
// cursor at the end.  text is silently truncated to Nbuf-1 bytes if necessary.
func mbSetText(state *MinibufferState, text []byte) {
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

// mbInsertChar inserts rune r at the current cursor position.
func mbInsertChar(state *MinibufferState, r rune) bool {
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

// mbDeleteBackward deletes the rune immediately before the cursor.
func mbDeleteBackward(state *MinibufferState) bool {
	if state.CursorPos == 0 {
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

// mbDeleteForward deletes the rune at the cursor position.
func mbDeleteForward(state *MinibufferState) bool {
	if int(state.CursorPos) >= len(state.Text) {
		return false
	}
	cpos := int(state.CursorPos)
	next := nextUtf8Offset(state.Text, cpos)
	n := next - cpos
	copy(state.Text[cpos:], state.Text[next:])
	state.Text = state.Text[:len(state.Text)-n]
	return true
}

// mbClearText erases all text and resets the cursor to position 0.
func mbClearText(state *MinibufferState) bool {
	if len(state.Text) == 0 {
		return false
	}
	state.Text = state.Text[:0]
	state.CursorPos = 0
	return true
}

// mbKillRange removes bytes [start, end) from state.Text and adds them to the
// kill ring.  The cursor is left at start.
func mbKillRange(state *MinibufferState, start, end int) bool {
	if start == end {
		return false
	}
	killed := make([]byte, end-start)
	copy(killed, state.Text[start:end])
	if PackageHooks.KillBegin != nil {
		PackageHooks.KillBegin()
	}
	if PackageHooks.KillAppend == nil || !PackageHooks.KillAppend(killed) {
		return false
	}
	if PackageHooks.KillWriteClipboard != nil {
		PackageHooks.KillWriteClipboard()
	}
	copy(state.Text[start:], state.Text[end:])
	state.Text = state.Text[:len(state.Text)-(end-start)]
	state.CursorPos = uint(start)
	return true
}

// mbBackwardChar moves the cursor one rune to the left.
func mbBackwardChar(state *MinibufferState) bool {
	if state.CursorPos == 0 {
		return false
	}
	state.CursorPos = uint(prevUtf8Offset(state.Text, int(state.CursorPos)))
	return true
}

// mbForwardChar moves the cursor one rune to the right.
func mbForwardChar(state *MinibufferState) bool {
	if int(state.CursorPos) >= len(state.Text) {
		return false
	}
	state.CursorPos = uint(nextUtf8Offset(state.Text, int(state.CursorPos)))
	return true
}

// mbGotoBol moves the cursor to the start of the text.
func mbGotoBol(state *MinibufferState) bool {
	if state.CursorPos == 0 {
		return false
	}
	state.CursorPos = 0
	return true
}

// mbGotoEol moves the cursor to the end of the text.
func mbGotoEol(state *MinibufferState) bool {
	end := uint(len(state.Text))
	if state.CursorPos == end {
		return false
	}
	state.CursorPos = end
	return true
}

// mbBackwardWord moves the cursor backward over one word
// (skips non-word chars, then skips word chars).
func mbBackwardWord(state *MinibufferState) bool {
	pos := int(state.CursorPos)
	// skip non-word chars
	for pos > 0 {
		prev := prevUtf8Offset(state.Text, pos)
		r, _ := utf8.DecodeRune(state.Text[prev:])
		if isMinibufWordRune(r) {
			break
		}
		pos = prev
	}
	// skip word chars
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

// mbForwardWord moves the cursor forward over one word
// (skips non-word chars, then skips word chars).
func mbForwardWord(state *MinibufferState) bool {
	pos := int(state.CursorPos)
	textLen := len(state.Text)
	// skip non-word chars
	for pos < textLen {
		r, sz := utf8.DecodeRune(state.Text[pos:])
		if isMinibufWordRune(r) {
			break
		}
		pos += sz
	}
	// skip word chars
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

// mbDeleteWordBackward kills from the start of the previous word to the cursor.
func mbDeleteWordBackward(state *MinibufferState) bool {
	oldPos := int(state.CursorPos)
	if !mbBackwardWord(state) {
		return false
	}
	return mbKillRange(state, int(state.CursorPos), oldPos)
}

// mbDeleteWordForward kills from the cursor to the end of the next word.
func mbDeleteWordForward(state *MinibufferState) bool {
	startPos := int(state.CursorPos)
	if !mbForwardWord(state) {
		return false
	}
	endPos := int(state.CursorPos)
	state.CursorPos = uint(startPos)
	return mbKillRange(state, startPos, endPos)
}

// mbKill kills from the cursor to the end of the text (C-k).
func mbKill(state *MinibufferState) bool {
	end := len(state.Text)
	cpos := int(state.CursorPos)
	if cpos >= end {
		return false
	}
	return mbKillRange(state, cpos, end)
}

// mbYank inserts the kill-ring contents at the cursor (C-y).
// Rejects pastes that contain newlines.
func mbYank(state *MinibufferState) bool {
	if PackageHooks.KillReadClipboard != nil {
		PackageHooks.KillReadClipboard()
	}
	if PackageHooks.KillBytes == nil {
		return false
	}
	k := PackageHooks.KillBytes()
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

// editorMinibufferPaste inserts bracketed-paste text into the active minibuffer.
func editorMinibufferPaste(text []byte) bool {
	state := app.State.ActiveMinibuffer
	if state == nil {
		return false
	}
	if state.Text == nil || state.Nbuf+uint(len(text)) >= uint(cap(state.Text)) {
		return false
	}

	for i := 0; i < len(text); i++ {
		if text[i] == '\r' {
			text[i] = '\n'
		}
	}

	insertLen := uint(len(text))
	if state.Nbuf+insertLen >= uint(cap(state.Text)) {
		insertLen = uint(cap(state.Text)) - state.Nbuf
	}
	copy(state.Text[state.CursorPos:], text[:insertLen])
	state.Nbuf += insertLen
	state.CursorPos += insertLen
	state.HaveSavedEdit = false

	return true
}

// mbStepHistory navigates the global history ring.
// dir < 0 moves backward (older), dir > 0 moves forward (newer).
func mbStepHistory(state *MinibufferState, dir int) bool {
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
		mbSetText(state, []byte(minibufferHistory[idx]))
		return true
	}
	// forward — newer / back to current edit
	if state.HistoryPos < 0 {
		return false
	}
	if state.HistoryPos == 0 {
		if state.HaveSavedEdit {
			mbSetText(state, state.SavedEdit)
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
	mbSetText(state, []byte(minibufferHistory[idx]))
	return true
}

// mbEditKey applies one editing keystroke to a raw prompt buffer (used by isearch).
func mbEditKey(buf []byte, cpos *int, nbuf int, k uint32) MinibufferEditResult {
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
	if k == KeyEnter || k == '\r' || k == '\n' || k == (CTL|'M') || k == (CTL|'J') {
		return MinibufEditUnhandled
	}
	if app.State.KillState != CmdStateNone {
		app.State.KillState--
	}
	switch k {
	case 0x7F, CTL | 'H':
		if !mbDeleteBackward(&state) {
			return MinibufEditNoChange
		}
	case CTL | 'U':
		if !mbClearText(&state) {
			return MinibufEditNoChange
		}
	default:
		if (k&KeyMask) != 0 || k < 0x20 {
			return MinibufEditUnhandled
		}
		if !mbInsertChar(&state, rune(k)) {
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

// mbEditKeyHistory is like mbEditKey but supports C-p/C-n history navigation.
func mbEditKeyHistory(buf []byte, cpos *int, nbuf int, initial []byte, historyPos *int16, haveSavedEdit *bool, savedEdit []byte, k uint32) MinibufferEditResult {
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
	if k == KeyEnter || k == '\r' || k == '\n' || k == (CTL|'M') || k == (CTL|'J') {
		return MinibufEditUnhandled
	}
	if app.State.KillState != CmdStateNone {
		app.State.KillState--
	}
	switch k {
	case CTL | 'P', KeyUp:
		if !mbStepHistory(&state, -1) {
			return MinibufEditNoChange
		}
	case CTL | 'N', KeyDown:
		if !mbStepHistory(&state, 1) {
			return MinibufEditNoChange
		}
	case 0x7F, CTL | 'H':
		if !mbDeleteBackward(&state) {
			return MinibufEditNoChange
		}
	case CTL | 'U':
		if !mbClearText(&state) {
			return MinibufEditNoChange
		}
	default:
		if (k&KeyMask) != 0 || k < 0x20 {
			return MinibufEditUnhandled
		}
		if !mbInsertChar(&state, rune(k)) {
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

// ---- Text prompts ------------------------------------------------------------

// mbReadInitial prompts the user and returns their input in buf.  If initial
// is non-nil it is used as the starting content.  Full Emacs-style editing
// (C-a/e/b/f/d, M-b/f/d, C-k, C-y, C-p/n history, …) is supported.
func mbReadInitial(prompt string, buf []byte, capacity int, initial []byte) PromptResult {
	if PackageHooks.MacroPlayPrompt != nil {
		if pr, played := PackageHooks.MacroPlayPrompt(buf); played {
			return pr
		}
	}

	state := MinibufferState{
		Prompt:     prompt,
		Text:       make([]byte, 0, capacity),
		Nbuf:       uint(capacity),
		HistoryPos: -1,
	}
	if len(initial) > 0 {
		mbSetText(&state, initial)
	}

	app.State.ActiveMinibuffer = &state
	defer func() { app.State.ActiveMinibuffer = nil }()
	drainGlobalMinibufKeys()
	drainGlobalKeyCh()

	mbWritePrompt(prompt, state.Text, int(state.CursorPos))
	DisplayUpdate()

	for {
		k, ok := <-GlobalMinibufKeyCh
		if !ok {
			return PromptResultAbort
		}
		if isPasteRedrawKey(k) {
			DisplayUpdate()
			mbWritePrompt(prompt, state.Text, int(state.CursorPos))
			continue
		}

		switch {
		case k == KeyEnter || k == '\r' || k == '\n' || k == (CTL|'M') || k == (CTL|'J'):
			mbHistoryAdd(string(state.Text))
			n := copy(buf, state.Text)
			if n < len(buf) {
				buf[n] = 0
			}
			if PackageHooks.MacroRecordMinibufferResult != nil {
				PackageHooks.MacroRecordMinibufferResult(state.Text)
			}
			mbClear()
			return PromptResultYes

		case k == (CTL|'G') || k == 0x07 || k == 0x1B:
			mbWrite("^G")
			mbClear()
			return PromptResultAbort

		// History navigation
		case k == (CTL|'P') || k == KeyUp:
			if !mbStepHistory(&state, -1) {
				term.Beep()
			}
		case k == (CTL|'N') || k == KeyDown:
			if !mbStepHistory(&state, 1) {
				term.Beep()
			}

		// Cursor movement
		case k == (CTL|'A') || k == KeyHome:
			if !mbGotoBol(&state) {
				term.Beep()
			}
		case k == (CTL|'E') || k == KeyEnd:
			if !mbGotoEol(&state) {
				term.Beep()
			}
		case k == (CTL|'B') || k == KeyLeft:
			if !mbBackwardChar(&state) {
				term.Beep()
			}
		case k == (CTL|'F') || k == KeyRight:
			if !mbForwardChar(&state) {
				term.Beep()
			}
		case k == (META|'B') || k == (SHIFT|KeyLeft):
			if !mbBackwardWord(&state) {
				term.Beep()
			}
		case k == (META|'F') || k == (SHIFT|KeyRight):
			if !mbForwardWord(&state) {
				term.Beep()
			}

		// Editing
		case k == 0x7F || k == (CTL|'H'):
			if !mbDeleteBackward(&state) {
				term.Beep()
			}
		case k == (CTL|'D') || k == KeyDelete:
			if !mbDeleteForward(&state) {
				term.Beep()
			}
		case k == (CTL | 'U'):
			if !mbClearText(&state) {
				term.Beep()
			}
		case k == (CTL | 'K'):
			if !mbKill(&state) {
				term.Beep()
			}
		case k == (CTL | 'Y'):
			if !mbYank(&state) {
				term.Beep()
			}
		case k == (META | 'D'):
			if !mbDeleteWordForward(&state) {
				term.Beep()
			}
		case k == (META|'H') || k == (META|0x7F):
			if !mbDeleteWordBackward(&state) {
				term.Beep()
			}

		default:
			if k&0x20000000 != 0 {
				continue // ignore mouse events while typing at the prompt
			}
			if k < UnicodeLimit && k >= 0x20 && (k&KeyMask) == 0 {
				if !mbInsertChar(&state, rune(k)) {
					term.Beep()
				}
			} else {
				term.Beep()
			}
		}

		mbWritePrompt(prompt, state.Text, int(state.CursorPos))
		DisplayUpdate()
	}
}

// mbRead is a convenience wrapper for mbReadInitial with no initial text.
func mbRead(prompt string, buf []byte) PromptResult {
	return mbReadInitial(prompt, buf, len(buf), nil)
}

// ---- Horizontal choice menu (mb_choose) --------------------------------------

const (
	mlChoiceLeftWidth      = 2 // "… "
	mlChoiceRightWidth     = 3 // " …"  (space + ellipsis)
	mlChoiceSeparatorWidth = 2 // "  "
)

// mlChoiceVisibleWidth returns the total column width if choices [start..end]
// are displayed (including overflow indicators but not the leading prompt).
func mlChoiceVisibleWidth(ctx any, labelFn MLChoiceLabelFn, count, start, end int) int {
	w := 0
	if start > 0 {
		w += mlChoiceLeftWidth
	}
	for i := start; i <= end; i++ {
		if i > start {
			w += mlChoiceSeparatorWidth
		}
		w += len(labelFn(ctx, uint8(i)))
	}
	if end < count-1 {
		w += mlChoiceRightWidth
	}
	return w
}

// mlChoiceWindow computes the widest visible window of choices around selected
// that fits within avail columns, alternating right/left expansion.
func mlChoiceWindow(ctx any, labelFn MLChoiceLabelFn, count, selected, avail int) (start, end int) {
	start = selected
	end = selected
	chooseRight := true
	for {
		expanded := false
		r := end + 1
		l := start - 1
		if chooseRight && r < count {
			if mlChoiceVisibleWidth(ctx, labelFn, count, start, r) <= avail {
				end = r
				expanded = true
			}
		}
		if l >= 0 {
			if mlChoiceVisibleWidth(ctx, labelFn, count, l, end) <= avail {
				start = l
				expanded = true
			}
		}
		if !chooseRight && r < count {
			if mlChoiceVisibleWidth(ctx, labelFn, count, start, r) <= avail {
				end = r
				expanded = true
			}
		}
		if !expanded {
			break
		}
		chooseRight = !chooseRight
	}
	return
}

// drainGlobalMinibufKeys discards any keys routed to the minibuffer channel
// before a minibuffer prompt begins.
func drainGlobalMinibufKeys() {
	for {
		select {
		case <-GlobalMinibufKeyCh:
		default:
			return
		}
	}
}

// drainGlobalKeyCh discards keys that reached the main channel during the
// brief window before ActiveMinibuffer is set (prevents stale dispatch).
func drainGlobalKeyCh() {
	for {
		select {
		case <-GlobalKeyCh:
		default:
			return
		}
	}
}

// mlChoiceRender renders the visible choice window on the message line and
// positions the cursor on the selected item.
func mlChoiceRender(prompt string, ctx any, labelFn MLChoiceLabelFn, count, start, end, selected int) {
	normalStyle := app.State.Theme.NormalStyle
	selStyle := app.State.Theme.PickerSelectionStyle
	maxcol := term.Cols() - 1
	col := 0
	selectedCol := 0

	mlBegin(normalStyle)

	if prompt != "" {
		pb := []byte(prompt)
		screenPutBytes(pb)
		col += displayWidthBytes(pb, len(pb))
	}

	if start > 0 && maxcol-col >= mlChoiceLeftWidth {
		screenPutBytes([]byte("\xe2\x80\xa6 ")) // "… "
		col += mlChoiceLeftWidth
	}

	for i := start; i <= end; i++ {
		label := labelFn(ctx, uint8(i))
		if i > start && maxcol-col >= mlChoiceSeparatorWidth {
			screenPutBytes([]byte("  "))
			col += mlChoiceSeparatorWidth
		}
		if i == selected {
			selectedCol = col
			screenSetStyle(selStyle)
		}
		screenPutBytes(label)
		col += displayWidthBytes(label, len(label))
		if i == selected {
			screenSetStyle(normalStyle)
		}
	}

	if end < count-1 && maxcol-col >= mlChoiceRightWidth {
		screenPutBytes([]byte("  \xe2\x80\xa6")) // "  …"
	}

	mlFinish(selectedCol, true)
}

// mbChoose presents a horizontal menu of count choices at the message line.
// Returns the selected index (≥0), -1 on Escape/cancel, or -2 on Ctrl-G abort.
func mbChoose(prompt string, ctx any, labelFn MLChoiceLabelFn, count uint8, defaultIdx uint8) int16 {
	n := int(count)
	if n <= 0 {
		return -1
	}
	selected := int(defaultIdx)
	if selected >= n {
		selected = 0
	}
	promptWidth := displayWidthBytes([]byte(prompt), len(prompt))
	avail := term.Cols() - 1 - promptWidth
	if avail < 1 {
		avail = 1
	}

	app.State.ActiveMinibuffer = &MinibufferState{}
	defer func() { app.State.ActiveMinibuffer = nil }()
	drainGlobalMinibufKeys()
	drainGlobalKeyCh()

	for {
		start, end := mlChoiceWindow(ctx, labelFn, n, selected, avail)
		mlChoiceRender(prompt, ctx, labelFn, n, start, end, selected)

		k, ok := <-GlobalMinibufKeyCh
		if !ok {
			mbClear()
			return -1
		}
		if isPasteRedrawKey(k) {
			continue
		}
		switch {
		case k == 0x0D || k == 0x0A || k == KeyEnter || k == (CTL|'M') || k == (CTL|'J'):
			mbClear()
			return int16(selected)
		case k == 0x07 || k == (CTL|'G'):
			mbClear()
			return -2
		case k == 0x1B:
			mbClear()
			return -1
		case k == KeyLeft || k == (CTL|'B') || k == KeyUp:
			if selected > 0 {
				selected--
			}
		case k == KeyRight || k == (CTL|'F') || k == KeyDown:
			if selected < n-1 {
				selected++
			}
		default:
			if k&0x20000000 != 0 {
				// Ignore mouse events while choosing.
				continue
			}
		}
	}
}

// mbYesNo prompts the user for a yes/no answer using the horizontal choice menu.
func mbYesNo(prompt string) PromptResult {
	choices := [][]byte{[]byte("yes"), []byte("no")}
	labelFn := func(ctx any, idx uint8) []byte {
		sl := ctx.([][]byte)
		if int(idx) < len(sl) {
			return sl[int(idx)]
		}
		return nil
	}
	question := prompt
	if len(prompt) > 0 && prompt[len(prompt)-1] != ' ' {
		question = prompt + " "
	}
	choice := mbChoose(question, choices, labelFn, 2, 0)
	switch choice {
	case 0:
		return PromptResultYes
	case 1:
		return PromptResultNo
	default:
		return PromptResultAbort
	}
}

// ---- Filename prompt with tab completion and fuzzy matching ------------------

// shouldSkipFuzzyFile returns true for binary/derived files that clutter the
// fuzzy picker (mirrors should_skip_fuzzy_file in C).
func shouldSkipFuzzyFile(name string) bool {
	return strings.HasSuffix(name, ".o") ||
		strings.HasSuffix(name, ".exe") ||
		strings.HasSuffix(name, ".pyc")
}

// collectFuzzyPaths lists the immediate children of dirpath for use in the
// fuzzy file picker.  Symlinks, hidden files/dirs, and binary artefacts are
// skipped.  Directories are returned with a trailing separator.
func collectFuzzyPaths(dirpath, prefix string) []string {
	openDir := fileio.OpenDirFromPrompt(dirpath)
	absDir, err := filepath.Abs(openDir)
	if err != nil {
		absDir = filepath.Clean(openDir)
	} else {
		absDir = filepath.Clean(absDir)
	}

	var paths []string
	if filepath.Dir(absDir) != absDir {
		if prefix == "" {
			paths = append(paths, "../")
		} else {
			paths = append(paths, filepath.Join(prefix, "..")+string(filepath.Separator))
		}
	}

	entries, err := os.ReadDir(openDir)
	if err != nil {
		return paths
	}
	sep := string(filepath.Separator)
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." || strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		rel := name
		if prefix != "" {
			rel = filepath.Join(prefix, name)
		}
		if e.IsDir() {
			if name == ".git" || name == "__pycache__" || name == "node_modules" {
				continue
			}
			paths = append(paths, rel+sep)
		} else if e.Type().IsRegular() {
			if shouldSkipFuzzyFile(name) {
				continue
			}
			paths = append(paths, rel)
		}
	}
	return paths
}

// lowerByte is a fast ASCII tolower helper.
func lowerByte(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c - 'A' + 'a'
	}
	return c
}

// filenameFuzzyScore scores name against query using the same algorithm as the
// C fuzzy_score function.  Returns a large negative number when no match exists.
func filenameFuzzyScore(name, query string) int {
	score := 0
	prev := -1
	nameLen := len(name)
	for qi := 0; qi < len(query); qi++ {
		qc := lowerByte(query[qi])
		pos := prev + 1
		for pos < nameLen && lowerByte(name[pos]) != qc {
			pos++
		}
		if pos >= nameLen {
			return -1000000
		}
		score += 10
		if pos == 0 || name[pos-1] == '/' || name[pos-1] == '_' ||
			name[pos-1] == '-' || name[pos-1] == '.' {
			score += 12
		}
		if prev >= 0 {
			if pos == prev+1 {
				score += 15
			} else {
				score -= pos - prev - 1
			}
		} else {
			score -= pos
		}
		prev = pos
	}
	score -= nameLen / 4
	return score
}

// filenameFuzzyMatches returns the indices (into paths) of up to maxMatches
// entries that best match query, ordered by score descending.
func filenameFuzzyMatches(paths []string, query string, maxMatches int) []uint {
	if len(paths) == 0 || maxMatches <= 0 {
		return nil
	}
	type entry struct {
		idx   int
		score int
	}
	var matches []entry
	for i, p := range paths {
		sc := filenameFuzzyScore(p, query)
		if sc <= -1000000 {
			continue
		}
		matches = append(matches, entry{idx: i, score: sc})
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(a, b int) bool {
		if matches[a].score != matches[b].score {
			return matches[a].score > matches[b].score
		}
		return paths[matches[a].idx] < paths[matches[b].idx]
	})
	n := len(matches)
	if n > maxMatches {
		n = maxMatches
	}
	out := make([]uint, n)
	for i := 0; i < n; i++ {
		out[i] = uint(matches[i].idx)
	}
	return out
}

// completePromptFilename performs tab-completion on the current text in state:
// it opens the directory implied by the typed path, finds all entries with the
// matching prefix, replaces the typed portion with the longest common prefix,
// and appends "/" when exactly one match is a directory.
// Returns true if the text was changed.
func completePromptFilename(state *MinibufferState) bool {
	typed := string(state.Text)
	expanded := fileio.ExpandPath(typed)

	if typed == "~" {
		mbSetText(state, []byte("~/"))
		return true
	}

	tdir, tprefix := fileio.PromptSplit(typed)
	_ = tprefix
	edir, eprefix := fileio.PromptSplit(expanded)
	openDir := fileio.OpenDirFromPrompt(edir)

	entries, err := os.ReadDir(openDir)
	if err != nil {
		return false
	}

	prefixLen := len(eprefix)
	common := ""
	matchCount := 0
	matchIsDir := false

	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		if prefixLen == 0 && strings.HasPrefix(name, ".") {
			continue
		}
		if !strings.HasPrefix(name, eprefix) {
			continue
		}
		isDir := e.IsDir()
		if matchCount == 0 {
			common = name
			matchIsDir = isDir
		} else {
			i := 0
			for i < len(common) && i < len(name) && common[i] == name[i] {
				i++
			}
			common = common[:i]
			matchIsDir = false
		}
		matchCount++
	}

	if matchCount == 0 {
		return false
	}

	newText := filepath.Join(tdir, common)
	if matchCount == 1 && matchIsDir {
		newText += string(filepath.Separator)
	}
	if uint(len(newText)) >= state.Nbuf {
		return false
	}
	mbSetText(state, []byte(newText))
	return true
}

// promptFormatWithCount formats a prompt string, inserting a "[sel+1/count]: "
// counter when the prompt ends with ": ".  It mirrors prompt_format_with_count
// in C and is used by the filename and command-palette prompts.
func promptFormatWithCount(prompt string, sel, count int) string {
	if count <= 0 {
		return prompt
	}
	plen := len(prompt)
	if plen >= 2 && prompt[plen-2] == ':' && prompt[plen-1] == ' ' {
		return fmt.Sprintf("%s [%d/%d]: ", prompt[:plen-2], sel+1, count)
	}
	return prompt
}

// mbReadFilename prompts for a filename with tab completion and fuzzy matching.
// The caller passes a pre-allocated buf of size nbuf; on success the chosen
// path is written as a NUL-terminated string into buf.
func mbReadFilename(prompt string, buf []byte, nbuf int) PromptResult {
	state := MinibufferState{
		Prompt:     prompt,
		Text:       make([]byte, 0, nbuf),
		Nbuf:       uint(nbuf),
		HistoryPos: -1,
	}
	// Pre-fill with existing buf content (initial text).
	if len(buf) > 0 && buf[0] != 0 {
		end := 0
		for end < len(buf) && buf[end] != 0 {
			end++
		}
		mbSetText(&state, buf[:end])
	}

	var filePaths []string
	var matchRoot string
	var currentDirPart string
	var matchIndices []uint
	var lastQuery string
	sel := 0
	programmatic := false

	// refreshList reloads filePaths only when the root directory changes.
	refreshList := func(dir string) {
		if dir == matchRoot {
			return
		}
		fp := collectFuzzyPaths(dir, "")
		if len(fp) > 0 && fp[0] == "../" {
			sort.Strings(fp[1:])
		} else {
			sort.Strings(fp)
		}
		filePaths = fp
		matchRoot = dir
	}

	parseDirPart := func(query string) (dirPart, pattern string) {
		return fileio.PromptSplit(query)
	}

	expandedDir := func(dirPart string) string {
		return fileio.OpenDirFromPrompt(dirPart)
	}

	// syncMatches recomputes matchIndices from the current query text.
	syncMatches := func() {
		query := string(state.Text)
		queryChanged := !programmatic && query != lastQuery
		lastQuery = query

		// "~" alone: don't trigger directory listing yet.
		if query == "~" {
			currentDirPart = ""
			matchIndices = nil
			if queryChanged {
				sel = 0
			}
			return
		}
		dirPart, pattern := parseDirPart(query)
		currentDirPart = dirPart
		refreshList(expandedDir(dirPart))

		const maxMatches = 16
		if pattern == "" {
			n := len(filePaths)
			if n > maxMatches {
				n = maxMatches
			}
			matchIndices = make([]uint, n)
			for i := range matchIndices {
				matchIndices[i] = uint(i)
			}
		} else {
			matchIndices = filenameFuzzyMatches(filePaths, pattern, maxMatches)
		}
		if queryChanged {
			sel = 0
		} else if sel >= len(matchIndices) {
			sel = 0
		}
	}

	applyMatchSelection := func() string {
		if len(matchIndices) == 0 || sel >= len(matchIndices) {
			return string(state.Text)
		}
		selected := filePaths[matchIndices[sel]]
		return fileio.ApplyFilenameSelection(currentDirPart, selected)
	}

	setPromptText := func(text string) {
		programmatic = true
		mbSetText(&state, []byte(text))
		programmatic = false
		lastQuery = text
	}

	// fpProvider returns the path string for the given index (for the match window).
	fpProvider := func(ctx any, idx uint) []byte {
		paths := ctx.([]string)
		if int(idx) >= len(paths) {
			return nil
		}
		return []byte(paths[int(idx)])
	}

	refreshList(".")
	syncMatches()

	app.State.ActiveMinibuffer = &state
	defer func() {
		app.State.ActiveMinibuffer = nil
		minibufferHideMatchWindow()
		DisplayUpdate()
	}()
	drainGlobalMinibufKeys()
	drainGlobalKeyCh()

	render := func() {
		p := prompt
		if len(matchIndices) > 0 {
			fctx := &fuzzyMatchCtx{
				provider:    fpProvider,
				providerCtx: filePaths,
			}
			fuzzyListRedraw(p, &state, fctx, matchIndices, sel)
		} else {
			minibufferHideMatchWindow()
			DisplayUpdate()
			mbWritePrompt(promptFormatWithCount(p, sel, len(matchIndices)), state.Text, int(state.CursorPos))
		}
	}
	render()

	for {
		k, ok := <-GlobalMinibufKeyCh
		if !ok {
			return PromptResultAbort
		}
		if isPasteRedrawKey(k) {
			DisplayUpdate()
			render()
			continue
		}

		changed := false

		switch {
		case k == KeyEnter || k == '\r' || k == '\n' || k == (CTL|'M') || k == (CTL|'J'):
			full := applyMatchSelection()
			if len(matchIndices) > 0 && sel < len(matchIndices) {
				selected := filePaths[matchIndices[sel]]
				if selected == "../" || strings.HasSuffix(selected, "/") {
					setPromptText(full)
					syncMatches()
					render()
					continue
				}
				setPromptText(full)
			}
			mbHistoryAdd(string(state.Text))
			n := copy(buf, state.Text)
			if n < len(buf) {
				buf[n] = 0
			}
			if PackageHooks.MacroRecordMinibufferResult != nil {
				PackageHooks.MacroRecordMinibufferResult(state.Text)
			}
			mbClear()
			return PromptResultYes

		case k == (CTL|'G') || k == 0x07 || k == 0x1B:
			mbClear()
			return PromptResultAbort

		case k == KeyTab:
			if len(matchIndices) > 0 && sel < len(matchIndices) {
				setPromptText(applyMatchSelection())
				syncMatches()
				changed = true
			} else if completePromptFilename(&state) {
				lastQuery = string(state.Text)
				syncMatches()
				changed = true
			} else {
				term.Beep()
			}

		case k == KeyUp || k == (CTL|'P'):
			if len(matchIndices) == 0 {
				term.Beep()
			} else {
				sel = (sel + len(matchIndices) - 1) % len(matchIndices)
				setPromptText(applyMatchSelection())
				syncMatches()
				changed = true
			}

		case k == KeyDown || k == (CTL|'N'):
			if len(matchIndices) == 0 {
				term.Beep()
			} else {
				sel = (sel + 1) % len(matchIndices)
				setPromptText(applyMatchSelection())
				syncMatches()
				changed = true
			}

		// Cursor movement
		case k == (CTL|'A') || k == KeyHome:
			if !mbGotoBol(&state) {
				term.Beep()
			}
		case k == (CTL|'E') || k == KeyEnd:
			if !mbGotoEol(&state) {
				term.Beep()
			}
		case k == (CTL|'B') || k == KeyLeft:
			if !mbBackwardChar(&state) {
				term.Beep()
			}
		case k == (CTL|'F') || k == KeyRight:
			if !mbForwardChar(&state) {
				term.Beep()
			}
		case k == (META|'B') || k == (SHIFT|KeyLeft):
			if !mbBackwardWord(&state) {
				term.Beep()
			}
		case k == (META|'F') || k == (SHIFT|KeyRight):
			if !mbForwardWord(&state) {
				term.Beep()
			}

		// Editing
		case k == 0x7F || k == (CTL|'H'):
			changed = mbDeleteBackward(&state)
			if !changed {
				term.Beep()
			}
		case k == (CTL|'D') || k == KeyDelete:
			changed = mbDeleteForward(&state)
			if !changed {
				term.Beep()
			}
		case k == (CTL | 'U'):
			changed = mbClearText(&state)
			if !changed {
				term.Beep()
			}
		case k == (CTL | 'K'):
			changed = mbKill(&state)
			if !changed {
				term.Beep()
			}
		case k == (META | 'D'):
			changed = mbDeleteWordForward(&state)
			if !changed {
				term.Beep()
			}
		case k == (META|'H') || k == (META|0x7F):
			changed = mbDeleteWordBackward(&state)
			if !changed {
				term.Beep()
			}

		default:
			if k < UnicodeLimit && k >= 0x20 && (k&KeyMask) == 0 {
				if mbInsertChar(&state, rune(k)) {
					changed = true
				} else {
					term.Beep()
				}
			} else {
				term.Beep()
			}
		}

		if changed {
			syncMatches()
		}
		render()
	}
}

// mbReadCommand opens the command-palette prompt (M-x).
func mbReadCommand(buf []byte, nbuf int) PromptResult {
	if PackageHooks.BuildCommandList == nil || PackageHooks.CommandsProvider == nil {
		return PromptResultAbort
	}
	names := PackageHooks.BuildCommandList()
	if len(names) == 0 {
		mbWrite("[no commands]")
		return PromptResultNo
	}
	return mbReadFuzzyList("M-x: ", PackageHooks.CommandsProvider, names, uint(len(names)), buf[:nbuf], nbuf)
}

// ---- Match window (passive *match* buffer for fuzzy pickers) -----------------

const fuzzyMaxMatches = 16

type fuzzyMatchCtx struct {
	provider         MbNameProviderFn
	providerCtx      any
	displayFormatter MbMatchFormatter
	displayCtx       any
	indices          []uint
}

func matchWindowGet() *Window {
	mbp := app.BufferFind("*match*")
	if mbp == nil {
		return nil
	}
	for i := 0; i < int(app.State.WindowCount); i++ {
		wp := app.State.WINDOWS[i]
		if wp != nil && wp.Buffer == mbp {
			return wp
		}
	}
	return nil
}

func matchWindowShow() {
	mbp := app.BufferFind("*match*")
	if mbp == nil {
		return
	}
	if matchWindowGet() != nil {
		return
	}
	wp := app.WindowCreate()
	if wp == nil {
		return
	}
	wp.Buffer = mbp
	wp.TopLine = 1
	wp.Cursor = Location{Line: 1, Offset: 0}
	wp.Mark = Location{Line: 0, Offset: 0}
	wp.ShouldRedraw = true
	wp.ShouldUpdateModeLine = true
	app.WindowRetile()
}

func matchWindowRemove() {
	mw := matchWindowGet()
	if mw == nil || app.State.WindowCount <= 1 {
		return
	}
	idx := -1
	for i := 0; i < int(app.State.WindowCount); i++ {
		if app.State.WINDOWS[i] == mw {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	if app.State.CurrentWindow == mw {
		newCur := app.State.WINDOWS[0]
		if idx == 0 && app.State.WindowCount > 1 {
			newCur = app.State.WINDOWS[1]
		}
		app.WindowSelect(newCur)
	}
	for i := idx; i < int(app.State.WindowCount)-1; i++ {
		app.State.WINDOWS[i] = app.State.WINDOWS[i+1]
	}
	app.State.WINDOWS[app.State.WindowCount-1] = nil
	app.State.WindowCount--
	app.WindowRetile()
}

func minibufferHideMatchWindow() {
	matchWindowRemove()
}

func matchWindowScrollToSelection(selected uint) {
	mw := matchWindowGet()
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

func fuzzyMatchFormatLine(ctx *fuzzyMatchCtx, out []byte, outSize uint, listIdx uint) {
	if int(listIdx) >= len(ctx.indices) {
		return
	}
	provIdx := ctx.indices[listIdx]
	if ctx.displayFormatter != nil {
		ctx.displayFormatter(out, outSize, provIdx, ctx.displayCtx)
		return
	}
	if ctx.provider == nil {
		return
	}
	name := ctx.provider(ctx.providerCtx, provIdx)
	if name == nil {
		return
	}
	n := len(name)
	if uint(n) >= outSize {
		n = int(outSize) - 1
	}
	if n < 0 {
		n = 0
	}
	copy(out, name[:n])
	out[n] = 0
}

func writeMatchBufferGeneric(formatter MbMatchFormatter, ctx any, count uint, selected uint) {
	if count == 0 {
		if app.BufferFind("*match*") != nil {
			minibufferHideMatchWindow()
			DisplayUpdate()
		}
		return
	}

	var out strings.Builder
	for i := uint(0); i < count; i++ {
		line := make([]byte, 512)
		formatter(line, uint(len(line)), i, ctx)
		end := 0
		for end < len(line) && line[end] != 0 {
			end++
		}
		if i == selected {
			out.WriteString("> ")
		} else {
			out.WriteString("  ")
		}
		out.Write(line[:end])
		out.WriteByte('\n')
	}

	mbp := app.BufferFind("*match*")
	if mbp == nil {
		mbp = app.BufferCreate(&app.State.EditorRuntimeState)
		if mbp == nil {
			return
		}
		mbp.Name = "*match*"
		mbp.LangMode = LModeNone
	}

	prevRO := mbp.IsReadonly
	mbp.IsReadonly = false
	text := []byte(out.String())
	eof := buffer.MakeLocation(mbp.EOF(), 0)
	bufferSetText(mbp, buffer.MakeLocation(1, 0), eof, text, nil, false)
	mbp.IsReadonly = prevRO
	mbp.IsReadonly = true

	matchWindowShow()
	matchWindowScrollToSelection(selected)
	DisplayUpdate()
}

func fuzzyMatchRefresh(matches []uint, sel int, ctx *fuzzyMatchCtx) {
	ctx.indices = matches
	count := uint(len(matches))
	if count > fuzzyMaxMatches {
		count = fuzzyMaxMatches
	}
	if count == 0 {
		writeMatchBufferGeneric(func([]byte, uint, uint, any) {}, ctx, 0, 0)
		return
	}
	if sel < 0 {
		sel = 0
	}
	if uint(sel) >= count {
		sel = int(count) - 1
	}
	writeMatchBufferGeneric(func(out []byte, outSize uint, idx uint, c any) {
		fuzzyMatchFormatLine(c.(*fuzzyMatchCtx), out, outSize, idx)
	}, ctx, count, uint(sel))
}

func fuzzyListRedraw(prompt string, state *MinibufferState, ctx *fuzzyMatchCtx, matches []uint, sel int) {
	fuzzyMatchRefresh(matches, sel, ctx)
	mbWritePrompt(promptFormatWithCount(prompt, sel, len(matches)), state.Text, int(state.CursorPos))
}

// ---- Fuzzy list prompt (generic) --------------------------------------------

// fuzzyScore computes a fuzzy match score for name against query.
// Returns (matched, score); higher score is better.
func fuzzyScore(name, query []byte) (bool, int) {
	if len(query) == 0 {
		return true, 1
	}
	n := len(name)
	q := len(query)
	ni := 0
	prev := -1
	totalGap := 0
	consecBonus := 0
	matched := 0
	for qi := 0; qi < q; qi++ {
		qc := query[qi]
		found := -1
		for ni < n {
			nc := name[ni]
			if nc >= 'A' && nc <= 'Z' {
				nc = nc - 'A' + 'a'
			}
			cc := qc
			if cc >= 'A' && cc <= 'Z' {
				cc = cc - 'A' + 'a'
			}
			if nc == cc {
				found = ni
				ni++
				break
			}
			ni++
		}
		if found == -1 {
			return false, 0
		}
		if prev != -1 {
			gap := found - prev - 1
			totalGap += gap
			if gap == 0 {
				consecBonus += 5
			}
		}
		prev = found
		matched++
	}
	score := matched*100 - totalGap*5 + consecBonus
	if prev >= 0 && prev < 3 {
		score += 20
	}
	return true, score
}

// fuzzyMatches returns up to maxMatches indices from provider that best match
// query, ordered by score descending.
func fuzzyMatches(provider MbNameProviderFn, ctx any, count uint, query []byte, maxMatches int) []uint {
	if count == 0 || maxMatches <= 0 {
		return nil
	}
	type entry struct {
		idx   uint
		score int
	}
	matches := make([]entry, 0, maxMatches)
	for i := uint(0); i < count; i++ {
		name := provider(ctx, i)
		if name == nil {
			continue
		}
		ok, sc := fuzzyScore(name, query)
		if !ok {
			continue
		}
		matches = append(matches, entry{idx: i, score: sc})
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(a, b int) bool {
		if matches[a].score != matches[b].score {
			return matches[a].score > matches[b].score
		}
		return matches[a].idx < matches[b].idx
	})
	n := len(matches)
	if n > maxMatches {
		n = maxMatches
	}
	out := make([]uint, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, matches[i].idx)
	}
	return out
}

// mbReadFuzzyListEx prompts the user with a live-filtering fuzzy list.
// Full cursor-movement editing of the query is supported.
func mbReadFuzzyListEx(prompt string, provider MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter MbMatchFormatter, displayCtx any, buf []byte, nbuf int) PromptResult {
	if PackageHooks.MacroPlayPrompt != nil {
		if pr, played := PackageHooks.MacroPlayPrompt(buf); played {
			return pr
		}
	}

	state := MinibufferState{
		Prompt:     prompt,
		Text:       make([]byte, 0, nbuf),
		Nbuf:       uint(nbuf),
		HistoryPos: -1,
	}
	sel := 0
	fctx := &fuzzyMatchCtx{
		provider:         provider,
		providerCtx:      providerCtx,
		displayFormatter: displayFormatter,
		displayCtx:       displayCtx,
	}

	matches := fuzzyMatches(provider, providerCtx, providerCount, state.Text, fuzzyMaxMatches)

	app.State.ActiveMinibuffer = &state
	defer func() {
		app.State.ActiveMinibuffer = nil
		minibufferHideMatchWindow()
		DisplayUpdate()
	}()
	drainGlobalMinibufKeys()
	drainGlobalKeyCh()

	fuzzyListRedraw(prompt, &state, fctx, matches, sel)

	for {
		k, ok := <-GlobalMinibufKeyCh
		if !ok {
			return PromptResultAbort
		}
		if isPasteRedrawKey(k) {
			DisplayUpdate()
			fuzzyListRedraw(prompt, &state, fctx, matches, sel)
			continue
		}

		changed := false

		switch {
		case k == KeyEnter || k == '\r' || k == '\n' || k == (CTL|'M') || k == (CTL|'J'):
			if len(matches) > 0 && sel >= 0 && sel < len(matches) {
				label := provider(providerCtx, matches[sel])
				if label != nil {
					n := copy(buf, label)
					if n < len(buf) {
						buf[n] = 0
					}
					if PackageHooks.MacroRecordMinibufferResult != nil {
						PackageHooks.MacroRecordMinibufferResult(label)
					}
				}
				mbClear()
				return PromptResultYes
			}
			mbClear()
			return PromptResultAbort

		case k == (CTL|'G') || k == 0x07 || k == 0x1B:
			mbClear()
			return PromptResultAbort

		// Match-list navigation (no query change)
		case k == KeyUp || k == (CTL|'P'):
			if len(matches) == 0 {
				term.Beep()
			} else {
				sel = (sel + len(matches) - 1) % len(matches)
			}
		case k == KeyDown || k == (CTL|'N'):
			if len(matches) == 0 {
				term.Beep()
			} else {
				sel = (sel + 1) % len(matches)
			}

		// Cursor movement within query
		case k == (CTL|'A') || k == KeyHome:
			if !mbGotoBol(&state) {
				term.Beep()
			}
		case k == (CTL|'E') || k == KeyEnd:
			if !mbGotoEol(&state) {
				term.Beep()
			}
		case k == (CTL|'B') || k == KeyLeft:
			if !mbBackwardChar(&state) {
				term.Beep()
			}
		case k == (CTL|'F') || k == KeyRight:
			if !mbForwardChar(&state) {
				term.Beep()
			}
		case k == (META|'B') || k == (SHIFT|KeyLeft):
			if !mbBackwardWord(&state) {
				term.Beep()
			}
		case k == (META|'F') || k == (SHIFT|KeyRight):
			if !mbForwardWord(&state) {
				term.Beep()
			}

		// Query editing
		case k == 0x7F || k == (CTL|'H'):
			changed = mbDeleteBackward(&state)
			if !changed {
				term.Beep()
			}
		case k == (CTL | 'D'):
			changed = mbDeleteForward(&state)
			if !changed {
				term.Beep()
			}
		case k == (CTL | 'U'):
			changed = mbClearText(&state)
			if !changed {
				term.Beep()
			}

		default:
			if k < UnicodeLimit && k >= 0x20 && (k&KeyMask) == 0 {
				if mbInsertChar(&state, rune(k)) {
					changed = true
				} else {
					term.Beep()
				}
			} else {
				term.Beep()
			}
		}

		if changed {
			matches = fuzzyMatches(provider, providerCtx, providerCount, state.Text, fuzzyMaxMatches)
			sel = 0
		}
		fuzzyListRedraw(prompt, &state, fctx, matches, sel)
	}
}

// mbReadFuzzyList is a convenience wrapper around mbReadFuzzyListEx with no
// custom display formatter.
func mbReadFuzzyList(prompt string, provider MbNameProviderFn, providerCtx any, providerCount uint, buf []byte, nbuf int) PromptResult {
	return mbReadFuzzyListEx(prompt, provider, providerCtx, providerCount, nil, nil, buf, nbuf)
}

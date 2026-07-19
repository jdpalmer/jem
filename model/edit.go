package model

import (
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
)

var defaultHistory buffer.UndoHistory

// History is the active undo stack. Bound by editor.Editor.Activate.
var History *buffer.UndoHistory = &defaultHistory

// BindHistory points History at h. Pass nil to restore the package default.
func BindHistory(h *buffer.UndoHistory) {
	if h == nil {
		History = &defaultHistory
		return
	}
	History = h
}

// ResetHistory clears the currently bound undo stack.
func ResetHistory() {
	*History = buffer.UndoHistory{}
}

func BeginCommand() {
	wp := State.CurrentWindow
	if History.IsReplaying || State.CurrentBuffer == nil || wp == nil {
		return
	}
	History.BeginCommand(State.CurrentBuffer, buffer.MakeLocation(wp.Cursor.Line, wp.Cursor.Offset))
}

func EndCommand() {
	History.EndCommand()
}

func ForgetBuffer(bp *buffer.Buffer) {
	History.ForgetBuffer(bp)
}

func NoteBufferSaved(bp *buffer.Buffer) {
	History.NoteBufferSaved(bp)
}

// SetText replaces [begin, end) with newText under History via buffer.SetText
// (undo, NoteEdit, adjust locations, syntax).
func SetText(bp *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error {
	if bp == nil {
		return buffer.ErrNilBuffer
	}
	return bp.SetText(History, begin, end, newText, newEndOut)
}

// InsertText inserts text at the window cursor and advances the cursor.
func InsertText(wp *Window, text []byte) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	bp := wp.Buffer
	BeginCommand()
	defer EndCommand()
	begin := wp.Cursor
	var newEnd buffer.Location
	if err := SetText(bp, begin, begin, text, &newEnd); err != nil {
		return false
	}
	wp.Cursor = newEnd
	wp.DidEdit = true
	return true
}

// InsertCodepoint inserts a Unicode codepoint at the window cursor.
func InsertCodepoint(wp *Window, cp rune) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	if cp < 0 {
		return false
	}
	if cp < 0x80 {
		return InsertText(wp, []byte{byte(cp)})
	}
	buf := make([]byte, utf8.RuneLen(cp))
	n := utf8.EncodeRune(buf, cp)
	return InsertText(wp, buf[:n])
}

// InsertNewline inserts a single newline at the window cursor.
func InsertNewline(wp *Window) bool {
	return InsertText(wp, []byte{'\n'})
}

// InsertPaste inserts bracketed-paste text at the window cursor (\r → \n).
func InsertPaste(wp *Window, text []byte) bool {
	if wp == nil || wp.Buffer == nil || len(text) == 0 {
		return false
	}
	paste := append([]byte(nil), text...)
	for i := range paste {
		if paste[i] == '\r' {
			paste[i] = '\n'
		}
	}
	return InsertText(wp, paste)
}

// MinibufferInsertPaste inserts bracketed-paste text into the active minibuffer.
func MinibufferInsertPaste(text []byte) bool {
	state := State.ActiveMinibuffer
	if state == nil || len(text) == 0 {
		return false
	}
	if state.Text == nil || state.Nbuf+uint(len(text)) >= uint(cap(state.Text)) {
		return false
	}

	paste := append([]byte(nil), text...)
	for i := range paste {
		if paste[i] == '\r' {
			paste[i] = '\n'
		}
	}

	insertLen := uint(len(paste))
	if state.Nbuf+insertLen >= uint(cap(state.Text)) {
		insertLen = uint(cap(state.Text)) - state.Nbuf
	}
	copy(state.Text[state.CursorPos:], paste[:insertLen])
	state.Nbuf += insertLen
	state.CursorPos += insertLen
	state.HaveSavedEdit = false
	return true
}

// MarkPasteDirty marks windows for redraw after a buffer paste (not minibuffer).
func MarkPasteDirty() {
	if State.ActiveMinibuffer != nil {
		return
	}
	State.ScreenDirty = true
	for i := 0; i < int(len(State.Windows)); i++ {
		wp := State.Windows[i]
		if wp != nil {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
	}
}

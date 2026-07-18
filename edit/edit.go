// Package edit provides undo-aware buffer edits that leaf packages (modes,
// search helpers) can call without importing editor (import cycle).
package edit

import (
	"unicode/utf8"

	"github.com/jdpalmer/jem/app"
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
	wp := app.State.CurrentWindow
	if History.IsReplaying || app.State.CurrentBuffer == nil || wp == nil {
		return
	}
	History.BeginCommand(app.State.CurrentBuffer, buffer.MakeLocation(wp.Cursor.Line, wp.Cursor.Offset))
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
// (the edit-session entry: undo, NoteEdit, adjust locations, syntax).
func SetText(bp *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error {
	if bp == nil {
		return buffer.ErrNilBuffer
	}
	return bp.SetText(History, begin, end, newText, newEndOut)
}

// InsertText inserts text at the window cursor and advances the cursor.
func InsertText(wp *app.Window, text []byte) bool {
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
func InsertCodepoint(wp *app.Window, cp rune) bool {
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
func InsertNewline(wp *app.Window) bool {
	return InsertText(wp, []byte{'\n'})
}

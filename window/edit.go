package window

import (
	"bytes"
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/syntax"
)

// SetText is the interactive edit entry point: undo on History, text replace,
// window cursor/mark/topline adjust, dirty flags, and syntax reparse.
// Use this from commands, modes, and search. Prefer buffer.SetText only when
// filling a buffer that is not being edited in a window (e.g. tool output).
func SetText(buf *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error {
	if buf == nil {
		return buffer.ErrBadRange
	}
	if buf.IsReadonly {
		return buffer.ErrReadonly
	}
	isStructural := begin.Line != end.Line || bytes.IndexByte(newText, '\n') >= 0
	if buffer.History != nil {
		oldText := buf.GetText(begin, end)
		buffer.History.RecordEdit(buf, buffer.History.Pending.Before, begin, oldText, newText)
	}
	meta, err := buf.ReplaceRaw(begin, end, newText, newEndOut)
	if err != nil {
		return err
	}
	NotifyReplace(buf, begin, meta, isStructural)
	buf.IsChanged = true
	return nil
}

// NotifyReplace updates windows and syntax after a ReplaceRaw.
// Call before setting buf.IsChanged so modeline first-change detection works.
func NotifyReplace(buf *buffer.Buffer, begin buffer.Location, meta buffer.ReplaceMeta, isStructural bool) {
	if buf == nil {
		return
	}
	AdjustLocationsAfterReplace(buf, begin, meta.NormEnd, meta.NewEnd)
	NoteBufferEdit(buf, isStructural)
	syntax.IncrementalReparse(buf, meta.FirstLine)
}

// InsertText inserts text at the window cursor and advances the cursor.
func InsertText(win *Window, text []byte) error {
	if win == nil || win.Buffer == nil {
		return ErrNilWindow
	}
	buf := win.Buffer
	buffer.BeginCommand(win.Cursor)
	defer buffer.EndCommand()
	begin := win.Cursor
	var newEnd buffer.Location
	if err := SetText(buf, begin, begin, text, &newEnd); err != nil {
		return err
	}
	win.Cursor = newEnd
	win.DidEdit = true
	return nil
}

// InsertCodepoint inserts a Unicode codepoint at the window cursor.
func InsertCodepoint(win *Window, r rune) error {
	if win == nil || win.Buffer == nil {
		return ErrNilWindow
	}
	if r < 0 {
		return ErrBadRune
	}
	if r < 0x80 {
		return InsertText(win, []byte{byte(r)})
	}
	encoded := make([]byte, utf8.RuneLen(r))
	n := utf8.EncodeRune(encoded, r)
	return InsertText(win, encoded[:n])
}

// InsertNewline inserts a single newline at the window cursor.
func InsertNewline(win *Window) error {
	return InsertText(win, []byte{'\n'})
}

// InsertPaste inserts bracketed-paste text at the window cursor (\r → \n).
func InsertPaste(win *Window, text []byte) error {
	if win == nil || win.Buffer == nil {
		return ErrNilWindow
	}
	if len(text) == 0 {
		return nil
	}
	paste := append([]byte(nil), text...)
	for i := range paste {
		if paste[i] == '\r' {
			paste[i] = '\n'
		}
	}
	return InsertText(win, paste)
}

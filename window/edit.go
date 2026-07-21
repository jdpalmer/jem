package window

import (
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
)

// InsertText inserts text at the window cursor and advances the cursor.
func InsertText(win *Window, text []byte) error {
	if win == nil || win.Buffer == nil {
		return ErrNilWindow
	}
	buf := win.Buffer
	if PackageHooks.BeginCommand != nil {
		PackageHooks.BeginCommand()
	}
	if PackageHooks.EndCommand != nil {
		defer PackageHooks.EndCommand()
	}
	begin := win.Cursor
	var newEnd buffer.Location
	if PackageHooks.SetText == nil {
		return ErrNoEditHook
	}
	if err := PackageHooks.SetText(buf, begin, begin, text, &newEnd); err != nil {
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

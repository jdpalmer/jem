package window

import (
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
)

// InsertText inserts text at the window cursor and advances the cursor.
func InsertText(wp *Window, text []byte) error {
	if wp == nil || wp.Buffer == nil {
		return ErrNilWindow
	}
	bp := wp.Buffer
	if PackageHooks.BeginCommand != nil {
		PackageHooks.BeginCommand()
	}
	if PackageHooks.EndCommand != nil {
		defer PackageHooks.EndCommand()
	}
	begin := wp.Cursor
	var newEnd buffer.Location
	if PackageHooks.SetText == nil {
		return ErrNoEditHook
	}
	if err := PackageHooks.SetText(bp, begin, begin, text, &newEnd); err != nil {
		return err
	}
	wp.Cursor = newEnd
	wp.DidEdit = true
	return nil
}

// InsertCodepoint inserts a Unicode codepoint at the window cursor.
func InsertCodepoint(wp *Window, cp rune) error {
	if wp == nil || wp.Buffer == nil {
		return ErrNilWindow
	}
	if cp < 0 {
		return ErrBadRune
	}
	if cp < 0x80 {
		return InsertText(wp, []byte{byte(cp)})
	}
	buf := make([]byte, utf8.RuneLen(cp))
	n := utf8.EncodeRune(buf, cp)
	return InsertText(wp, buf[:n])
}

// InsertNewline inserts a single newline at the window cursor.
func InsertNewline(wp *Window) error {
	return InsertText(wp, []byte{'\n'})
}

// InsertPaste inserts bracketed-paste text at the window cursor (\r → \n).
func InsertPaste(wp *Window, text []byte) error {
	if wp == nil || wp.Buffer == nil {
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
	return InsertText(wp, paste)
}

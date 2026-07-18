package editor

import (
	"testing"

	"github.com/jdpalmer/jem/app"
)

func TestMouseLeftClick(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("hello world")
	te.SetCursor(1, 0)

	gutter := app.WindowGutterWidth(te.WP())
	te.Click(0, gutter+5)
	te.ExpectCursor(1, 5)
}

package runtime

import (
	"testing"
)

func TestMouseLeftClick(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("hello world")
	te.SetCursor(1, 0)

	gutter := te.WP().GutterWidth()
	te.Click(0, gutter+5)
	te.ExpectCursor(1, 5)
}

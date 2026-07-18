package editor

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"testing"
)

func TestCmdEdit(t *testing.T) {
	te := NewTestEditor(t)

	te.LoadText("abcde")
	te.SetCursor(1, 2)
	te.Press("C-t")
	te.ExpectText("acbde")

	te.LoadText("hello")
	te.SetCursor(1, 0)
	te.Press("C-d")
	te.ExpectText("ello")

	te.SetCursor(1, 3)
	te.Key(0x7F)
	te.ExpectText("elo")

	te.LoadText("café")
	te.SetCursor(1, uint(len(te.BufferText())))
	te.Key(0x7F)
	te.ExpectText("caf")

	te.LoadText("hello world")
	te.SetCursor(1, 5)
	app.State.KillState = app.CmdStateNone
	te.Press("C-k")
	te.ExpectText("hello")
	te.Press("C-y")
	te.ExpectText("hello world")
}

func TestBufferSetText(t *testing.T) {
	te := NewTestEditor(t)

	te.LoadText("hello\nworld")
	te.Edit(buffer.MakeLocation(1, 3), buffer.MakeLocation(2, 2), "")
	te.ExpectText("helrld")
	te.ExpectLineCount(1)

	te.LoadText("hello world")
	te.Edit(buffer.MakeLocation(1, 6), buffer.MakeLocation(1, 11), "a\nb")
	te.ExpectText("hello a\nb")
	te.ExpectLineCount(2)
}

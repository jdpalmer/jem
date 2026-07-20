package runtime

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestUndoContent(t *testing.T) {
	te := NewTestEditor(t)

	te.LoadText("hello")
	te.ForgetUndo()
	te.Edit(buffer.MakeLocation(1, 5), buffer.MakeLocation(1, 5), " world")
	te.ExpectText("hello world")
	te.Undo()
	te.ExpectText("hello")

	te.LoadText("hello world")
	te.ForgetUndo()
	te.Edit(buffer.MakeLocation(1, 5), buffer.MakeLocation(1, 11), "")
	te.ExpectText("hello")
	te.Undo()
	te.ExpectText("hello world")

	te.LoadText("helloworld")
	te.ForgetUndo()
	te.Edit(buffer.MakeLocation(1, 5), buffer.MakeLocation(1, 5), "\n")
	te.ExpectLineCount(2)
	te.Undo()
	te.ExpectLineCount(1)
	te.ExpectText("helloworld")
}

func TestUndoCleanState(t *testing.T) {
	te := NewTestEditor(t)

	te.LoadText("hello")
	NoteBufferSaved(te.BP())
	te.ExpectChanged(false)

	te.Edit(buffer.MakeLocation(1, 5), buffer.MakeLocation(1, 5), " world")
	te.ExpectChanged(true)

	te.Undo()
	te.ExpectChanged(false)
}

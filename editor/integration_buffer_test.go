package editor

import (
	"testing"

	"github.com/jdpalmer/jem/app"
)

func TestBufferSwitchRestoresCursor(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("buffer one")
	te.SetCursor(1, 3)
	bp1 := te.BP()

	bp2 := app.BufferCreate(&app.State.EditorRuntimeState)
	if bp2 == nil {
		t.Fatal("buffer create failed")
	}
	bp2.Name = "two"
	editorSwitchBuffer(bp2)
	te.LoadText("second")
	te.SetCursor(1, 4)

	editorSwitchBuffer(bp1)
	te.ExpectCursor(1, 3)
}

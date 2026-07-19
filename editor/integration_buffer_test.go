package editor

import (
	"testing"

	"github.com/jdpalmer/jem/model"
)

func TestBufferSwitchRestoresCursor(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("buffer one")
	te.SetCursor(1, 3)
	bp1 := te.BP()

	bp2 := model.BufferCreate(&model.State.EditorRuntimeState)
	if bp2 == nil {
		t.Fatal("buffer create failed")
	}
	bp2.Name = "two"
	model.SwitchBuffer(bp2)
	te.LoadText("second")
	te.SetCursor(1, 4)

	model.SwitchBuffer(bp1)
	te.ExpectCursor(1, 3)
}

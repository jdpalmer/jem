package runtime

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/window"
	"testing"
)

func TestBufferSwitchRestoresCursor(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("buffer one")
	te.SetCursor(1, 3)
	bp1 := te.BP()

	bp2 := buffer.Create()
	if bp2 == nil {
		t.Fatal("buffer create failed")
	}
	bp2.Name = "two"
	window.SwitchBuffer(bp2)
	te.LoadText("second")
	te.SetCursor(1, 4)

	window.SwitchBuffer(bp1)
	te.ExpectCursor(1, 3)
}

package runtime

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/window"
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

func TestPickBufferListNames(t *testing.T) {
	Reset()
	bp1 := buffer.Create()
	bp2 := buffer.Create()
	if bp1 == nil || bp2 == nil {
		t.Fatal("buffer create failed")
	}
	bp1.Name = "alpha"
	bp2.Name = "beta"

	list := pickBufferList()
	if len(list) != 2 {
		t.Fatalf("pickBufferList len = %d, want 2", len(list))
	}
	if list[0].Name != "alpha" || list[1].Name != "beta" {
		t.Fatalf("pickBufferList order = %q, %q", list[0].Name, list[1].Name)
	}
	if findBufferByLabel("ALPHA") != bp1 {
		t.Fatal("findBufferByLabel should match case-insensitively")
	}
}

func TestCmdUseBufferDirectIndex(t *testing.T) {
	Reset()
	bp1 := buffer.Create()
	bp2 := buffer.Create()
	if bp1 == nil || bp2 == nil {
		t.Fatal("buffer create failed")
	}
	bp1.Name = "one"
	bp2.Name = "two"
	win := window.WindowCreate()
	if win == nil {
		t.Fatal("window create failed")
	}
	win.Buffer = bp1
	window.WindowSelect(win)

	if !CmdUseBuffer(true, 2) {
		t.Fatal("CmdUseBuffer with universal arg n=2 failed")
	}
	if buffer.All.Current != bp2 {
		t.Fatal("expected second buffer selected")
	}
}

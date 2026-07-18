package editor

import (
	"testing"

	"github.com/jdpalmer/jem/app"
)

func TestPickBufferListNames(t *testing.T) {
	app.State = App{}
	bp1 := app.BufferCreate(&app.State.EditorRuntimeState)
	bp2 := app.BufferCreate(&app.State.EditorRuntimeState)
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
	app.State = App{}
	bp1 := app.BufferCreate(&app.State.EditorRuntimeState)
	bp2 := app.BufferCreate(&app.State.EditorRuntimeState)
	if bp1 == nil || bp2 == nil {
		t.Fatal("buffer create failed")
	}
	bp1.Name = "one"
	bp2.Name = "two"
	wp := app.WindowCreate()
	if wp == nil {
		t.Fatal("window create failed")
	}
	app.WindowSelect(wp)

	if !CmdUseBuffer(true, 2) {
		t.Fatal("CmdUseBuffer with universal arg n=2 failed")
	}
	if app.State.CurrentBuffer != bp2 {
		t.Fatal("expected second buffer selected")
	}
}

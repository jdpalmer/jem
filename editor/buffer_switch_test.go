package editor

import (
	"testing"

	"github.com/jdpalmer/jem/model"
)

func TestPickBufferListNames(t *testing.T) {
	model.Reset()
	bp1 := model.BufferCreate(&model.State.EditorRuntimeState)
	bp2 := model.BufferCreate(&model.State.EditorRuntimeState)
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
	model.Reset()
	bp1 := model.BufferCreate(&model.State.EditorRuntimeState)
	bp2 := model.BufferCreate(&model.State.EditorRuntimeState)
	if bp1 == nil || bp2 == nil {
		t.Fatal("buffer create failed")
	}
	bp1.Name = "one"
	bp2.Name = "two"
	wp := model.WindowCreate()
	if wp == nil {
		t.Fatal("window create failed")
	}
	model.WindowSelect(wp)

	if !CmdUseBuffer(true, 2) {
		t.Fatal("CmdUseBuffer with universal arg n=2 failed")
	}
	if model.State.CurrentBuffer != bp2 {
		t.Fatal("expected second buffer selected")
	}
}

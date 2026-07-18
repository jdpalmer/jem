package editor

import (
	"testing"

	sess "github.com/jdpalmer/jem/session"
)

func TestPickBufferListNames(t *testing.T) {
	*session.App = App{}
	bp1 := sess.BufferCreate(&session.App.EditorRuntimeState)
	bp2 := sess.BufferCreate(&session.App.EditorRuntimeState)
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
	*session.App = App{}
	bp1 := sess.BufferCreate(&session.App.EditorRuntimeState)
	bp2 := sess.BufferCreate(&session.App.EditorRuntimeState)
	if bp1 == nil || bp2 == nil {
		t.Fatal("buffer create failed")
	}
	bp1.Name = "one"
	bp2.Name = "two"
	wp := sess.WindowCreate()
	if wp == nil {
		t.Fatal("window create failed")
	}
	sess.WindowSelect(wp)

	if !CmdUseBuffer(true, 2) {
		t.Fatal("CmdUseBuffer with universal arg n=2 failed")
	}
	if session.App.CurrentBuffer != bp2 {
		t.Fatal("expected second buffer selected")
	}
}

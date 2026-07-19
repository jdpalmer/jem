package editor

import (
	"testing"

	"github.com/jdpalmer/jem/model"
)

func TestCmdYankDoesNotUseStaleKillRing(t *testing.T) {
	// Regression: C-y must read the pasteboard, not reuse an old kill-ring entry.
	oldReady := model.ClipboardReady
	model.ClipboardReady = false
	t.Cleanup(func() { model.ClipboardReady = oldReady })

	te := NewTestEditor(t)
	te.LoadText("hello")
	model.KillBegin()
	_ = model.KillAppend([]byte("stale kill ring text"))
	model.State.KillState = model.CmdStateNone

	if CmdYank(false, 1) {
		t.Fatal("CmdYank should fail when clipboard is unavailable")
	}
	te.ExpectText("hello")
}

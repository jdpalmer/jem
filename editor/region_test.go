package editor

import (
	"testing"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/edit"
)

func TestCmdYankDoesNotUseStaleKillRing(t *testing.T) {
	// Regression: C-y must read the pasteboard, not reuse an old kill-ring entry.
	oldReady := edit.ClipboardReady
	edit.ClipboardReady = false
	t.Cleanup(func() { edit.ClipboardReady = oldReady })

	te := NewTestEditor(t)
	te.LoadText("hello")
	edit.KillBegin()
	_ = edit.KillAppend([]byte("stale kill ring text"))
	app.State.KillState = app.CmdStateNone

	if CmdYank(false, 1) {
		t.Fatal("CmdYank should fail when clipboard is unavailable")
	}
	te.ExpectText("hello")
}

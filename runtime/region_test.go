package runtime

import (
	"github.com/jdpalmer/jem/killring"
	"testing"
)

func TestCmdYankDoesNotUseStaleKillRing(t *testing.T) {
	// Regression: C-y must read the pasteboard, not reuse an old kill-ring entry.
	oldReady := killring.ClipboardReady
	killring.ClipboardReady = false
	t.Cleanup(func() { killring.ClipboardReady = oldReady })

	te := NewTestEditor(t)
	te.LoadText("hello")
	killring.KillBegin()
	_ = killring.KillAppend([]byte("stale kill ring text"))
	killring.KillState = killring.CmdStateNone

	if CmdYank(false, 1) {
		t.Fatal("CmdYank should fail when clipboard is unavailable")
	}
	te.ExpectText("hello")
}

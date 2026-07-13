package editor

import "testing"

func TestCmdYankDoesNotUseStaleKillRing(t *testing.T) {
	// Regression: C-y must read the pasteboard, not reuse an old kill-ring entry.
	oldReady := clipboardReady
	clipboardReady = false
	t.Cleanup(func() { clipboardReady = oldReady })

	te := NewTestEditor(t)
	te.LoadText("hello")
	killAggregate = []byte("stale kill ring text")

	if CmdYank(false, 1) {
		t.Fatal("CmdYank should fail when clipboard is unavailable")
	}
	te.ExpectText("hello")
}

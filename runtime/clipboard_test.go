package runtime

import (
	"github.com/jdpalmer/jem/killring"
	"testing"
)

func TestClipboardWriteOSC52Fallback(t *testing.T) {
	// OSC52 path is used when native clipboard is unavailable (SSH, headless tests).
	killring.ClipboardReady = false
	ok := killring.ClipboardWrite([]byte("hi"))
	if !ok {
		t.Fatal("clipboardWriteOSC52 failed")
	}
}

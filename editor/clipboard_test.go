package editor

import (
	"testing"

	"github.com/jdpalmer/jem/edit"
)

func TestClipboardWriteOSC52Fallback(t *testing.T) {
	// OSC52 path is used when native clipboard is unavailable (SSH, headless tests).
	edit.ClipboardReady = false
	ok := edit.ClipboardWrite([]byte("hi"))
	if !ok {
		t.Fatal("clipboardWriteOSC52 failed")
	}
}

package editor

import (
	"testing"

	"github.com/jdpalmer/jem/model"
)

func TestClipboardWriteOSC52Fallback(t *testing.T) {
	// OSC52 path is used when native clipboard is unavailable (SSH, headless tests).
	model.ClipboardReady = false
	ok := model.ClipboardWrite([]byte("hi"))
	if !ok {
		t.Fatal("clipboardWriteOSC52 failed")
	}
}

package editor

import (
	"encoding/base64"
	"os"
	"testing"
)

func TestClipboardWriteOSC52(t *testing.T) {
	// OSC52 path is used when native clipboard is unavailable (SSH, headless tests).
	clipboardReady = false
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	ok := clipboardWriteText([]byte("hi"))
	w.Close()
	if !ok {
		t.Fatal("clipboardWriteOSC52 failed")
	}

	buf := make([]byte, 256)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if out[:len("\x1b]52;c;")] != "\x1b]52;c;" {
		t.Fatalf("unexpected OSC52 prefix: %q", out)
	}
	payload := out[len("\x1b]52;c;") : len(out)-1]
	if payload != base64.StdEncoding.EncodeToString([]byte("hi")) {
		t.Fatalf("OSC52 payload = %q", payload)
	}
}

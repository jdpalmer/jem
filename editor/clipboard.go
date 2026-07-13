package editor

import (
	"encoding/base64"
	"os"

	"golang.design/x/clipboard"
)

var clipboardReady bool

// clipboardWriteOSC52 sets the clipboard via the OSC 52 escape sequence.
func clipboardWriteOSC52(data []byte) bool {
	enc := base64.StdEncoding.EncodeToString(data)
	osc52 := "\x1b]52;c;" + enc + "\x07"
	_, err := os.Stdout.WriteString(osc52)
	return err == nil
}

// clipboardWriteText writes bytes to the system clipboard. Returns true on success.
func clipboardWriteText(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if clipboardReady {
		if err := clipboard.Write(clipboard.FmtText, data); err == nil {
			return true
		}
	}
	return clipboardWriteOSC52(data)
}

// clipboardReadText attempts to read from the system clipboard. Returns data, ok.
func clipboardReadText() ([]byte, bool) {
	if !clipboardReady {
		return nil, false
	}
	data := clipboard.Read(clipboard.FmtText)
	if data == nil {
		return nil, false
	}
	return data, true
}

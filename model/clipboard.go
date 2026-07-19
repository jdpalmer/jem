package model

import (
	"encoding/base64"
	"os"

	"golang.design/x/clipboard"
)

// ClipboardReady is set true after a successful clipboard.Init at editor startup.
var ClipboardReady bool

func clipboardWriteOSC52(data []byte) bool {
	enc := base64.StdEncoding.EncodeToString(data)
	osc52 := "\x1b]52;c;" + enc + "\x07"
	_, err := os.Stdout.WriteString(osc52)
	return err == nil
}

// ClipboardWrite writes bytes to the system clipboard (native or OSC 52).
func ClipboardWrite(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if ClipboardReady {
		if err := clipboard.Write(clipboard.FmtText, data); err == nil {
			return true
		}
	}
	return clipboardWriteOSC52(data)
}

// ClipboardRead attempts to read from the system clipboard.
func ClipboardRead() ([]byte, bool) {
	if !ClipboardReady {
		return nil, false
	}
	data := clipboard.Read(clipboard.FmtText)
	if data == nil {
		return nil, false
	}
	return data, true
}

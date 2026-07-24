package window

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestInsertPaste(t *testing.T) {
	buf := buffer.New()
	win := &Window{Buffer: buf, Cursor: buffer.Location{Line: 1, Offset: 0}}
	Active.CurrentWindow = win
	Active.Windows = []*Window{win}
	buffer.SetCurrent(buf)

	if err := InsertPaste(win, []byte("hel\rlo")); err != nil {
		t.Fatalf("InsertPaste failed: %v", err)
	}
	if got := string(buf.Line(1).Data); got != "hel" {
		t.Fatalf("line 1 after paste = %q, want hel", got)
	}
	if got := string(buf.Line(2).Data); got != "lo" {
		t.Fatalf("line 2 after paste = %q, want lo", got)
	}
}

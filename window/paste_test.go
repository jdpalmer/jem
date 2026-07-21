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

	old := PackageHooks
	PackageHooks = Hooks{
		BeginCommand: func() {},
		EndCommand:   func() {},
		SetText: func(b *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error {
			return b.SetText(nil, begin, end, newText, newEndOut)
		},
	}
	defer func() { PackageHooks = old }()

	if err := InsertPaste(win, []byte("hel\rlo")); err != nil {
		t.Fatalf("InsertPaste failed: %v", err)
	}
	if got := string(buf.Line(1).Data); got != "hel" {
		t.Fatalf("line 1 after paste = %q, want hel", got)
	}
	if len(buf.Lines) < 2 {
		t.Fatal("paste with CR should create a second line")
	}
	if got := string(buf.Line(2).Data); got != "lo" {
		t.Fatalf("line 2 after paste = %q, want lo", got)
	}
}

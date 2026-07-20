package window

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestInsertPaste(t *testing.T) {
	bp := buffer.New()
	wp := &Window{Buffer: bp, Cursor: buffer.Location{Line: 1, Offset: 0}}
	Active.CurrentWindow = wp
	Active.Windows = []*Window{wp}

	old := PackageHooks
	PackageHooks = Hooks{
		BeginCommand: func() {},
		EndCommand:   func() {},
		SetText: func(b *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error {
			return b.SetText(nil, begin, end, newText, newEndOut)
		},
	}
	defer func() { PackageHooks = old }()

	if !InsertPaste(wp, []byte("hel\rlo")) {
		t.Fatal("InsertPaste failed")
	}
	if got := string(bp.Line(1).Data); got != "hel" {
		t.Fatalf("line 1 after paste = %q, want hel", got)
	}
	if bp.LineCount < 2 {
		t.Fatal("paste with CR should create a second line")
	}
	if got := string(bp.Line(2).Data); got != "lo" {
		t.Fatalf("line 2 after paste = %q, want lo", got)
	}
}

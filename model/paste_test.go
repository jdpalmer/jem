package model

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestInsertPaste(t *testing.T) {
	bp := buffer.New()
	wp := &Window{Buffer: bp, Cursor: buffer.Location{Line: 1, Offset: 0}}
	State.CurrentWindow = wp
	State.Windows = []*Window{wp}
	State.ActiveMinibuffer = nil

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
	MarkPasteDirty()
	if !wp.ShouldRedraw {
		t.Fatal("MarkPasteDirty should mark window for redraw")
	}
}

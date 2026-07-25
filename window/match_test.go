package window

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/term"
)

func withFreshWindowState(t *testing.T) {
	t.Helper()
	term.SetSize(24, 80)
	Active.Windows = nil
	Active.CurrentWindow = nil
	buffer.All.Buffers = nil
	buffer.All.Current = nil
	t.Cleanup(func() {
		Active.Windows = nil
		Active.CurrentWindow = nil
		buffer.All.Buffers = nil
		buffer.All.Current = nil
	})
}

func TestSetMatchBufferTextMarksRedraw(t *testing.T) {
	withFreshWindowState(t)

	// Need a primary window so ShowMatchWindow can split.
	primary := WindowCreate()
	if primary == nil {
		t.Fatal("WindowCreate failed")
	}
	primary.Buffer = buffer.Create()
	Active.CurrentWindow = primary

	SetMatchBufferText([]byte("→ one\n  two\n"), 0)
	mw := MatchWindow()
	if mw == nil {
		t.Fatal("expected match window")
	}
	if !mw.ShouldRedraw {
		t.Fatal("first SetMatchBufferText should mark ShouldRedraw")
	}
	if mw.Cursor.Line != 1 {
		t.Fatalf("Cursor.Line = %d, want 1", mw.Cursor.Line)
	}
	mw.ShouldRedraw = false

	SetMatchBufferText([]byte("  one\n→ two\n"), 1)
	mw = MatchWindow()
	if mw == nil {
		t.Fatal("expected match window after update")
	}
	if !mw.ShouldRedraw {
		t.Fatal("selection change should mark ShouldRedraw")
	}
	if mw.Cursor.Line != 2 {
		t.Fatalf("Cursor.Line = %d, want 2 for selected index 1", mw.Cursor.Line)
	}
	line1 := mw.Buffer.Line(1)
	line2 := mw.Buffer.Line(2)
	if line1 == nil || line2 == nil {
		t.Fatal("expected two match lines")
	}
	if string(line1.Data) != "  one" || string(line2.Data) != "→ two" {
		t.Fatalf("match lines = %q / %q", line1.Data, line2.Data)
	}
}

func TestScrollMatchToSelectionScrollsBackToTop(t *testing.T) {
	withFreshWindowState(t)

	primary := WindowCreate()
	if primary == nil {
		t.Fatal("WindowCreate failed")
	}
	primary.Buffer = buffer.Create()
	Active.CurrentWindow = primary

	var text []byte
	for i := 0; i < 40; i++ {
		if i == 30 {
			text = append(text, '>', ' ')
		} else {
			text = append(text, ' ', ' ')
		}
		text = append(text, byte('a'+i%26), '\n')
	}
	SetMatchBufferText(text, 30)
	mw := MatchWindow()
	if mw == nil {
		t.Fatal("expected match window")
	}
	if mw.Cursor.Line != 31 {
		t.Fatalf("Cursor.Line = %d, want 31", mw.Cursor.Line)
	}
	if mw.TopLine <= 1 {
		t.Fatalf("TopLine = %d, want scrolled down for selection 30", mw.TopLine)
	}
	if mw.Cursor.Line < mw.TopLine || mw.Cursor.Line >= mw.TopLine+mw.Height {
		t.Fatalf("cursor line %d not visible in [%d, %d)", mw.Cursor.Line, mw.TopLine, mw.TopLine+mw.Height)
	}
	mw.ShouldRedraw = false

	ScrollMatchToSelection(0)
	if mw.Cursor.Line != 1 {
		t.Fatalf("Cursor.Line = %d, want 1", mw.Cursor.Line)
	}
	if mw.TopLine != 1 {
		t.Fatalf("TopLine = %d, want 1 after selecting first item", mw.TopLine)
	}
	if !mw.ShouldRedraw {
		t.Fatal("scroll back to top should mark ShouldRedraw")
	}
}

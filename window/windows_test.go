package window

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestAdjustWindowLocationsMovesCursorAndTopLine(t *testing.T) {
	buf := buffer.New()
	buf.DiscardLines()
	buf.AppendLineBytes([]byte("aaa"))
	buf.AppendLineBytes([]byte("bbb"))
	win := &Window{
		Buffer:  buf,
		TopLine: 2,
		Cursor:  buffer.MakeLocation(2, 1),
		Mark:    buffer.MakeLocation(2, 2),
	}
	begin := buffer.MakeLocation(1, 0)
	end := buffer.MakeLocation(1, 0)
	newEnd := buffer.MakeLocation(2, 0) // inserted one line before

	AdjustWindowLocations([]*Window{win}, buf, begin, end, newEnd)

	if win.Cursor.Line != 3 || win.Mark.Line != 3 {
		t.Fatalf("cursor/mark = (%d,%d)/(%d,%d), want lines 3", win.Cursor.Line, win.Cursor.Offset, win.Mark.Line, win.Mark.Offset)
	}
	if win.TopLine != 3 {
		t.Fatalf("TopLine = %d, want 3", win.TopLine)
	}
}

func TestNoteBufferEditOnWindowsFirstChange(t *testing.T) {
	buf := buffer.New()
	buf.DiscardLines()
	buf.AppendLineBytes([]byte("x"))
	win := &Window{Buffer: buf}

	NoteBufferEditOnWindows([]*Window{win}, buf, false)
	if !win.DidEdit {
		t.Fatal("expected DidEdit for non-structural single window")
	}
	if !win.ShouldUpdateModeLine {
		t.Fatal("expected modeline update on first change")
	}
	if win.ShouldRedraw {
		t.Fatal("should not full-redraw non-structural single window")
	}

	buf.IsChanged = true
	wp2 := &Window{Buffer: buf}
	NoteBufferEditOnWindows([]*Window{win, wp2}, buf, false)
	if !win.ShouldRedraw || !wp2.ShouldRedraw {
		t.Fatal("multi-window edit should force redraw")
	}
}

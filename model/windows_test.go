package model

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestAdjustWindowLocationsMovesCursorAndTopLine(t *testing.T) {
	bp := buffer.New()
	bp.AppendLineBytes([]byte("aaa"))
	bp.AppendLineBytes([]byte("bbb"))
	wp := &Window{
		Buffer:  bp,
		TopLine: 2,
		Cursor:  buffer.MakeLocation(2, 1),
		Mark:    buffer.MakeLocation(2, 2),
	}
	begin := buffer.MakeLocation(1, 0)
	end := buffer.MakeLocation(1, 0)
	newEnd := buffer.MakeLocation(2, 0) // inserted one line before

	AdjustWindowLocations([]*Window{wp}, bp, begin, end, newEnd)

	if wp.Cursor.Line != 3 || wp.Mark.Line != 3 {
		t.Fatalf("cursor/mark = (%d,%d)/(%d,%d), want lines 3", wp.Cursor.Line, wp.Cursor.Offset, wp.Mark.Line, wp.Mark.Offset)
	}
	if wp.TopLine != 3 {
		t.Fatalf("TopLine = %d, want 3", wp.TopLine)
	}
}

func TestNoteBufferEditOnWindowsFirstChange(t *testing.T) {
	bp := buffer.New()
	bp.AppendLineBytes([]byte("x"))
	wp := &Window{Buffer: bp}

	NoteBufferEditOnWindows([]*Window{wp}, bp, false)
	if !wp.DidEdit {
		t.Fatal("expected DidEdit for non-structural single window")
	}
	if !wp.ShouldUpdateModeLine {
		t.Fatal("expected modeline update on first change")
	}
	if wp.ShouldRedraw {
		t.Fatal("should not full-redraw non-structural single window")
	}

	bp.IsChanged = true
	wp2 := &Window{Buffer: bp}
	NoteBufferEditOnWindows([]*Window{wp, wp2}, bp, false)
	if !wp.ShouldRedraw || !wp2.ShouldRedraw {
		t.Fatal("multi-window edit should force redraw")
	}
}

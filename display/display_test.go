package display

import (
	"github.com/jdpalmer/jem/window"
	"testing"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/term"
)

func TestTabWidth(t *testing.T) {
	t.Run("lineColAtOffset", func(t *testing.T) {
		line := &buffer.Line{Data: []byte("\t")}
		if got := lineColAtOffset(line, 1); got != 8 {
			t.Fatalf("tab at col 0: got %d, want 8", got)
		}

		line = &buffer.Line{Data: []byte("hello\t")}
		if got := lineColAtOffset(line, 5); got != 5 {
			t.Fatalf("before tab: got %d, want 5", got)
		}
		if got := lineColAtOffset(line, 6); got != 8 {
			t.Fatalf("after tab at col 5: got %d, want 8", got)
		}
	})

	t.Run("lineOffsetAtCol", func(t *testing.T) {
		line := &buffer.Line{Data: []byte("\t")}
		if got := lineOffsetAtCol(line, 8); got != 1 {
			t.Fatalf("offset at col 8: got %d, want 1", got)
		}
		if got := lineOffsetAtCol(line, 7); got != 0 {
			t.Fatalf("offset at col 7: got %d, want 0", got)
		}
	})

	t.Run("screenPutGlyph", func(t *testing.T) {
		backScreen.Rows = make([]ScreenRow, 1)
		backScreen.Rows[0].Text = make([]rune, 80)
		backScreen.Rows[0].Style = make([]buffer.TextStyle, 80)
		swCursorRow = 0
		swCursorCol = 0
		tabOriginCol = 0
		clipLeftCol = 0
		term.SetSize(24, 80)

		screenPutGlyph('\t')
		if swCursorCol != 8 {
			t.Fatalf("tab at col 0 rendered %d cols, want 8", swCursorCol)
		}

		swCursorCol = 8
		screenPutGlyph('\t')
		if swCursorCol != 16 {
			t.Fatalf("tab at col 8 rendered to %d, want 16", swCursorCol)
		}
	})

	t.Run("lineMeasureAdvance", func(t *testing.T) {
		cases := []struct {
			col  int
			want int
		}{
			{0, 8},
			{5, 8},
			{8, 16},
		}
		for _, tc := range cases {
			if got := lineMeasureAdvance(tc.col, '\t'); got != tc.want {
				t.Fatalf("col %d: got %d, want %d", tc.col, got, tc.want)
			}
		}
	})
}

func TestLineMeasureAdvanceWideRune(t *testing.T) {
	if got := lineMeasureAdvance(0, '世'); got != 2 {
		t.Fatalf("wide rune at col 0: got %d, want 2", got)
	}
	if got := lineMeasureAdvance(1, '世'); got != 3 {
		t.Fatalf("wide rune at col 1: got %d, want 3", got)
	}
}

func TestLineColAtOffsetWideRune(t *testing.T) {
	line := &buffer.Line{Data: []byte("a世b")}
	if got := lineColAtOffset(line, 1); got != 1 {
		t.Fatalf("before wide rune: got %d, want 1", got)
	}
	if got := lineColAtOffset(line, 4); got != 3 {
		t.Fatalf("after wide rune: got %d, want 3", got)
	}
}

func TestDisplayUpdateRestoresEditorCursorAfterMessage(t *testing.T) {
	DisplayInit()
	Reset()
	buf := buffer.Create()
	if buf == nil {
		t.Fatal("buffer create failed")
	}
	win := window.WindowCreate()
	if win == nil {
		t.Fatal("window create failed")
	}
	win.Buffer = buf
	window.WindowSelect(win)
	win.Cursor = buffer.Location{Line: 1, Offset: 3}
	win.TopLine = 1
	window.WindowRetile()

	MBWrite("[region copied]")
	if Active.Cursor.Row != uint32(term.Rows()) {
		t.Fatalf("MBWrite cursor row = %d, want message row %d", Active.Cursor.Row, term.Rows())
	}

	DisplayUpdate()
	if Active.Cursor.Row == uint32(term.Rows()) {
		t.Fatal("DisplayUpdate should move cursor back to the editor")
	}
	if !Active.MessagePresent {
		t.Fatal("message text should remain visible until the next key")
	}
}

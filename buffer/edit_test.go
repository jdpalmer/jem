package buffer

import "testing"

func linesOf(buf *Buffer) []string {
	out := make([]string, 0, len(buf.Lines))
	for i := 1; i <= len(buf.Lines); i++ {
		out = append(out, string(buf.Line(i).Data))
	}
	return out
}

func TestReplaceRaw_SingleLineInsert(t *testing.T) {
	buf := withLines("abcdef")

	loc := MakeLocation(1, 3)
	var newEnd Location
	if err := buf.ReplaceRaw(loc, loc, []byte("X"), &newEnd); err != nil {
		t.Fatal("ReplaceRaw failed")
	}
	lines := linesOf(buf)
	if len(lines) != 1 || lines[0] != "abcXdef" {
		t.Fatalf("unexpected content: %q", lines)
	}
	if newEnd.Line != 1 || newEnd.Offset != 4 {
		t.Fatalf("unexpected newEnd: %+v", newEnd)
	}
}

func TestReplaceRaw_MultiLineInsert(t *testing.T) {
	buf := withLines("abcdef")

	loc := MakeLocation(1, 3)
	var newEnd Location
	if err := buf.ReplaceRaw(loc, loc, []byte("X\nY"), &newEnd); err != nil {
		t.Fatal("ReplaceRaw failed")
	}
	lines := linesOf(buf)
	if len(lines) != 2 || lines[0] != "abcX" || lines[1] != "Ydef" {
		t.Fatalf("unexpected content: %q", lines)
	}
}

func TestReplaceRaw_DeleteToEOF(t *testing.T) {
	buf := withLines("hello")

	if err := buf.ReplaceRaw(MakeLocation(1, 3), MakeLocation(buf.EOF(), 0), nil, nil); err != nil {
		t.Fatal("ReplaceRaw failed")
	}
	lines := linesOf(buf)
	if len(lines) != 1 || lines[0] != "hel" {
		t.Fatalf("unexpected content: %q", lines)
	}
}

func TestReplaceRaw_InsertAtEOF(t *testing.T) {
	buf := withLines("hello")

	eof := MakeLocation(buf.EOF(), 0)
	var newEnd Location
	if err := buf.ReplaceRaw(eof, eof, []byte("world"), &newEnd); err != nil {
		t.Fatal("ReplaceRaw failed")
	}
	lines := linesOf(buf)
	if len(lines) != 2 || lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("unexpected content: %q", lines)
	}
	if newEnd.Line != 2 || newEnd.Offset != 5 {
		t.Fatalf("unexpected newEnd: %+v", newEnd)
	}
}

func TestReplaceRaw_NoOp(t *testing.T) {
	buf := withLines("hello")

	var newEnd Location
	if err := buf.ReplaceRaw(MakeLocation(1, 2), MakeLocation(1, 2), nil, &newEnd); err != nil {
		t.Fatal("ReplaceRaw failed")
	}
	if newEnd.Line != 1 || newEnd.Offset != 2 {
		t.Fatalf("unexpected newEnd: %+v", newEnd)
	}
	if linesOf(buf)[0] != "hello" {
		t.Fatalf("content changed: %q", linesOf(buf))
	}
}

func TestReplaceRaw_InsertSlicePrefix(t *testing.T) {
	buf := withLines("abcdef")

	if err := buf.ReplaceRaw(MakeLocation(1, 3), MakeLocation(1, 3), []byte("XYZZ")[:1], nil); err != nil {
		t.Fatal("ReplaceRaw failed")
	}
	if linesOf(buf)[0] != "abcXdef" {
		t.Fatalf("unexpected content: %q", linesOf(buf))
	}
}

func TestSetTextUndoDelete(t *testing.T) {
	buf := withLines("hello world")
	var undo UndoHistory
	undo.BeginCommand(buf, MakeLocation(1, 11))
	if err := buf.SetText(&undo, MakeLocation(1, 5), MakeLocation(1, 11), nil, nil); err != nil {
		t.Fatal("SetText failed")
	}
	undo.EndCommand()
	if linesOf(buf)[0] != "hello" {
		t.Fatalf("after delete: %q", linesOf(buf))
	}
	if err := undo.Undo(testUndoReplay(buf)); err != nil {
		t.Fatalf("undo failed: %v", err)
	}
	if linesOf(buf)[0] != "hello world" {
		t.Fatalf("after undo: %q", linesOf(buf))
	}
}

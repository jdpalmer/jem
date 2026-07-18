package buffer

import "testing"

func linesOf(bp *Buffer) []string {
	out := make([]string, 0, int(bp.LineCount))
	for i := uint(1); i <= bp.LineCount; i++ {
		lp := bp.Line(i)
		if lp == nil {
			out = append(out, "")
			continue
		}
		out = append(out, string(lp.Data))
	}
	return out
}

func TestReplaceRaw_SingleLineInsert(t *testing.T) {
	bp := New()
	bp.AppendLineBytes([]byte("abcdef"))

	begin := MakeLocation(1, 3)
	end := MakeLocation(1, 3)
	var newEnd Location
	ok := bp.ReplaceRaw(begin, end, []byte("X"), &newEnd)
	if !ok {
		t.Fatal("ReplaceRaw failed")
	}
	lines := linesOf(bp)
	if len(lines) != 1 || lines[0] != "abcXdef" {
		t.Fatalf("unexpected content: %q", lines)
	}
	if newEnd.Line != 1 || newEnd.Offset != 4 {
		t.Fatalf("unexpected newEnd: %+v", newEnd)
	}
}

func TestReplaceRaw_MultiLineInsert(t *testing.T) {
	bp := New()
	bp.AppendLineBytes([]byte("abcdef"))

	begin := MakeLocation(1, 3)
	end := MakeLocation(1, 3)
	var newEnd Location
	ok := bp.ReplaceRaw(begin, end, []byte("X\nY"), &newEnd)
	if !ok {
		t.Fatal("ReplaceRaw failed")
	}
	lines := linesOf(bp)
	if len(lines) != 2 || lines[0] != "abcX" || lines[1] != "Ydef" {
		t.Fatalf("unexpected content: %q", lines)
	}
}

func TestReplaceRaw_DeleteToEOF(t *testing.T) {
	bp := New()
	bp.AppendLineBytes([]byte("hello"))

	ok := bp.ReplaceRaw(MakeLocation(1, 3), MakeLocation(bp.EOF(), 0), nil, nil)
	if !ok {
		t.Fatal("ReplaceRaw failed")
	}
	lines := linesOf(bp)
	if len(lines) != 1 || lines[0] != "hel" {
		t.Fatalf("unexpected content: %q", lines)
	}
}

func TestReplaceRaw_InsertAtEOF(t *testing.T) {
	bp := New()
	bp.AppendLineBytes([]byte("hello"))

	var newEnd Location
	ok := bp.ReplaceRaw(MakeLocation(bp.EOF(), 0), MakeLocation(bp.EOF(), 0), []byte("world"), &newEnd)
	if !ok {
		t.Fatal("ReplaceRaw failed")
	}
	lines := linesOf(bp)
	if len(lines) != 2 || lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("unexpected content: %q", lines)
	}
	if newEnd.Line != 2 || newEnd.Offset != 5 {
		t.Fatalf("unexpected newEnd: %+v", newEnd)
	}
}

func TestReplaceRaw_NoOp(t *testing.T) {
	bp := New()
	bp.AppendLineBytes([]byte("hello"))

	var newEnd Location
	ok := bp.ReplaceRaw(MakeLocation(1, 2), MakeLocation(1, 2), nil, &newEnd)
	if !ok {
		t.Fatal("ReplaceRaw failed")
	}
	if newEnd.Line != 1 || newEnd.Offset != 2 {
		t.Fatalf("unexpected newEnd: %+v", newEnd)
	}
	if linesOf(bp)[0] != "hello" {
		t.Fatalf("content changed: %q", linesOf(bp))
	}
}

func TestReplaceRaw_InsertSlicePrefix(t *testing.T) {
	bp := New()
	bp.AppendLineBytes([]byte("abcdef"))

	ok := bp.ReplaceRaw(MakeLocation(1, 3), MakeLocation(1, 3), []byte("XYZZ")[:1], nil)
	if !ok {
		t.Fatal("ReplaceRaw failed")
	}
	if linesOf(bp)[0] != "abcXdef" {
		t.Fatalf("unexpected content: %q", linesOf(bp))
	}
}

func TestSetTextUndoDelete(t *testing.T) {
	bp := New()
	bp.AppendLineBytes([]byte("hello world"))
	var undo UndoHistory
	undo.BeginCommand(bp, MakeLocation(1, 11))
	ok := bp.SetText(&undo, MakeLocation(1, 5), MakeLocation(1, 11), nil, nil)
	undo.EndCommand()
	if !ok {
		t.Fatal("SetText failed")
	}
	if string(bp.Line(1).Data) != "hello" {
		t.Fatalf("after delete: %q", bp.Line(1).Data)
	}
	replay := UndoReplay{
		InsertText: func(lineNumber, offset uint, text []byte) bool {
			loc := MakeLocation(lineNumber, offset)
			return bp.ReplaceRaw(loc, loc, text, nil)
		},
		DeleteText: func(lineNumber, offset uint, text []byte) bool {
			begin := MakeLocation(lineNumber, offset)
			endLine, endOffset := lineNumber, offset
			for i := 0; i < len(text); i++ {
				if text[i] == '\n' {
					endLine++
					endOffset = 0
				} else {
					endOffset++
				}
			}
			return bp.ReplaceRaw(begin, MakeLocation(endLine, endOffset), nil, nil)
		},
	}
	if !undo.Undo(replay) {
		t.Fatal("undo failed")
	}
	if string(bp.Line(1).Data) != "hello world" {
		t.Fatalf("after undo: %q", bp.Line(1).Data)
	}
}

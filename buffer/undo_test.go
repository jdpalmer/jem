package buffer

import "testing"

func testUndoReplay(bp *Buffer) UndoReplay {
	return UndoReplay{
		InsertText: func(lineNumber, offset uint, text []byte, length uint) bool {
			loc := MakeLocation(lineNumber, offset)
			return ReplaceRaw(bp, loc, loc, text, length, nil)
		},
		DeleteText: func(lineNumber, offset uint, text []byte, length uint) bool {
			begin := MakeLocation(lineNumber, offset)
			endLine, endOffset := lineNumber, offset
			for i := uint(0); i < length; i++ {
				if text[i] == '\n' {
					endLine++
					endOffset = 0
				} else {
					endOffset++
				}
			}
			return ReplaceRaw(bp, begin, MakeLocation(endLine, endOffset), nil, 0, nil)
		},
	}
}

func TestUndoMultiEditGroup(t *testing.T) {
	bp := New()
	AppendLineBytes(bp, []byte("abcdef"), 6)

	var undo UndoHistory
	undo.BeginCommand(bp, MakeLocation(1, 0))
	if !SetText(bp, &undo, MakeLocation(1, 3), MakeLocation(1, 3), []byte("X"), 1, nil) {
		t.Fatal("first SetText failed")
	}
	if !SetText(bp, &undo, MakeLocation(1, 4), MakeLocation(1, 4), []byte("Y"), 1, nil) {
		t.Fatal("second SetText failed")
	}
	undo.EndCommand()

	if string(GetLine(bp, 1).Data) != "abcXYdef" {
		t.Fatalf("after edits: %q", GetLine(bp, 1).Data)
	}
	if undo.Pending.Count != 0 {
		t.Fatalf("pending count = %d, want 0", undo.Pending.Count)
	}
	if undo.Groups[0].Count != 2 {
		t.Fatalf("group record count = %d, want 2", undo.Groups[0].Count)
	}

	if !undo.Undo(testUndoReplay(bp)) {
		t.Fatal("undo failed")
	}
	if string(GetLine(bp, 1).Data) != "abcdef" {
		t.Fatalf("after undo: %q", GetLine(bp, 1).Data)
	}
}

func TestForgetBuffer(t *testing.T) {
	bp1 := New()
	AppendLineBytes(bp1, []byte("one"), 3)
	bp2 := New()
	AppendLineBytes(bp2, []byte("two"), 3)

	var undo UndoHistory
	undo.BeginCommand(bp1, MakeLocation(1, 0))
	if !SetText(bp1, &undo, MakeLocation(1, 0), MakeLocation(1, 0), []byte("A"), 1, nil) {
		t.Fatal("SetText bp1 failed")
	}
	undo.EndCommand()

	undo.BeginCommand(bp2, MakeLocation(1, 0))
	if !SetText(bp2, &undo, MakeLocation(1, 0), MakeLocation(1, 0), []byte("B"), 1, nil) {
		t.Fatal("SetText bp2 failed")
	}
	undo.EndCommand()

	if undo.Count != 2 {
		t.Fatalf("count = %d, want 2", undo.Count)
	}

	undo.ForgetBuffer(bp1)
	if undo.Count != 1 {
		t.Fatalf("count after forget = %d, want 1", undo.Count)
	}
	if undo.Groups[0].Buffer != bp2 {
		t.Fatal("expected remaining group for bp2")
	}
	if undo.Groups[1].Buffer != nil {
		t.Fatal("stale group slot should be cleared")
	}
}

func TestNoteBufferSavedOnRestoredSave(t *testing.T) {
	bp := New()
	AppendLineBytes(bp, []byte("hello"), 5)

	var undo UndoHistory
	undo.BeginCommand(bp, MakeLocation(1, 5))
	if !SetText(bp, &undo, MakeLocation(1, 5), MakeLocation(1, 5), []byte("!"), 1, nil) {
		t.Fatal("SetText failed")
	}
	undo.EndCommand()
	undo.NoteBufferSaved(bp)

	undo.BeginCommand(bp, MakeLocation(1, 6))
	if !SetText(bp, &undo, MakeLocation(1, 6), MakeLocation(1, 6), []byte("?"), 1, nil) {
		t.Fatal("SetText failed")
	}
	undo.EndCommand()

	restored := false
	replay := testUndoReplay(bp)
	replay.OnRestoredSave = func(b *Buffer) {
		if b != bp {
			t.Fatal("unexpected buffer in OnRestoredSave")
		}
		restored = true
	}

	if !undo.Undo(replay) {
		t.Fatal("undo failed")
	}
	if !restored {
		t.Fatal("OnRestoredSave should run when undo reaches saved state")
	}
	if string(GetLine(bp, 1).Data) != "hello!" {
		t.Fatalf("after undo: %q", GetLine(bp, 1).Data)
	}
}

func TestUndoStaleSerial(t *testing.T) {
	bp := New()
	AppendLineBytes(bp, []byte("abc"), 3)

	var undo UndoHistory
	undo.BeginCommand(bp, MakeLocation(1, 0))
	if !SetText(bp, &undo, MakeLocation(1, 3), MakeLocation(1, 3), []byte("X"), 1, nil) {
		t.Fatal("SetText failed")
	}
	undo.EndCommand()

	bp.Serial++
	if undo.Undo(testUndoReplay(bp)) {
		t.Fatal("undo with stale serial should fail")
	}
	if string(GetLine(bp, 1).Data) != "abcX" {
		t.Fatalf("buffer should be unchanged after stale undo: %q", GetLine(bp, 1).Data)
	}
}

func TestRecordEditSkipsIdentityReplace(t *testing.T) {
	bp := New()
	AppendLineBytes(bp, []byte("hello"), 5)

	var undo UndoHistory
	undo.BeginCommand(bp, MakeLocation(1, 0))
	if !SetText(bp, &undo, MakeLocation(1, 1), MakeLocation(1, 4), []byte("ell"), 3, nil) {
		t.Fatal("identity SetText failed")
	}
	undo.EndCommand()

	if undo.Count != 0 {
		t.Fatalf("identity replace should not create undo group, count = %d", undo.Count)
	}
}

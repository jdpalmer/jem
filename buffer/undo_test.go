package buffer

import (
	"bytes"
	"testing"
)

func testUndoReplay(buf *Buffer) UndoReplay {
	return UndoReplay{
		InsertText: func(lineNumber, offset int, text []byte) error {
			loc := MakeLocation(lineNumber, offset)
			_, err := buf.ReplaceRaw(loc, loc, text, nil)
			return err
		},
		DeleteText: func(lineNumber, offset int, text []byte) error {
			begin := MakeLocation(lineNumber, offset)
			nls := bytes.Count(text, []byte{'\n'})
			endLine := lineNumber + nls
			endOffset := offset + len(text)
			if nls > 0 {
				endOffset = len(text) - bytes.LastIndexByte(text, '\n') - 1
			}
			_, err := buf.ReplaceRaw(begin, MakeLocation(endLine, endOffset), nil, nil)
			return err
		},
	}
}

func TestUndoMultiEditGroup(t *testing.T) {
	buf := withLines("abcdef")

	var undo UndoHistory
	BindHistory(&undo)
	defer BindHistory(nil)
	undo.BeginCommand(buf, MakeLocation(1, 0))
	if err := buf.SetText(MakeLocation(1, 3), MakeLocation(1, 3), []byte("X"), nil); err != nil {
		t.Fatal("first SetText failed")
	}
	if err := buf.SetText(MakeLocation(1, 4), MakeLocation(1, 4), []byte("Y"), nil); err != nil {
		t.Fatal("second SetText failed")
	}
	undo.EndCommand()

	if string(buf.Line(1).Data) != "abcXYdef" {
		t.Fatalf("after edits: %q", buf.Line(1).Data)
	}
	if undo.Pending.Count != 0 {
		t.Fatalf("pending count = %d, want 0", undo.Pending.Count)
	}
	if undo.Groups[0].Count != 2 {
		t.Fatalf("group record count = %d, want 2", undo.Groups[0].Count)
	}

	if err := undo.Undo(testUndoReplay(buf)); err != nil {
		t.Fatalf("undo failed: %v", err)
	}
	if string(buf.Line(1).Data) != "abcdef" {
		t.Fatalf("after undo: %q", buf.Line(1).Data)
	}
}

func TestForgetBuffer(t *testing.T) {
	bp1 := withLines("one")
	bp2 := withLines("two")

	var undo UndoHistory
	BindHistory(&undo)
	defer BindHistory(nil)
	undo.BeginCommand(bp1, MakeLocation(1, 0))
	if err := bp1.SetText(MakeLocation(1, 0), MakeLocation(1, 0), []byte("A"), nil); err != nil {
		t.Fatal("SetText bp1 failed")
	}
	undo.EndCommand()

	undo.BeginCommand(bp2, MakeLocation(1, 0))
	if err := bp2.SetText(MakeLocation(1, 0), MakeLocation(1, 0), []byte("B"), nil); err != nil {
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
	buf := withLines("hello")

	var undo UndoHistory
	BindHistory(&undo)
	defer BindHistory(nil)
	undo.BeginCommand(buf, MakeLocation(1, 5))
	if err := buf.SetText(MakeLocation(1, 5), MakeLocation(1, 5), []byte("!"), nil); err != nil {
		t.Fatal("SetText failed")
	}
	undo.EndCommand()
	undo.NoteBufferSaved(buf)

	undo.BeginCommand(buf, MakeLocation(1, 6))
	if err := buf.SetText(MakeLocation(1, 6), MakeLocation(1, 6), []byte("?"), nil); err != nil {
		t.Fatal("SetText failed")
	}
	undo.EndCommand()

	restored := false
	replay := testUndoReplay(buf)
	replay.OnRestoredSave = func(b *Buffer) {
		if b != buf {
			t.Fatal("unexpected buffer in OnRestoredSave")
		}
		restored = true
	}

	if err := undo.Undo(replay); err != nil {
		t.Fatalf("undo failed: %v", err)
	}
	if !restored {
		t.Fatal("OnRestoredSave should run when undo reaches saved state")
	}
	if string(buf.Line(1).Data) != "hello!" {
		t.Fatalf("after undo: %q", buf.Line(1).Data)
	}
}

func TestUndoStaleSerial(t *testing.T) {
	buf := withLines("abc")

	var undo UndoHistory
	BindHistory(&undo)
	defer BindHistory(nil)
	undo.BeginCommand(buf, MakeLocation(1, 0))
	if err := buf.SetText(MakeLocation(1, 3), MakeLocation(1, 3), []byte("X"), nil); err != nil {
		t.Fatal("SetText failed")
	}
	undo.EndCommand()

	buf.Serial++
	if err := undo.Undo(testUndoReplay(buf)); err != ErrUndoStale {
		t.Fatalf("undo with stale serial: got %v, want ErrUndoStale", err)
	}
	if string(buf.Line(1).Data) != "abcX" {
		t.Fatalf("buffer should be unchanged after stale undo: %q", buf.Line(1).Data)
	}
}

func TestRecordEditSkipsIdentityReplace(t *testing.T) {
	buf := withLines("hello")

	var undo UndoHistory
	BindHistory(&undo)
	defer BindHistory(nil)
	undo.BeginCommand(buf, MakeLocation(1, 0))
	if err := buf.SetText(MakeLocation(1, 1), MakeLocation(1, 4), []byte("ell"), nil); err != nil {
		t.Fatal("identity SetText failed")
	}
	undo.EndCommand()

	if undo.Count != 0 {
		t.Fatalf("identity replace should not create undo group, count = %d", undo.Count)
	}
}

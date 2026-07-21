package buffer

import "testing"

func TestNoteEditSetsChangedAndCallsHook(t *testing.T) {
	buf := New()
	if buf.IsChanged {
		t.Fatal("new buffer should not be changed")
	}

	var called bool
	var structural bool
	var sawUnchanged bool
	old := PackageHooks
	PackageHooks = Hooks{
		NoteEdit: func(b *Buffer, isStructural bool) {
			called = true
			structural = isStructural
			sawUnchanged = !b.IsChanged
			if b != buf {
				t.Fatal("unexpected buffer in NoteEdit")
			}
		},
	}
	defer func() { PackageHooks = old }()

	buf.NoteEdit(true)

	if !buf.IsChanged {
		t.Fatal("NoteEdit should set IsChanged")
	}
	if !called || !structural {
		t.Fatalf("hook not called correctly: called=%v structural=%v", called, structural)
	}
	if !sawUnchanged {
		t.Fatal("NoteEdit hook should run before IsChanged is set")
	}
}

func TestInvalidateSyntaxFromLine(t *testing.T) {
	buf := New()
	buf.AppendLineBytes([]byte("a"))
	buf.AppendLineBytes([]byte("b"))
	buf.AppendLineBytes([]byte("c"))
	for i := uint(1); i <= buf.LineCount; i++ {
		if line := buf.Line(i); line != nil {
			line.SyntaxValid = true
		}
	}

	buf.InvalidateSyntaxFrom(2)
	if buf.Line(1).SyntaxValid != true {
		t.Fatal("line 1 should stay valid")
	}
	if buf.Line(2).SyntaxValid != false || buf.Line(3).SyntaxValid != false {
		t.Fatal("lines 2-3 should be invalidated")
	}
}

func TestReplaceRawUsesPackageHooks(t *testing.T) {
	buf := New()
	buf.AppendLineBytes([]byte("hello"))

	var adjusted bool
	var reparsed uint
	old := PackageHooks
	PackageHooks = Hooks{
		AdjustLocationsAfterReplace: func(b *Buffer, begin, end, newEnd Location) {
			adjusted = true
			if b != buf || begin.Line != 1 || newEnd.Offset != 6 {
				t.Fatalf("unexpected adjust args: begin=%v newEnd=%v", begin, newEnd)
			}
		},
		ReparseFrom: func(b *Buffer, lineNumber uint) {
			reparsed = lineNumber
		},
	}
	defer func() { PackageHooks = old }()

	if err := buf.ReplaceRaw(MakeLocation(1, 5), MakeLocation(1, 5), []byte("!"), nil); err != nil {
		t.Fatal(err)
	}

	if !adjusted {
		t.Fatal("expected AdjustLocationsAfterReplace")
	}
	if reparsed != 1 {
		t.Fatalf("reparse line = %d, want 1", reparsed)
	}
	if buf.Line(1).SyntaxValid {
		t.Fatal("syntax should be invalidated locally")
	}
}

package buffer

import "testing"

func TestNoteEditSetsChangedAndCallsHook(t *testing.T) {
	bp := New()
	if bp.IsChanged {
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
			if b != bp {
				t.Fatal("unexpected buffer in NoteEdit")
			}
		},
	}
	defer func() { PackageHooks = old }()

	bp.NoteEdit(true)

	if !bp.IsChanged {
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
	bp := New()
	bp.AppendLineBytes([]byte("a"))
	bp.AppendLineBytes([]byte("b"))
	bp.AppendLineBytes([]byte("c"))
	for i := uint(1); i <= bp.LineCount; i++ {
		if lp := bp.Line(i); lp != nil {
			lp.SyntaxValid = true
		}
	}

	bp.InvalidateSyntaxFrom(2)
	if bp.Line(1).SyntaxValid != true {
		t.Fatal("line 1 should stay valid")
	}
	if bp.Line(2).SyntaxValid != false || bp.Line(3).SyntaxValid != false {
		t.Fatal("lines 2-3 should be invalidated")
	}
}

func TestReplaceRawUsesPackageHooks(t *testing.T) {
	bp := New()
	bp.AppendLineBytes([]byte("hello"))

	var adjusted bool
	var reparsed uint
	old := PackageHooks
	PackageHooks = Hooks{
		AdjustLocationsAfterReplace: func(b *Buffer, begin, end, newEnd Location) {
			adjusted = true
			if b != bp || begin.Line != 1 || newEnd.Offset != 6 {
				t.Fatalf("unexpected adjust args: begin=%v newEnd=%v", begin, newEnd)
			}
		},
		ReparseFrom: func(b *Buffer, lineNumber uint) {
			reparsed = lineNumber
		},
	}
	defer func() { PackageHooks = old }()

	if err := bp.ReplaceRaw(MakeLocation(1, 5), MakeLocation(1, 5), []byte("!"), nil); err != nil {
		t.Fatal(err)
	}

	if !adjusted {
		t.Fatal("expected AdjustLocationsAfterReplace")
	}
	if reparsed != 1 {
		t.Fatalf("reparse line = %d, want 1", reparsed)
	}
	if bp.Line(1).SyntaxValid {
		t.Fatal("syntax should be invalidated locally")
	}
}

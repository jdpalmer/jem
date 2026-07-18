package buffer

import "testing"

func TestNoteEditSetsChangedAndCallsHook(t *testing.T) {
	bp := New()
	if bp.IsChanged {
		t.Fatal("new buffer should not be changed")
	}

	var called bool
	var structural bool
	old := PackageHooks
	PackageHooks = Hooks{
		NoteEdit: func(b *Buffer, isStructural bool) {
			called = true
			structural = isStructural
			if b != bp {
				t.Fatal("unexpected buffer in NoteEdit hook")
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
}

func TestInvalidateSyntaxFromLine(t *testing.T) {
	bp := New()
	bp.AppendLineBytes([]byte("a"), 1)
	bp.AppendLineBytes([]byte("b"), 1)
	bp.AppendLineBytes([]byte("c"), 1)
	for i := uint(1); i <= bp.LineCount; i++ {
		if lp := bp.Line(i); lp != nil {
			lp.SyntaxValid = true
		}
	}

	bp.InvalidateSyntaxFrom( 2)
	if bp.Line(1).SyntaxValid != true {
		t.Fatal("line 1 should stay valid")
	}
	if bp.Line(2).SyntaxValid != false || bp.Line(3).SyntaxValid != false {
		t.Fatal("lines 2-3 should be invalidated")
	}
}

func TestCallInvalidateSyntaxUsesHookWhenSet(t *testing.T) {
	bp := New()
	bp.AppendLineBytes([]byte("a"), 1)
	lp := bp.Line(1)
	lp.SyntaxValid = true

	var hookLine uint
	old := PackageHooks
	PackageHooks = Hooks{
		InvalidateSyntaxFrom: func(b *Buffer, lineNumber uint) {
			hookLine = lineNumber
			if lp := b.Line(lineNumber); lp != nil {
				lp.SyntaxValid = false
			}
		},
	}
	defer func() { PackageHooks = old }()

	callInvalidateSyntax(bp, 1)
	if hookLine != 1 {
		t.Fatalf("hook line = %d, want 1", hookLine)
	}
	if lp.SyntaxValid {
		t.Fatal("hook should invalidate syntax")
	}
}

func TestCallInvalidateSyntaxFallsBackWithoutHook(t *testing.T) {
	bp := New()
	bp.AppendLineBytes([]byte("a"), 1)
	bp.AppendLineBytes([]byte("b"), 1)
	for i := uint(1); i <= bp.LineCount; i++ {
		if lp := bp.Line(i); lp != nil {
			lp.SyntaxValid = true
		}
	}

	old := PackageHooks
	PackageHooks = Hooks{}
	defer func() { PackageHooks = old }()

	callInvalidateSyntax(bp, 1)
	if bp.Line(1).SyntaxValid != false || bp.Line(2).SyntaxValid != false {
		t.Fatal("fallback should invalidate from line 1")
	}
}

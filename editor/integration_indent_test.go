package editor

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestCNewlineIndent(t *testing.T) {
	te := NewTestEditor(t)
	te.SetLangMode(buffer.LModeC)
	te.BP().CIndent = 2

	te.LoadText("if (x) {")
	if col := te.NewlineIndent(); col != 2 {
		t.Fatalf("indent after open brace = %d, want 2", col)
	}

	te.LoadText("if (x) {\n  foo;")
	if col := te.NewlineIndent(); col != 2 {
		t.Fatalf("indent continues inside block = %d, want 2", col)
	}

	te.LoadText("if (x) {\n  foo;\n}")
	if col := te.NewlineIndent(); col != 0 {
		t.Fatalf("indent after close brace = %d, want 0", col)
	}

	te.LoadText("switch (x) {\ncase FOO:")
	if col := te.NewlineIndent(); col != 2 {
		t.Fatalf("indent after case label = %d, want 2", col)
	}

	te.LoadText("// comment")
	if col := te.NewlineIndent(); col != 0 {
		t.Fatalf("indent after top-level comment = %d, want 0", col)
	}

	te.LoadText("int x = 0;")
	if col := te.NewlineIndent(); col != 0 {
		t.Fatalf("indent after top-level statement = %d, want 0", col)
	}

	te.LoadText("if (x) {\n  if (y) {")
	if col := te.NewlineIndent(); col != 4 {
		t.Fatalf("indent for nested open brace = %d, want 4", col)
	}

	te.LoadText("  foo(a,")
	if col := te.NewlineIndent(); col != 6 {
		t.Fatalf("indent aligns after open paren = %d, want 6", col)
	}
}

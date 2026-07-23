package buffer

import "testing"

// withLines returns a buffer whose lines are exactly the given texts.
func withLines(texts ...string) *Buffer {
	buf := New()
	buf.DiscardLines()
	for _, s := range texts {
		buf.AppendLineBytes([]byte(s))
	}
	buf.EnsureMinLines()
	return buf
}

func TestNewHasOneEmptyLine(t *testing.T) {
	buf := New()
	if len(buf.Lines) != 1 {
		t.Fatalf("New lines = %d, want 1", len(buf.Lines))
	}
	if buf.Line(1) == nil || buf.Line(1).Len() != 0 {
		t.Fatal("New should provide an empty line 1")
	}
}

func TestClearRestoresOneEmptyLine(t *testing.T) {
	buf := withLines("a", "b")
	buf.Clear()
	if len(buf.Lines) != 1 || buf.Line(1).Len() != 0 {
		t.Fatalf("Clear lines = %q, want one empty line", linesOf(buf))
	}
}

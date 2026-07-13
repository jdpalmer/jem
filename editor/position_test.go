package editor

import "testing"

func TestBufferCharStats(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("ab\nc")
	te.SetCursor(2, 0)

	charAt, before, total := bufferCharStats(te.BP(), te.WP())
	if charAt != 'c' {
		t.Fatalf("charAt = %q, want c", charAt)
	}
	if before != 3 {
		t.Fatalf("before = %d, want 3 (ab + newline)", before)
	}
	if total != 4 {
		t.Fatalf("total = %d, want 4", total)
	}
}

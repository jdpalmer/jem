package editor

import "testing"

func TestUtf8CursorOffsets(t *testing.T) {
	t.Run("prev", func(t *testing.T) {
		data := []byte("a\xe2\x82\xacb") // a€b
		if got := utf8PrevOffset(data, 5); got != 4 {
			t.Fatalf("prev from end: got %d, want 4", got)
		}
		if got := utf8PrevOffset(data, 4); got != 1 {
			t.Fatalf("prev over euro: got %d, want 1", got)
		}
		if got := utf8PrevOffset(data, 1); got != 0 {
			t.Fatalf("prev over a: got %d, want 0", got)
		}
	})

	t.Run("next", func(t *testing.T) {
		data := []byte("a\xe4\xb8\x96b") // a世b
		if got := utf8NextOffset(data, 0); got != 1 {
			t.Fatalf("next from start: got %d, want 1", got)
		}
		if got := utf8NextOffset(data, 1); got != 4 {
			t.Fatalf("next over CJK: got %d, want 4", got)
		}
	})
}

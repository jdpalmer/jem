package runtime

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/killring"
)

func TestCmdEdit(t *testing.T) {
	te := NewTestEditor(t)

	te.LoadText("abcde")
	te.SetCursor(1, 2)
	te.Press("C-t")
	te.ExpectText("acbde")

	te.LoadText("hello")
	te.SetCursor(1, 0)
	te.Press("C-d")
	te.ExpectText("ello")

	te.SetCursor(1, 3)
	te.Key(0x7F)
	te.ExpectText("elo")

	te.LoadText("café")
	te.SetCursor(1, uint(len(te.BufferText())))
	te.Key(0x7F)
	te.ExpectText("caf")

	te.LoadText("hello world")
	te.SetCursor(1, 5)
	killring.ClearSequence()
	te.Press("C-k")
	te.ExpectText("hello")
	te.Press("C-y")
	te.ExpectText("hello world")
}

func TestBufferSetText(t *testing.T) {
	te := NewTestEditor(t)

	te.LoadText("hello\nworld")
	te.Edit(buffer.MakeLocation(1, 3), buffer.MakeLocation(2, 2), "")
	te.ExpectText("helrld")
	te.ExpectLineCount(1)

	te.LoadText("hello world")
	te.Edit(buffer.MakeLocation(1, 6), buffer.MakeLocation(1, 11), "a\nb")
	te.ExpectText("hello a\nb")
	te.ExpectLineCount(2)
}

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

func TestClipboardWriteOSC52Fallback(t *testing.T) {
	// OSC52 path is used when native clipboard is unavailable (SSH, headless tests).
	killring.ClipboardReady = false
	ok := killring.ClipboardWrite([]byte("hi"))
	if !ok {
		t.Fatal("clipboardWriteOSC52 failed")
	}
}

func TestMouseLeftClick(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("hello world")
	te.SetCursor(1, 0)

	gutter := te.WP().GutterWidth()
	te.Click(0, gutter+5)
	te.ExpectCursor(1, 5)
}

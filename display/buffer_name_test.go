package display

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestFitBufferName(t *testing.T) {
	if got := FitBufferName("short", 0); got != "short" {
		t.Fatalf("short name: got %q", got)
	}
	long := "abcdefghijklmnopq" // 17
	got := FitBufferName(long, BufferNameMaxCols)
	if w := runewidth.StringWidth(got); w > BufferNameMaxCols {
		t.Fatalf("fitted width %d > %d (%q)", w, BufferNameMaxCols, got)
	}
	if got == long {
		t.Fatal("expected truncation")
	}
	if !strings.ContainsRune(got, '…') {
		t.Fatalf("expected ellipsis in %q", got)
	}
}

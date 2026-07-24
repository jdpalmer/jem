package term

import (
	"bufio"
	"strings"
	"testing"

	"github.com/jdpalmer/jem/event"
)

func withTermReader(t *testing.T, input string) func() {
	t.Helper()
	prev := termReader
	termReader = bufio.NewReader(strings.NewReader(input))
	return func() { termReader = prev }
}

func TestReadKeySGRMousePressAndRelease(t *testing.T) {
	defer withTermReader(t, "\x1b[<0;11;6M\x1b[<0;11;6m\x1b[<0;21;9M")()

	k1, ok := termReadKeyImpl()
	if !ok || k1 != MouseLeft {
		t.Fatalf("first click = 0x%x ok=%v, want MouseLeft", k1, ok)
	}
	col, row := MousePos()
	if row != 5 || col != 10 {
		t.Fatalf("first click mouse = (%d,%d), want (10,5)", col, row)
	}

	k2, ok := termReadKeyImpl()
	if !ok || k2 != MouseLeft {
		t.Fatalf("second click = 0x%x ok=%v, want MouseLeft", k2, ok)
	}
	col, row = MousePos()
	if row != 8 || col != 20 {
		t.Fatalf("second click mouse = (%d,%d), want (20,8)", col, row)
	}
}

func TestReadKeyMetaPrefix(t *testing.T) {
	defer withTermReader(t, "\x1bx")()

	k1, ok := termReadKeyImpl()
	if !ok || k1 != 0x1B {
		t.Fatalf("meta prefix = 0x%x ok=%v, want ESC", k1, ok)
	}

	k2, ok := termReadKeyImpl()
	if !ok || k2 != 'x' {
		t.Fatalf("meta key = 0x%x ok=%v, want x", k2, ok)
	}
}

func TestReadKeyMouseReleaseAlone(t *testing.T) {
	defer withTermReader(t, "\x1b[<0;5;3m")()

	_, ok := termReadKeyImpl()
	if ok {
		t.Fatal("mouse release alone should not return a key")
	}
}

func TestReadKeyKittyLetter(t *testing.T) {
	defer withTermReader(t, "\x1b[97u")()

	k, ok := termReadKeyImpl()
	if !ok || k != 'a' {
		t.Fatalf("kitty a = 0x%x ok=%v, want 'a'", k, ok)
	}
}

func TestReadKeyKittyCtrlA(t *testing.T) {
	defer withTermReader(t, "\x1b[97;5u")()

	k, ok := termReadKeyImpl()
	if !ok || k != (CTL|'A') {
		t.Fatalf("kitty C-a = 0x%x ok=%v, want CTL|A", k, ok)
	}
}

func TestReadKeyKittyArrow(t *testing.T) {
	defer withTermReader(t, "\x1b[57352u")()

	k, ok := termReadKeyImpl()
	if !ok || k != KeyUp {
		t.Fatalf("kitty up = 0x%x ok=%v, want KeyUp", k, ok)
	}
}

func TestBracketedPaste(t *testing.T) {
	event.DrainForTest()
	defer withTermReader(t, "\x1b[200~hello\x1b[201~")()
	k, ok := termReadKeyImpl()
	if !ok || k != KeyPasteComplete {
		t.Fatalf("expected KeyPasteComplete after paste, got 0x%x ok=%v", k, ok)
	}
	select {
	case e := <-event.Chan():
		pe, ok := e.(event.PasteEvent)
		if !ok {
			t.Fatalf("event type %T, want PasteEvent", e)
		}
		if string(pe.Data) != "hello" {
			t.Fatalf("paste = %q, want hello", pe.Data)
		}
	default:
		t.Fatal("expected PasteEvent on bus")
	}
}

func TestReadKeyUTF8Resync(t *testing.T) {
	defer withTermReader(t, "\xC3A")()

	_, ok := termReadKeyImpl()
	if ok {
		t.Fatal("invalid utf-8 lead should not return a key")
	}
	k, ok := termReadKeyImpl()
	if !ok || k != 'A' {
		t.Fatalf("resync key = 0x%x ok=%v, want A", k, ok)
	}
}

package display

import (
	"testing"

	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/minibuffer"
)

func TestEscapeQuitsWhenMinibufferActive(t *testing.T) {
	escapePending = false
	ctlxPending = false

	var state minibuffer.MinibufferState
	minibuffer.Active = &state
	t.Cleanup(func() {
		minibuffer.Active = nil
		escapePending = false
	})

	event.DrainForTest()
	decodeAndDeliver(0x1B)
	if escapePending {
		t.Fatal("Esc with active minibuffer should not start meta prefix")
	}
	select {
	case ev := <-event.Chan():
		ke, ok := ev.(event.KeyEvent)
		if !ok {
			t.Fatalf("expected KeyEvent, got %T %#v", ev, ev)
		}
		if ke.Code != 0x1B {
			t.Fatalf("key = %#x, want Esc 0x1b", ke.Code)
		}
	default:
		t.Fatal("expected Esc key event")
	}
}

func TestEscapeStillMetaWhenNoMinibuffer(t *testing.T) {
	escapePending = false
	ctlxPending = false
	minibuffer.Active = nil
	t.Cleanup(func() { escapePending = false })

	event.DrainForTest()
	decodeAndDeliver(0x1B)
	if !escapePending {
		t.Fatal("Esc without minibuffer should start meta prefix")
	}
	select {
	case ev := <-event.Chan():
		t.Fatalf("bare Esc should not deliver a key yet, got %#v", ev)
	default:
	}
}

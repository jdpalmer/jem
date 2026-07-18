package ui

import (
	"github.com/jdpalmer/jem/app"
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestApplyPendingPasteOnMainThread(t *testing.T) {
	GlobalKeyCh = make(chan uint32, 4)
	GlobalMinibufKeyCh = make(chan uint32, 4)
	pendingPasteCh = make(chan []byte, 4)

	bp := buffer.New()
	wp := &app.Window{Buffer: bp, Cursor: buffer.Location{Line: 1, Offset: 0}}
	app.State.CurrentWindow = wp
	app.State.WINDOWS = []*app.Window{wp}

	queuePaste([]byte("hello"))
	applyPendingPaste()

	if got := string(bp.Line(1).Data); got != "hello" {
		t.Fatalf("buffer after paste = %q, want hello", got)
	}
	if !wp.ShouldRedraw {
		t.Fatal("paste should mark window for redraw")
	}
}

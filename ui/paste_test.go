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
	wp := &Window{Buffer: bp, Cursor: Location{Line: 1, Offset: 0}}
	app.State.CurrentWindow = wp
	app.State.WINDOWS[0] = wp
	app.State.WindowCount = 1

	queuePaste([]byte("hello"))
	applyPendingPaste()

	if got := string(bp.Line(1).Data); got != "hello" {
		t.Fatalf("buffer after paste = %q, want hello", got)
	}
	if !wp.ShouldRedraw {
		t.Fatal("paste should mark window for redraw")
	}
}

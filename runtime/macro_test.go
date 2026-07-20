package runtime

import (
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
	"testing"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/term"
)

func resetMacroState() {
	State.Macro = nil
	State.PlayPos = -1
	macroInit()
}

func TestMacroRecording(t *testing.T) {
	resetMacroState()

	if !CmdMacroStart(false, 1) {
		t.Fatal("macro start failed")
	}
	if !macroRecordKey(int('x'), false, 1) {
		t.Fatal("record plain key failed")
	}
	if !macroRecordKey(int('y'), true, 3) {
		t.Fatal("record prefixed key failed")
	}
	if !macroRecordKey(int(term.CTLX|'E'), false, 1) {
		t.Fatal("skip macro execute while recording failed")
	}
	if !macroRecordKey(int(term.CTLX|')'), false, 1) {
		t.Fatal("skip macro end while recording failed")
	}
	if !CmdMacroEnd(false, 1) {
		t.Fatal("macro end failed")
	}

	if len(State.Macro) != 2 {
		t.Fatalf("macro len = %d, want 2", len(State.Macro))
	}
	s0, ok := State.Macro[0].(event.MacroStepEvent)
	if !ok || s0.Code != 'x' || s0.F {
		t.Fatalf("macro[0] = %#v, want MacroStepEvent{'x'}", State.Macro[0])
	}
	s1, ok := State.Macro[1].(event.MacroStepEvent)
	if !ok || s1.Code != 'y' || !s1.F || s1.N != 3 {
		t.Fatalf("macro[1] = %#v, want MacroStepEvent{'y', F, 3}", State.Macro[1])
	}
	if State.IsRecording() {
		t.Fatal("still recording after end")
	}
}

func TestMacroPlayback(t *testing.T) {
	resetMacroState()
	AppInit("test")
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil || wp == nil {
		t.Fatal("editor init failed")
	}
	wp.Cursor = buffer.Location{Line: 1, Offset: 0}

	if !CmdMacroStart(false, 1) {
		t.Fatal("macro start failed")
	}
	if !macroRecordKey(int('a'), false, 1) {
		t.Fatal("record key failed")
	}
	if !CmdMacroEnd(false, 1) {
		t.Fatal("macro end failed")
	}

	if !CmdMacroExec(false, 1) {
		t.Fatal("macro playback failed")
	}
	line := bp.Line(1)
	if line == nil || string(line.Data) != "a" {
		t.Fatalf("after one playback got %q, want %q", string(line.Data), "a")
	}

	if !CmdMacroExec(true, 3) {
		t.Fatal("macro repeat playback failed")
	}
	line = bp.Line(1)
	if line == nil || string(line.Data) != "aaaa" {
		t.Fatalf("after repeat playback got %q, want %q", string(line.Data), "aaaa")
	}
}

func TestMacroPromptReply(t *testing.T) {
	resetMacroState()
	State.Macro = []event.Event{
		event.PromptReplyEvent{Text: "hello"},
	}
	State.PlayPos = 0

	text, pr, ok := TakeMacroPromptReply()
	if !ok || pr != minibuffer.PromptResultYes || text != "hello" {
		t.Fatalf("TakeMacroPromptReply = %q %v %v", text, pr, ok)
	}
	if State.PlayPos != 1 {
		t.Fatalf("PlayPos = %d, want 1", State.PlayPos)
	}
}

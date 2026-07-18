package editor

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"testing"
)

func resetMacroState() {
	app.State.EditorMacroState = EditorMacroState{}
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
	if !macroRecordKey(int(CTLX|'E'), false, 1) {
		t.Fatal("skip macro execute while recording failed")
	}
	if !macroRecordKey(int(CTLX|')'), false, 1) {
		t.Fatal("record macro terminator failed")
	}
	if !CmdMacroEnd(false, 1) {
		t.Fatal("macro end failed")
	}

	if app.State.Keys[0] != int32('x') {
		t.Fatalf("keys[0] = %d, want %d", app.State.Keys[0], 'x')
	}
	if app.State.Keys[1] != int32(CTL|'U') {
		t.Fatalf("keys[1] = %d, want CTL|U", app.State.Keys[1])
	}
	if app.State.Keys[2] != 3 {
		t.Fatalf("keys[2] = %d, want 3", app.State.Keys[2])
	}
	if app.State.Keys[3] != int32('y') {
		t.Fatalf("keys[3] = %d, want %d", app.State.Keys[3], 'y')
	}
	if app.State.Keys[4] != int32(CTLX|')') {
		t.Fatalf("keys[4] = %d, want CTLX|)", app.State.Keys[4])
	}
	if app.State.Keys[5] != 0 {
		t.Fatalf("keys[5] = %d, want 0", app.State.Keys[5])
	}
}

func TestMacroPlayback(t *testing.T) {
	resetMacroState()
	EditorInit("test")
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil {
		t.Fatal("editor init failed")
	}
	wp.Cursor = Location{Line: 1, Offset: 0}

	if !CmdMacroStart(false, 1) {
		t.Fatal("macro start failed")
	}
	if !macroRecordKey(int('a'), false, 1) {
		t.Fatal("record key failed")
	}
	if !macroRecordKey(int(CTLX|')'), false, 1) {
		t.Fatal("record terminator failed")
	}
	if !CmdMacroEnd(false, 1) {
		t.Fatal("macro end failed")
	}

	if !CmdMacroExec(false, 1) {
		t.Fatal("macro playback failed")
	}
	line := buffer.GetLine(bp, 1)
	if line == nil || string(line.Data) != "a" {
		t.Fatalf("after one playback got %q, want %q", string(line.Data), "a")
	}

	if !CmdMacroExec(true, 3) {
		t.Fatal("macro repeat playback failed")
	}
	line = buffer.GetLine(bp, 1)
	if line == nil || string(line.Data) != "aaaa" {
		t.Fatalf("after repeat playback got %q, want %q", string(line.Data), "aaaa")
	}
}

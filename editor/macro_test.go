package editor

import "testing"

func resetMacroState() {
	session.App.EditorMacroState = EditorMacroState{}
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

	if session.App.Keys[0] != int32('x') {
		t.Fatalf("keys[0] = %d, want %d", session.App.Keys[0], 'x')
	}
	if session.App.Keys[1] != int32(CTL|'U') {
		t.Fatalf("keys[1] = %d, want CTL|U", session.App.Keys[1])
	}
	if session.App.Keys[2] != 3 {
		t.Fatalf("keys[2] = %d, want 3", session.App.Keys[2])
	}
	if session.App.Keys[3] != int32('y') {
		t.Fatalf("keys[3] = %d, want %d", session.App.Keys[3], 'y')
	}
	if session.App.Keys[4] != int32(CTLX|')') {
		t.Fatalf("keys[4] = %d, want CTLX|)", session.App.Keys[4])
	}
	if session.App.Keys[5] != 0 {
		t.Fatalf("keys[5] = %d, want 0", session.App.Keys[5])
	}
}

func TestMacroPlayback(t *testing.T) {
	resetMacroState()
	EditorInit("test")
	bp := session.App.CurrentBuffer
	wp := session.App.CurrentWindow
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
	line := BufferGetLine(bp, 1)
	if line == nil || string(line.Data) != "a" {
		t.Fatalf("after one playback got %q, want %q", string(line.Data), "a")
	}

	if !CmdMacroExec(true, 3) {
		t.Fatal("macro repeat playback failed")
	}
	line = BufferGetLine(bp, 1)
	if line == nil || string(line.Data) != "aaaa" {
		t.Fatalf("after repeat playback got %q, want %q", string(line.Data), "aaaa")
	}
}

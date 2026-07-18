package editor

import (
	"github.com/jdpalmer/jem/buffer"
	"testing"

	"github.com/jdpalmer/jem/app"
)

func setupSexpTest(text string, lang LangMode, line, offset uint) (*Window, *Buffer) {
	app.State = App{}
	bp := app.BufferCreate(&app.State.EditorRuntimeState)
	if bp == nil {
		return nil, nil
	}
	buffer.AppendLineBytes(bp, []byte(text), uint(len(text)))
	bp.LangMode = lang
	wp := &Window{Buffer: bp, Cursor: buffer.MakeLocation(line, offset)}
	app.State.CurrentWindow = wp
	app.State.CurrentBuffer = bp
	app.State.WINDOWS[0] = wp
	app.State.WindowCount = 1
	return wp, bp
}

func TestForwardSexpPastParenGroup(t *testing.T) {
	wp, _ := setupSexpTest("(abc)", LModeC, 1, 0)
	if wp == nil {
		t.Fatal("setup failed")
	}
	if !CmdForwardSexp(false, 1) {
		t.Fatal("CmdForwardSexp failed")
	}
	if wp.Cursor.Offset != 5 {
		t.Fatalf("cursor offset = %d, want 5", wp.Cursor.Offset)
	}
}

func TestBackwardSexpToOpenParen(t *testing.T) {
	wp, _ := setupSexpTest("(abc)", LModeC, 1, 5)
	if wp == nil {
		t.Fatal("setup failed")
	}
	if !CmdBackwardSexp(false, 1) {
		t.Fatal("CmdBackwardSexp failed")
	}
	if wp.Cursor.Offset != 0 {
		t.Fatalf("cursor offset = %d, want 0", wp.Cursor.Offset)
	}
}

func TestForwardSexpFallsBackToWord(t *testing.T) {
	wp, _ := setupSexpTest("foo bar", LModeC, 1, 0)
	if wp == nil {
		t.Fatal("setup failed")
	}
	if !CmdForwardSexp(false, 1) {
		t.Fatal("CmdForwardSexp failed")
	}
	if wp.Cursor.Offset != 3 {
		t.Fatalf("cursor offset = %d, want 3", wp.Cursor.Offset)
	}
}

func TestForwardSexpRepeat(t *testing.T) {
	wp, _ := setupSexpTest("(a) (b)", LModeC, 1, 0)
	if wp == nil {
		t.Fatal("setup failed")
	}
	if !CmdForwardSexp(false, 2) {
		t.Fatal("CmdForwardSexp n=2 failed")
	}
	if wp.Cursor.Offset != 7 {
		t.Fatalf("cursor offset = %d, want 7", wp.Cursor.Offset)
	}
}

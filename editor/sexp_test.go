package editor

import (
	"testing"

	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
)

func setupSexpTest(text string, lang buffer.LangMode, line, offset uint) (*model.Window, *buffer.Buffer) {
	model.Reset()
	bp := model.BufferCreate(&model.State.EditorRuntimeState)
	if bp == nil {
		return nil, nil
	}
	bp.AppendLineBytes([]byte(text))
	bp.LangMode = lang
	wp := &model.Window{Buffer: bp, Cursor: buffer.MakeLocation(line, offset)}
	model.State.CurrentWindow = wp
	model.State.CurrentBuffer = bp
	model.State.Windows = []*model.Window{wp}
	return wp, bp
}

func TestForwardSexpPastParenGroup(t *testing.T) {
	wp, _ := setupSexpTest("(abc)", buffer.LModeC, 1, 0)
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
	wp, _ := setupSexpTest("(abc)", buffer.LModeC, 1, 5)
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
	wp, _ := setupSexpTest("foo bar", buffer.LModeC, 1, 0)
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
	wp, _ := setupSexpTest("(a) (b)", buffer.LModeC, 1, 0)
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

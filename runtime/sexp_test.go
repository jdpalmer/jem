package runtime

import (
	"github.com/jdpalmer/jem/window"
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func setupSexpTest(text string, lang buffer.LangMode, line, offset uint) (*window.Window, *buffer.Buffer) {
	Reset()
	buf := buffer.Create()
	if buf == nil {
		return nil, nil
	}
	buf.AppendLineBytes([]byte(text))
	buf.LangMode = lang
	win := &window.Window{Buffer: buf, Cursor: buffer.MakeLocation(line, offset)}
	window.Active.CurrentWindow = win
	buffer.All.Current = buf
	window.Active.Windows = []*window.Window{win}
	return win, buf
}

func TestForwardSexpPastParenGroup(t *testing.T) {
	win, _ := setupSexpTest("(abc)", buffer.LModeC, 1, 0)
	if win == nil {
		t.Fatal("setup failed")
	}
	if !CmdForwardSexp(false, 1) {
		t.Fatal("CmdForwardSexp failed")
	}
	if win.Cursor.Offset != 5 {
		t.Fatalf("cursor offset = %d, want 5", win.Cursor.Offset)
	}
}

func TestBackwardSexpToOpenParen(t *testing.T) {
	win, _ := setupSexpTest("(abc)", buffer.LModeC, 1, 5)
	if win == nil {
		t.Fatal("setup failed")
	}
	if !CmdBackwardSexp(false, 1) {
		t.Fatal("CmdBackwardSexp failed")
	}
	if win.Cursor.Offset != 0 {
		t.Fatalf("cursor offset = %d, want 0", win.Cursor.Offset)
	}
}

func TestForwardSexpFallsBackToWord(t *testing.T) {
	win, _ := setupSexpTest("foo bar", buffer.LModeC, 1, 0)
	if win == nil {
		t.Fatal("setup failed")
	}
	if !CmdForwardSexp(false, 1) {
		t.Fatal("CmdForwardSexp failed")
	}
	if win.Cursor.Offset != 3 {
		t.Fatalf("cursor offset = %d, want 3", win.Cursor.Offset)
	}
}

func TestForwardSexpRepeat(t *testing.T) {
	win, _ := setupSexpTest("(a) (b)", buffer.LModeC, 1, 0)
	if win == nil {
		t.Fatal("setup failed")
	}
	if !CmdForwardSexp(false, 2) {
		t.Fatal("CmdForwardSexp n=2 failed")
	}
	if win.Cursor.Offset != 7 {
		t.Fatalf("cursor offset = %d, want 7", win.Cursor.Offset)
	}
}

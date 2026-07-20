package runtime

import (
	"testing"

	"github.com/jdpalmer/jem/term"
)

func TestCmdMove(t *testing.T) {
	te := NewTestEditor(t)

	te.LoadText("   hello")
	te.SetCursor(1, 6)
	te.Key(term.META | 'm')
	te.ExpectCursor(1, 3)

	te.LoadText("hello")
	te.SetCursor(1, 3)
	te.Press("C-a")
	te.ExpectCursor(1, 0)

	te.SetCursor(1, 2)
	te.Key(term.KeyLeft)
	te.ExpectCursor(1, 1)

	te.SetCursor(1, 2)
	te.Key(term.KeyRight)
	te.ExpectCursor(1, 3)

	te.LoadText("aaa\nbbb\nccc")
	te.SetCursor(3, 2)
	te.Press("M-<")
	te.ExpectCursor(1, 0)

	te.SetCursor(1, 0)
	te.Press("M->")
	te.ExpectCursor(3, 3)

	te.LoadText("aaa\nbbb\nccc")
	te.SetCursor(1, 0)
	State.MovementState = CmdStateNone
	te.Key(term.KeyDown)
	te.ExpectCursor(2, 0)

	te.SetCursor(3, 0)
	State.MovementState = CmdStateNone
	te.Key(term.KeyUp)
	te.ExpectCursor(2, 0)
}

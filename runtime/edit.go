package runtime

import (
	"errors"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
)

// History is the process-wide undo history (same pointer as buffer.History after BindHistory).
var History *buffer.UndoHistory = buffer.History

func BindHistory(h *buffer.UndoHistory) {
	buffer.BindHistory(h)
	History = buffer.History
}

func BeginCommand() {
	win := window.Active.CurrentWindow
	if win == nil {
		return
	}
	buffer.BeginCommand(win.Cursor)
}

func EndCommand() { buffer.EndCommand() }

func ForgetBuffer(buf *buffer.Buffer) {
	if History != nil {
		History.ForgetBuffer(buf)
	}
}

func NoteBufferSaved(buf *buffer.Buffer) {
	if History != nil {
		History.NoteBufferSaved(buf)
	}
}

func MarkPasteDirty() {
	if minibuffer.Active != nil {
		return
	}
	display.Active.ScreenDirty = true
	for _, win := range window.Active.Windows {
		if win != nil {
			win.ShouldRedraw = true
			win.ShouldUpdateModeLine = true
		}
	}
}

func undoInsertText(win *window.Window, lineNumber, offset int, text []byte) error {
	buf := win.Buffer
	loc := buffer.MakeLocation(lineNumber, offset)
	meta, err := buf.ReplaceRaw(loc, loc, text, nil)
	if err != nil {
		return err
	}
	window.NotifyReplace(buf, loc, meta, false)
	buf.IsChanged = true
	return nil
}

func undoDeleteText(win *window.Window, lineNumber, offset int, text []byte) error {
	buf := win.Buffer
	begin := buffer.MakeLocation(lineNumber, offset)
	endLine := lineNumber
	endOffset := offset
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			endLine++
			endOffset = 0
		} else {
			endOffset++
		}
	}
	isStructural := endLine != lineNumber
	meta, err := buf.ReplaceRaw(begin, buffer.MakeLocation(endLine, endOffset), nil, nil)
	if err != nil {
		return err
	}
	window.NotifyReplace(buf, begin, meta, isStructural)
	buf.IsChanged = true
	return nil
}

func CmdUndo(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	for i := 0; i < n; i++ {
		win := window.Active.CurrentWindow
		err := History.Undo(buffer.UndoReplay{
			InsertText: func(lineNumber, offset int, text []byte) error {
				if win == nil {
					return window.ErrNilWindow
				}
				return undoInsertText(win, lineNumber, offset, text)
			},
			DeleteText: func(lineNumber, offset int, text []byte) error {
				if win == nil {
					return window.ErrNilWindow
				}
				return undoDeleteText(win, lineNumber, offset, text)
			},
			SetCursor: func(loc buffer.Location) {
				if win != nil {
					win.SetCursor(loc)
					win.DidMove = true
				}
			},
			SwitchBuffer: func(buf *buffer.Buffer) {
				window.SwitchBuffer(buf)
			},
			CurrentBuffer: func() *buffer.Buffer {
				return buffer.All.Current
			},
			OnRestoredSave: func(buf *buffer.Buffer) {
				buf.IsChanged = false
				if win != nil {
					win.ShouldUpdateModeLine = true
				}
			},
		})
		if err != nil {
			if errors.Is(err, buffer.ErrNoUndo) {
				display.MBWrite("[no undo]")
			} else {
				display.MBWrite("[undo failed]")
			}
			return false
		}
	}
	display.MBWrite("[undo]")
	return true
}

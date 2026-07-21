package runtime

import (
	"errors"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
)

var defaultHistory buffer.UndoHistory
var History *buffer.UndoHistory = &defaultHistory

func BindHistory(h *buffer.UndoHistory) {
	if h == nil {
		History = &defaultHistory
		return
	}
	History = h
}

func BeginCommand() {
	wp := window.Active.CurrentWindow
	if History.IsReplaying || buffer.All.Current == nil || wp == nil {
		return
	}
	History.BeginCommand(buffer.All.Current, buffer.MakeLocation(wp.Cursor.Line, wp.Cursor.Offset))
}

func EndCommand() { History.EndCommand() }

func ForgetBuffer(bp *buffer.Buffer) { History.ForgetBuffer(bp) }

func NoteBufferSaved(bp *buffer.Buffer) { History.NoteBufferSaved(bp) }

func SetText(bp *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error {
	if bp == nil {
		return buffer.ErrNilBuffer
	}
	return bp.SetText(History, begin, end, newText, newEndOut)
}

func MarkPasteDirty() {
	if minibuffer.Active != nil {
		return
	}
	display.Active.ScreenDirty = true
	for _, wp := range window.Active.Windows {
		if wp != nil {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
	}
}

func undoInsertText(wp *window.Window, lineNumber, offset uint, text []byte) error {
	bp := wp.Buffer
	loc := buffer.MakeLocation(lineNumber, offset)
	bp.NoteEdit(false)
	return bp.ReplaceRaw(loc, loc, text, nil)
}

func undoDeleteText(wp *window.Window, lineNumber, offset uint, text []byte) error {
	bp := wp.Buffer
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
	bp.NoteEdit(endLine != lineNumber)
	return bp.ReplaceRaw(begin, buffer.MakeLocation(endLine, endOffset), nil, nil)
}

func CmdUndo(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	for i := 0; i < n; i++ {
		wp := window.Active.CurrentWindow
		err := History.Undo(buffer.UndoReplay{
			InsertText: func(lineNumber, offset uint, text []byte) error {
				if wp == nil {
					return window.ErrNilWindow
				}
				return undoInsertText(wp, lineNumber, offset, text)
			},
			DeleteText: func(lineNumber, offset uint, text []byte) error {
				if wp == nil {
					return window.ErrNilWindow
				}
				return undoDeleteText(wp, lineNumber, offset, text)
			},
			SetCursor: func(loc buffer.Location) {
				if wp != nil {
					wp.SetCursor(loc)
					wp.DidMove = true
				}
			},
			SwitchBuffer: func(bp *buffer.Buffer) {
				window.SwitchBuffer(bp)
			},
			CurrentBuffer: func() *buffer.Buffer {
				return buffer.All.Current
			},
			OnRestoredSave: func(bp *buffer.Buffer) {
				bp.IsChanged = false
				if wp != nil {
					wp.ShouldUpdateModeLine = true
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

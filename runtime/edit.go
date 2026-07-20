package runtime

import (
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

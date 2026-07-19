package editor

import (
	"fmt"

	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
)

func undoInsertText(wp *model.Window, lineNumber, offset uint, text []byte) bool {
	bp := wp.Buffer
	loc := buffer.MakeLocation(lineNumber, offset)
	bp.NoteEdit(false)
	return bp.ReplaceRaw(loc, loc, text, nil) == nil
}

func undoDeleteText(wp *model.Window, lineNumber, offset uint, text []byte) bool {
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
	return bp.ReplaceRaw(begin, buffer.MakeLocation(endLine, endOffset), nil, nil) == nil
}

func CmdUndo(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	for i := 0; i < n; i++ {
		if model.History.Count == 0 {
			fmt.Println("[no undo]")
			return false
		}
		wp := model.State.CurrentWindow
		ok := model.History.Undo(buffer.UndoReplay{
			InsertText: func(lineNumber, offset uint, text []byte) bool {
				if wp == nil {
					return false
				}
				return undoInsertText(wp, lineNumber, offset, text)
			},
			DeleteText: func(lineNumber, offset uint, text []byte) bool {
				if wp == nil {
					return false
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
				model.SwitchBuffer(bp)
			},
			CurrentBuffer: func() *buffer.Buffer {
				return model.State.CurrentBuffer
			},
			OnRestoredSave: func(bp *buffer.Buffer) {
				bp.IsChanged = false
				if wp != nil {
					wp.ShouldUpdateModeLine = true
				}
			},
		})
		if !ok {
			fmt.Println("[undo failed]")
			return false
		}
	}
	fmt.Println("[undo]")
	return true
}

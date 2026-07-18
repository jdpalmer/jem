package editor

// edit.go — kill ring, undo command, and editor-specific buffer wiring.

import (
	"fmt"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/edit"
)

func UndoBeginCommand() {
	edit.BeginCommand()
}

func UndoEndCommand() {
	edit.EndCommand()
}

func UndoForgetBuffer(bp *buffer.Buffer) {
	edit.ForgetBuffer(bp)
}

func UndoNoteBufferSaved(bp *buffer.Buffer) {
	edit.NoteBufferSaved(bp)
}

func undoInsertText(wp *app.Window, lineNumber, offset uint, text []byte) bool {
	bp := wp.Buffer
	loc := buffer.MakeLocation(lineNumber, offset)
	bp.NoteEdit(false)
	return bp.ReplaceRaw(loc, loc, text, nil) == nil
}

func undoDeleteText(wp *app.Window, lineNumber, offset uint, text []byte) bool {
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
		if edit.History.Count == 0 {
			fmt.Println("[no undo]")
			return false
		}
		wp := app.State.CurrentWindow
		ok := edit.History.Undo(buffer.UndoReplay{
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
				editorSwitchBuffer(bp)
			},
			CurrentBuffer: func() *buffer.Buffer {
				return app.State.CurrentBuffer
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

func editorSwitchBuffer(bp *buffer.Buffer) {
	if bp == nil {
		return
	}
	cw := app.State.CurrentWindow
	if cw == nil {
		return
	}

	cw.SaveState()

	app.SetCurrentBuffer(bp)
	cw.Buffer = bp
	cw.ShouldUpdateModeLine = true
	cw.ShouldReframe = true
	cw.ShouldRedraw = true
	cw.SetTopLine(1)
	cw.HScroll = 0

	for i := 0; i < int(len(app.State.WINDOWS)); i++ {
		wp := app.State.WINDOWS[i]
		if wp != nil && wp != cw && wp.Buffer == bp {
			cw.TopLine = wp.TopLine
			cw.Cursor = wp.Cursor
			cw.Mark = wp.Mark
			cw.HScroll = wp.HScroll
			return
		}
	}

	if bp.Cursor.Line >= 1 {
		cw.SetCursor(bp.Cursor)
	} else {
		cw.SetCursor(buffer.Location{Line: 1, Offset: 0})
	}
	cw.Mark = bp.Mark
}

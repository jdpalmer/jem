package editor

// edit.go — kill ring, undo command, and editor-specific buffer wiring.

import (
	"fmt"

	"github.com/jdpalmer/jem/buffer"
	sess "github.com/jdpalmer/jem/session"
)

var editorUndo buffer.UndoHistory

func UndoBeginCommand() {
	wp := session.App.CurrentWindow
	if editorUndo.IsReplaying || session.App.CurrentBuffer == nil || wp == nil {
		return
	}
	editorUndo.BeginCommand(session.App.CurrentBuffer, MakeLocation(wp.Cursor.Line, wp.Cursor.Offset))
}

func UndoEndCommand() {
	editorUndo.EndCommand()
}

func UndoForgetBuffer(bp *Buffer) {
	editorUndo.ForgetBuffer(bp)
}

func UndoNoteBufferSaved(bp *Buffer) {
	editorUndo.NoteBufferSaved(bp)
}

func undoInsertText(wp *Window, lineNumber, offset uint, text []byte, length uint) bool {
	bp := wp.Buffer
	loc := MakeLocation(lineNumber, offset)
	buffer.NoteEdit(bp, false)
	return bufferReplaceRaw(bp, loc, loc, text, length, nil)
}

func undoDeleteText(wp *Window, lineNumber, offset uint, text []byte, length uint) bool {
	bp := wp.Buffer
	begin := MakeLocation(lineNumber, offset)
	endLine := lineNumber
	endOffset := offset
	for i := uint(0); i < length; i++ {
		if text[i] == '\n' {
			endLine++
			endOffset = 0
		} else {
			endOffset++
		}
	}
	buffer.NoteEdit(bp, endLine != lineNumber)
	return bufferReplaceRaw(bp, begin, MakeLocation(endLine, endOffset), nil, 0, nil)
}

func CmdUndo(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	for i := 0; i < n; i++ {
		if editorUndo.Count == 0 {
			fmt.Println("[no undo]")
			return false
		}
		wp := session.App.CurrentWindow
		ok := editorUndo.Undo(buffer.UndoReplay{
			InsertText: func(lineNumber, offset uint, text []byte, length uint) bool {
				if wp == nil {
					return false
				}
				return undoInsertText(wp, lineNumber, offset, text, length)
			},
			DeleteText: func(lineNumber, offset uint, text []byte, length uint) bool {
				if wp == nil {
					return false
				}
				return undoDeleteText(wp, lineNumber, offset, text, length)
			},
			SetCursor: func(loc Location) {
				if wp != nil {
					sess.WindowSetCursor(wp, loc)
					wp.DidMove = true
				}
			},
			SwitchBuffer: func(bp *Buffer) {
				editorSwitchBuffer(bp)
			},
			CurrentBuffer: func() *Buffer {
				return session.App.CurrentBuffer
			},
			OnRestoredSave: func(bp *Buffer) {
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

func editorSwitchBuffer(bp *Buffer) {
	if bp == nil {
		return
	}
	cw := session.App.CurrentWindow
	if cw == nil {
		return
	}

	sess.WindowSaveState(cw)

	sess.SetCurrentBuffer(bp)
	cw.Buffer = bp
	cw.ShouldUpdateModeLine = true
	cw.ShouldReframe = true
	cw.ShouldRedraw = true
	sess.WindowSetTopLine(cw, 1)
	cw.HScroll = 0

	for i := 0; i < int(session.App.WindowCount); i++ {
		wp := session.App.WINDOWS[i]
		if wp != nil && wp != cw && wp.Buffer == bp {
			cw.TopLine = wp.TopLine
			cw.Cursor = wp.Cursor
			cw.Mark = wp.Mark
			cw.HScroll = wp.HScroll
			return
		}
	}

	if bp.Cursor.Line >= 1 {
		sess.WindowSetCursor(cw, bp.Cursor)
	} else {
		sess.WindowSetCursor(cw, Location{Line: 1, Offset: 0})
	}
	cw.Mark = bp.Mark
}

var killRing [16][]byte
var killRingCount uint8
var killRingIdx uint8
var killAggregate []byte

func killBegin() {
	if session.App.KillState == CmdStateNone {
		killAggregate = nil
	}
	session.App.KillState = CmdStateCurrent
}

func killAppend(text []byte, length uint) bool {
	if length == 0 {
		return true
	}
	killAggregate = append(killAggregate, text[:length]...)
	entry := make([]byte, length)
	copy(entry, text[:length])
	killRing[killRingIdx] = entry
	killRingIdx = (killRingIdx + 1) % 16
	if killRingCount < 16 {
		killRingCount++
	}
	return true
}

func killBytes(length *uint) []byte {
	if length != nil {
		*length = uint(len(killAggregate))
	}
	return killAggregate
}

func killWriteClipboard() {
	if len(killAggregate) == 0 && killRingCount > 0 {
		idx := (killRingIdx + 15) % 16
		_ = clipboardWriteText(killRing[idx])
		return
	}
	if len(killAggregate) > 0 {
		_ = clipboardWriteText(killAggregate)
	}
}

func killReadClipboard() bool {
	data, ok := clipboardReadText()
	if !ok {
		mbWrite("[clipboard read failed]")
		return false
	}
	killAggregate = make([]byte, len(data))
	copy(killAggregate, data)
	entry := make([]byte, len(data))
	copy(entry, data)
	killRing[killRingIdx] = entry
	killRingIdx = (killRingIdx + 1) % 16
	if killRingCount < 16 {
		killRingCount++
	}
	return true
}

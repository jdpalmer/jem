package ui

import (
	"github.com/jdpalmer/jem/buffer"
	sesspkg "github.com/jdpalmer/jem/session"
)

var (
	GlobalKeyCh      chan uint32
	GlobalMinibufKeyCh chan uint32
	marksState = &sesspkg.MarksState
)

func windowCreate() *Window { return sesspkg.WindowCreate() }
func windowRetile()         { sesspkg.WindowRetile() }
func windowSelect(wp *Window) {
	sesspkg.WindowSelect(wp)
}
func windowSetCursor(wp *Window, loc Location) { sesspkg.WindowSetCursor(wp, loc) }
func windowSetTopLine(wp *Window, line uint)   { sesspkg.WindowSetTopLine(wp, line) }
func WindowCenterCursor(wp *Window)            { sesspkg.WindowCenterCursor(wp) }
func WindowGutterWidth(wp *Window) uint32      { return sesspkg.WindowGutterWidth(wp) }
func bufferFind(name string) *Buffer           { return sesspkg.BufferFind(name) }
func bufferCreate(ed *EditorRuntimeState) *Buffer {
	return sesspkg.BufferCreate(ed)
}
func bufferSetText(bp *Buffer, begin, end Location, newText []byte, newLen uint, newEndOut *Location, kill bool) bool {
	if kill {
		var oldLen uint
		oldText := buffer.GetText(bp, begin, end, &oldLen)
		if oldLen > 0 {
			if PackageHooks.KillAppend == nil || !PackageHooks.KillAppend(oldText, oldLen) {
				return false
			}
		}
	}
	ok := buffer.SetText(bp, nil, begin, end, newText, newLen, newEndOut)
	if kill && ok && PackageHooks.KillWriteClipboard != nil {
		PackageHooks.KillWriteClipboard()
	}
	return ok
}
func gitLineDiff(bp *Buffer, lineNumber uint) GitLineDiff {
	if PackageHooks.GitLineDiff == nil {
		return GitLineDiffNone
	}
	return PackageHooks.GitLineDiff(bp, lineNumber)
}
func gitModelineText(bp *Buffer) string {
	if PackageHooks.GitModelineText == nil {
		return ""
	}
	return PackageHooks.GitModelineText(bp)
}

func mbWriteHook(format string, args ...any) {
	if PackageHooks.MBWrite != nil {
		PackageHooks.MBWrite(format, args...)
		return
	}
	mbWrite(format, args...)
}

func bufferChoiceLabel(ctx any, idx uint8) []byte {
	buffers := ctx.([]*Buffer)
	if int(idx) >= len(buffers) {
		return nil
	}
	bp := buffers[int(idx)]
	if bp == nil {
		return nil
	}
	return []byte(bp.Name)
}

func editorInsertPaste(text []byte, length int) bool {
	if PackageHooks.EditorInsertPaste != nil {
		return PackageHooks.EditorInsertPaste(text, length)
	}
	wp := session.App.CurrentWindow
	if wp == nil || wp.Buffer == nil {
		return false
	}
	if length > len(text) {
		length = len(text)
	}
	paste := append([]byte(nil), text[:length]...)
	for i := range paste {
		if paste[i] == '\r' {
			paste[i] = '\n'
		}
	}
	loc := wp.Cursor
	var newEnd Location
	if !bufferSetText(wp.Buffer, loc, loc, paste, uint(len(paste)), &newEnd, false) {
		return false
	}
	windowSetCursor(wp, newEnd)
	return true
}


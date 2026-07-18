package ui

import (
	"github.com/jdpalmer/jem/app"
)

var (
	GlobalKeyCh        chan uint32
	GlobalMinibufKeyCh chan uint32
	marksState         = &app.MarksState
)

func bufferSetText(bp *Buffer, begin, end Location, newText []byte, newEndOut *Location, kill bool) bool {
	if kill {
		oldText := bp.GetText(begin, end)
		if len(oldText) > 0 {
			if PackageHooks.KillAppend == nil || !PackageHooks.KillAppend(oldText) {
				return false
			}
		}
	}
	ok := bp.SetText(nil, begin, end, newText, newEndOut)
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

func editorInsertPaste(text []byte) bool {
	if PackageHooks.EditorInsertPaste != nil {
		return PackageHooks.EditorInsertPaste(text)
	}
	wp := app.State.CurrentWindow
	if wp == nil || wp.Buffer == nil {
		return false
	}
	paste := append([]byte(nil), text...)
	for i := range paste {
		if paste[i] == '\r' {
			paste[i] = '\n'
		}
	}
	loc := wp.Cursor
	var newEnd Location
	if !bufferSetText(wp.Buffer, loc, loc, paste, &newEnd, false) {
		return false
	}
	wp.SetCursor(newEnd)
	return true
}

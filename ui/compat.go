package ui

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/edit"
)

var (
	GlobalKeyCh        chan uint32
	GlobalMinibufKeyCh chan uint32
	marksState         = &app.MarksState
)

func bufferSetText(bp *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location, kill bool) bool {
	if kill {
		oldText := bp.GetText(begin, end)
		if len(oldText) > 0 && !edit.KillAppend(oldText) {
			return false
		}
	}
	if err := bp.SetText(nil, begin, end, newText, newEndOut); err != nil {
		return false
	}
	if kill {
		edit.KillWriteClipboard()
	}
	return true
}

func gitLineDiff(bp *buffer.Buffer, lineNumber uint) app.GitLineDiff {
	if PackageHooks.GitLineDiff == nil {
		return app.GitLineDiffNone
	}
	return PackageHooks.GitLineDiff(bp, lineNumber)
}

func gitModelineText(bp *buffer.Buffer) string {
	if PackageHooks.GitModelineText == nil {
		return ""
	}
	return PackageHooks.GitModelineText(bp)
}

func bufferChoiceLabel(ctx any, idx uint8) []byte {
	buffers := ctx.([]*buffer.Buffer)
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
	return edit.InsertText(wp, paste)
}

package display

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/minibuffer"
)

// InitInputChannels is retained for call-site compatibility (paste now uses PasteEvent).
func InitInputChannels(pasteQueueSize int) {
	_ = pasteQueueSize
}

// ShowMinibuffer marks the session minibuffer active for paste/display.
func ShowMinibuffer(state *minibuffer.MinibufferState) {
	minibuffer.Active = state
}

// HideMinibuffer clears ActiveMinibuffer.
func HideMinibuffer() {
	minibuffer.Active = nil
}

func gitLineDiff(bp *buffer.Buffer, lineNumber uint) int {
	if PackageHooks.GitLineDiff == nil {
		return 0
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

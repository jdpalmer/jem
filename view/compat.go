package view

import (
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
)

// InitInputChannels is retained for call-site compatibility (paste now uses PasteEvent).
func InitInputChannels(pasteQueueSize int) {
	_ = pasteQueueSize
}

// BeginMinibufCapture installs the editor key-capture listener (via hooks).
func BeginMinibufCapture() {
	if PackageHooks.BeginMinibuf != nil {
		PackageHooks.BeginMinibuf()
	}
}

// EndMinibufCapture uninstalls the key-capture listener (via hooks).
func EndMinibufCapture() {
	if PackageHooks.EndMinibuf != nil {
		PackageHooks.EndMinibuf()
	}
}

// WaitKey reads the next key from the single event bus (via editor hook).
func WaitKey() (uint32, bool) {
	if PackageHooks.WaitKey == nil {
		return 0, false
	}
	return PackageHooks.WaitKey()
}

// ShowMinibuffer marks the session minibuffer active for paste/display (no key capture).
func ShowMinibuffer(state *model.MinibufferState) {
	model.State.ActiveMinibuffer = state
}

// HideMinibuffer clears ActiveMinibuffer without touching the capture listener.
func HideMinibuffer() {
	model.State.ActiveMinibuffer = nil
}

// ActivateMinibuffer shows the minibuffer and begins nested key capture (blocking prompts).
func ActivateMinibuffer(state *model.MinibufferState) {
	ShowMinibuffer(state)
	BeginMinibufCapture()
}

// DeactivateMinibuffer clears ActiveMinibuffer and ends key capture.
func DeactivateMinibuffer() {
	HideMinibuffer()
	EndMinibufCapture()
}

func gitLineDiff(bp *buffer.Buffer, lineNumber uint) model.GitLineDiff {
	if PackageHooks.GitLineDiff == nil {
		return model.GitLineDiffNone
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

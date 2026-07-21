package tools

import (
	"bytes"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
)

func promptStringFromBuf(data []byte) string {
	n := bytes.IndexByte(data, 0)
	if n < 0 {
		n = len(data)
	}
	return string(data[:n])
}

func mbWrite(format string, args ...interface{}) {
	display.MBWrite(format, args...)
}

func mbClear() {
	display.MBClear()
}

func mbHistoryAdd(text string) {
	display.MBHistoryAdd(text)
}

func askString(prompt, initial string, onDone func(string, minibuffer.PromptResult)) {
	if PackageHooks.AskString != nil {
		PackageHooks.AskString(prompt, initial, onDone)
		return
	}
	if onDone != nil {
		onDone("", minibuffer.PromptResultAbort)
	}
}

func askStringCap(prompt, initial string, capacity int, onDone func(string, minibuffer.PromptResult)) {
	if PackageHooks.AskStringCap != nil {
		PackageHooks.AskStringCap(prompt, initial, capacity, onDone)
		return
	}
	if onDone != nil {
		onDone("", minibuffer.PromptResultAbort)
	}
}

func askFuzzyEx(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter minibuffer.MbMatchFormatter, displayCtx any, onDone func(string, minibuffer.PromptResult)) {
	if PackageHooks.AskFuzzyEx != nil {
		PackageHooks.AskFuzzyEx(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx, onDone)
		return
	}
	if onDone != nil {
		onDone("", minibuffer.PromptResultAbort)
	}
}

func markPushCurrent() {
	markring.PushCurrent()
}

func fileVisitLocation(path string, line, column uint32) bool {
	if PackageHooks.VisitLocation == nil {
		return false
	}
	return PackageHooks.VisitLocation(path, line, column)
}

func editorSwitchBuffer(buf *buffer.Buffer) {
	if PackageHooks.SwitchBuffer != nil {
		PackageHooks.SwitchBuffer(buf)
		return
	}
	window.SwitchBuffer(buf)
}

func CmdAbort(_ bool, _ int) bool {
	if PackageHooks.Abort != nil {
		PackageHooks.Abort()
		return true
	}
	return false
}

func TermFreezeInput() bool {
	return display.TermFreezeInput()
}

func TermThawInput() {
	display.TermThawInput()
}

func editorReadKey(keyOut *uint32) bool {
	if PackageHooks.ReadKey == nil || keyOut == nil {
		return false
	}
	k, ok := PackageHooks.ReadKey()
	if !ok {
		return false
	}
	*keyOut = k
	return true
}

func windowRetile() {
	window.WindowRetile()
}

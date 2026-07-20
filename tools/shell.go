package tools

import (
	"bytes"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
)

func promptStringFromBuf(buf []byte) string {
	n := bytes.IndexByte(buf, 0)
	if n < 0 {
		n = len(buf)
	}
	return string(buf[:n])
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
	display.AskString(prompt, initial, onDone)
}

func askStringCap(prompt, initial string, capacity int, onDone func(string, minibuffer.PromptResult)) {
	display.AskStringCap(prompt, initial, capacity, onDone)
}

func askFuzzyEx(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter minibuffer.MbMatchFormatter, displayCtx any, onDone func(string, minibuffer.PromptResult)) {
	display.AskFuzzyEx(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx, onDone)
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

func editorSwitchBuffer(bp *buffer.Buffer) {
	if PackageHooks.SwitchBuffer != nil {
		PackageHooks.SwitchBuffer(bp)
		return
	}
	window.SwitchBuffer(bp)
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

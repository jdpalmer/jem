package tools

import (
	"bytes"

	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/view"
)

func promptStringFromBuf(buf []byte) string {
	n := bytes.IndexByte(buf, 0)
	if n < 0 {
		n = len(buf)
	}
	return string(buf[:n])
}

func mbWrite(format string, args ...interface{}) {
	view.MBWrite(format, args...)
}

func mbClear() {
	view.MBClear()
}

func mbHistoryAdd(text string) {
	view.MBHistoryAdd(text)
}

func askString(prompt, initial string, onDone func(string, model.PromptResult)) {
	view.AskString(prompt, initial, onDone)
}

func askStringCap(prompt, initial string, capacity int, onDone func(string, model.PromptResult)) {
	view.AskStringCap(prompt, initial, capacity, onDone)
}

func askFuzzyEx(prompt string, provider model.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter model.MbMatchFormatter, displayCtx any, onDone func(string, model.PromptResult)) {
	view.AskFuzzyEx(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx, onDone)
}

func markPushCurrent() {
	model.MarkPushCurrent()
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
	model.SwitchBuffer(bp)
}

func CmdAbort(_ bool, _ int) bool {
	if PackageHooks.Abort != nil {
		PackageHooks.Abort()
		return true
	}
	return false
}

func TermFreezeInput() bool {
	return view.TermFreezeInput()
}

func TermThawInput() {
	view.TermThawInput()
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
	model.WindowRetile()
}

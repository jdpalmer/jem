package tools

import (
	"bytes"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/ui"
)

func promptStringFromBuf(buf []byte) string {
	n := bytes.IndexByte(buf, 0)
	if n < 0 {
		n = len(buf)
	}
	return string(buf[:n])
}

func mbWrite(format string, args ...interface{}) {
	ui.MBWrite(format, args...)
}

func mbClear() {
	ui.MBClear()
}

func mbHistoryAdd(text string) {
	ui.MBHistoryAdd(text)
}

func mbReadString(prompt, initial string) (string, app.PromptResult) {
	return ui.MBReadString(prompt, initial)
}

func mbReadStringCap(prompt, initial string, capacity int) (string, app.PromptResult) {
	return ui.MBReadStringCap(prompt, initial, capacity)
}

func mbReadFuzzyListExString(prompt string, provider app.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter app.MbMatchFormatter, displayCtx any) (string, app.PromptResult) {
	return ui.MBReadFuzzyListExString(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx)
}

func markPushCurrent() {
	app.MarkPushCurrent()
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
	app.SetCurrentBuffer(bp)
	if wp := app.State.CurrentWindow; wp != nil {
		wp.Buffer = bp
	}
}

func CmdAbort(_ bool, _ int) bool {
	if PackageHooks.Abort != nil {
		PackageHooks.Abort()
		return true
	}
	return false
}

func TermFreezeInput() bool {
	return ui.TermFreezeInput()
}

func TermThawInput() {
	ui.TermThawInput()
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
	app.WindowRetile()
}

package tools

import (
	"bytes"

	"github.com/jdpalmer/jem/app"
)

type (
	Buffer       = app.Buffer
	Line         = app.Line
	Location     = app.Location
	Window       = app.Window
	PromptResult = app.PromptResult
	GitLineDiff  = app.GitLineDiff
)

const (
	MaxBuffers            = app.MaxBuffers
	PatternCapacity       = app.PatternCapacity
	CommandPromptCapacity = app.CommandPromptCapacity
	PromptResultYes       = app.PromptResultYes
	PromptResultAbort     = app.PromptResultAbort
	KeyEnter              = app.KeyEnter
	LModeMarkdown         = app.LModeMarkdown

	GitLineDiffNone     = app.GitLineDiffNone
	GitLineDiffAdded    = app.GitLineDiffAdded
	GitLineDiffModified = app.GitLineDiffModified
	GitLineDiffDeleted  = app.GitLineDiffDeleted
)

func promptStringFromBuf(buf []byte) string {
	n := bytes.IndexByte(buf, 0)
	if n < 0 {
		n = len(buf)
	}
	return string(buf[:n])
}

func mbWrite(format string, args ...interface{}) {
	if PackageHooks.MBWrite != nil {
		PackageHooks.MBWrite(format, args...)
	}
}

func mbClear() {
	if PackageHooks.MBClear != nil {
		PackageHooks.MBClear()
	}
}

func mbHistoryAdd(text string) {
	if PackageHooks.MBHistoryAdd != nil {
		PackageHooks.MBHistoryAdd(text)
	}
}

func mbReadString(prompt, initial string) (string, PromptResult) {
	if PackageHooks.MBReadString == nil {
		return "", PromptResultAbort
	}
	return PackageHooks.MBReadString(prompt, initial)
}

func mbReadStringCap(prompt, initial string, capacity int) (string, PromptResult) {
	if PackageHooks.MBReadStringCap != nil {
		return PackageHooks.MBReadStringCap(prompt, initial, capacity)
	}
	return mbReadString(prompt, initial)
}

func mbReadFuzzyListExString(prompt string, provider app.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter app.MbMatchFormatter, displayCtx any) (string, PromptResult) {
	if PackageHooks.MBReadFuzzyListExString == nil {
		return "", PromptResultAbort
	}
	return PackageHooks.MBReadFuzzyListExString(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx)
}

func markPushCurrent() {
	if PackageHooks.MarkPushCurrent != nil {
		PackageHooks.MarkPushCurrent()
	}
}

func fileVisitLocation(path string, line, column uint32) bool {
	if PackageHooks.VisitLocation == nil {
		return false
	}
	return PackageHooks.VisitLocation(path, line, column)
}

func editorSwitchBuffer(bp *Buffer) {
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
	if PackageHooks.TermFreezeInput == nil {
		return false
	}
	return PackageHooks.TermFreezeInput()
}

func TermThawInput() {
	if PackageHooks.TermThawInput != nil {
		PackageHooks.TermThawInput()
	}
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
	if PackageHooks.WindowRetile != nil {
		PackageHooks.WindowRetile()
	}
}

package tools

import (
	"bytes"

	sess "github.com/jdpalmer/jem/session"
)

type (
	Buffer       = sess.Buffer
	Line         = sess.Line
	Location     = sess.Location
	Window       = sess.Window
	PromptResult = sess.PromptResult
	GitLineDiff  = sess.GitLineDiff
)

const (
	MaxBuffers            = sess.MaxBuffers
	PatternCapacity       = sess.PatternCapacity
	CommandPromptCapacity = sess.CommandPromptCapacity
	PromptResultYes       = sess.PromptResultYes
	PromptResultAbort     = sess.PromptResultAbort
	KeyEnter              = sess.KeyEnter
	LModeMarkdown         = sess.LModeMarkdown

	GitLineDiffNone     = sess.GitLineDiffNone
	GitLineDiffAdded    = sess.GitLineDiffAdded
	GitLineDiffModified = sess.GitLineDiffModified
	GitLineDiffDeleted  = sess.GitLineDiffDeleted
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

func mbReadFuzzyListExString(prompt string, provider sess.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter sess.MbMatchFormatter, displayCtx any) (string, PromptResult) {
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
	sess.SetCurrentBuffer(bp)
	if wp := sess.App.CurrentWindow; wp != nil {
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

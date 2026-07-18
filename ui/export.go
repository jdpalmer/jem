package ui

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
)

// Exported minibuffer wrappers used by editor bridge code.
func MBWrite(format string, args ...any) { mbWrite(format, args...) }
func MBClear()                           { mbClear() }
func MBHistoryAdd(text string)           { mbHistoryAdd(text) }

func MBWritePromptStyle(prompt string, text []byte, cpos int, style buffer.TextStyle) {
	mbWritePromptStyle(prompt, text, cpos, style)
}

func MBReadString(prompt, initial string) (string, app.PromptResult) {
	return mbReadString(prompt, initial)
}

func MBReadStringCap(prompt, initial string, capacity int) (string, app.PromptResult) {
	return mbReadStringCap(prompt, initial, capacity)
}

func MBReadFilenameString(prompt, initial string) (string, app.PromptResult) {
	return mbReadFilenameString(prompt, initial)
}

func MBReadFuzzyListString(prompt string, provider app.MbNameProviderFn, providerCtx any, providerCount uint) (string, app.PromptResult) {
	return mbReadFuzzyListString(prompt, provider, providerCtx, providerCount)
}

func MBReadFuzzyListExString(prompt string, provider app.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter app.MbMatchFormatter, displayCtx any) (string, app.PromptResult) {
	return mbReadFuzzyListExString(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx)
}

func MBReadCommand(buf []byte, nbuf int) app.PromptResult {
	return mbReadCommand(buf, nbuf)
}

func MBYesNo(prompt string) app.PromptResult {
	return mbYesNo(prompt)
}

func MBChoose(prompt string, ctx any, labelFn app.MLChoiceLabelFn, count uint8, defaultIdx uint8) int16 {
	return mbChoose(prompt, ctx, labelFn, count, defaultIdx)
}

func MBEditKeyHistory(buf []byte, cpos *int, nbuf int, initial []byte, historyPos *int16, haveSavedEdit *bool, savedEdit []byte, k uint32) app.MinibufferEditResult {
	return mbEditKeyHistory(buf, cpos, nbuf, initial, historyPos, haveSavedEdit, savedEdit, k)
}

// Input and mouse wrappers used by editor bridge code.
func QueuePaste(data []byte) {
	queuePaste(data)
}

func IsPasteRedrawKey(k uint32) bool {
	return isPasteRedrawKey(k)
}

func ApplyWheelTicks(net int) {
	applyWheelTicks(net)
}

func StartKeyReader() {
	startKeyReader()
}

func ThemeUpdate() {
	themeUpdate()
}

func WindowCursorScreenCol(wp *app.Window) int {
	return windowCursorScreenCol(wp)
}

func InitInputChannels(globalKeyCh, globalMinibufKeyCh chan uint32, pasteQueueSize int) {
	GlobalKeyCh = globalKeyCh
	GlobalMinibufKeyCh = globalMinibufKeyCh
	if pasteQueueSize <= 0 {
		pasteQueueSize = 4
	}
	pendingPasteCh = make(chan []byte, pasteQueueSize)
}

package editor

import "github.com/jdpalmer/jem/ui"

func mbWrite(format string, args ...any) { ui.MBWrite(format, args...) }
func mbClear()                           { ui.MBClear() }
func mbHistoryAdd(text string)           { ui.MBHistoryAdd(text) }

func DisplayInit() {
	ui.DisplayInit()
}

func DisplayInitHeadless(rows, cols int) {
	ui.DisplayInitHeadless(rows, cols)
}

func DisplayUpdate() {
	ui.DisplayUpdate()
}

func ScreenSync() {
	ui.ScreenSync()
}

func RenderModeline(wp *Window) {
	ui.RenderModeline(wp)
}

func themeUpdate() {
	ui.ThemeUpdate()
}

func windowCursorScreenCol(wp *Window) int {
	return ui.WindowCursorScreenCol(wp)
}

func mbReadString(prompt, initial string) (string, PromptResult) {
	return ui.MBReadString(prompt, initial)
}

func mbReadStringCap(prompt, initial string, capacity int) (string, PromptResult) {
	return ui.MBReadStringCap(prompt, initial, capacity)
}

func mbReadFilenameString(prompt, initial string) (string, PromptResult) {
	return ui.MBReadFilenameString(prompt, initial)
}

func mbReadFuzzyListString(prompt string, provider MbNameProviderFn, providerCtx any, providerCount uint) (string, PromptResult) {
	return ui.MBReadFuzzyListString(prompt, provider, providerCtx, providerCount)
}

func mbReadFuzzyListExString(prompt string, provider MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter MbMatchFormatter, displayCtx any) (string, PromptResult) {
	return ui.MBReadFuzzyListExString(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx)
}

func mbWritePromptStyle(prompt string, text []byte, cpos int, style TextStyle) {
	ui.MBWritePromptStyle(prompt, text, cpos, style)
}

func mbEditKeyHistory(buf []byte, cpos *int, nbuf int, initial []byte, historyPos *int16, haveSavedEdit *bool, savedEdit []byte, k uint32) MinibufferEditResult {
	return ui.MBEditKeyHistory(buf, cpos, nbuf, initial, historyPos, haveSavedEdit, savedEdit, k)
}

func mbReadCommand(buf []byte, nbuf int) PromptResult {
	return ui.MBReadCommand(buf, nbuf)
}

func mbYesNo(prompt string) PromptResult {
	return ui.MBYesNo(prompt)
}

func mbChoose(prompt string, ctx any, labelFn MLChoiceLabelFn, count uint8, defaultIdx uint8) int16 {
	return ui.MBChoose(prompt, ctx, labelFn, count, defaultIdx)
}

func TermFreezeInput() bool {
	return ui.TermFreezeInput()
}

func TermThawInput() {
	ui.TermThawInput()
}

func RequestDisplayRefresh() {
	ui.RequestDisplayRefresh()
}

func queuePaste(data []byte) {
	ui.QueuePaste(data)
}

func isPasteRedrawKey(k uint32) bool {
	return ui.IsPasteRedrawKey(k)
}

func applyWheelTicks(net int) {
	ui.ApplyWheelTicks(net)
}

func startKeyReader() {
	ui.StartKeyReader()
}

func initUIInputChannels() {
	GlobalKeyCh = make(chan uint32, 64)
	GlobalMinibufKeyCh = make(chan uint32, 16)
	ui.InitInputChannels(GlobalKeyCh, GlobalMinibufKeyCh, 4)
}

func CmdRefresh(f bool, n int) bool {
	return ui.CmdRefresh(f, n)
}

func CmdThemeToggle(f bool, n int) bool {
	return ui.CmdThemeToggle(f, n)
}

func CmdMenuRun(f bool, n int) bool {
	return ui.CmdMenuRun(f, n)
}

func CmdMouseLeft(f bool, n int) bool {
	return ui.CmdMouseLeft(f, n)
}

func CmdMouseDrag(f bool, n int) bool {
	return ui.CmdMouseDrag(f, n)
}

func CmdMouseWheelUp(f bool, n int) bool {
	return ui.CmdMouseWheelUp(f, n)
}

func CmdMouseWheelDown(f bool, n int) bool {
	return ui.CmdMouseWheelDown(f, n)
}

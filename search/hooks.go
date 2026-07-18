package search

import "github.com/jdpalmer/jem/app"

type Hooks struct {
	MBWrite            func(format string, args ...interface{})
	MBClear            func()
	MBReadString       func(prompt, initial string) (string, app.PromptResult)
	MBWritePromptStyle func(prompt string, text []byte, cpos int, style app.TextStyle)
	MBHistoryAdd       func(text string)
	MBEditKeyHistory   func(buf []byte, cpos *int, nbuf int, initial []byte, historyPos *int16, haveSavedEdit *bool, savedEdit []byte, k uint32) app.MinibufferEditResult
	DisplayUpdate      func()
	MarkPushCurrent    func()
	ReadKey            func() (uint32, bool)
	IsPasteRedrawKey   func(k uint32) bool
	SetText            func(bp *app.Buffer, begin, end app.Location, newText []byte, newEndOut *app.Location, kill bool) bool
	Beep               func()
}

var PackageHooks Hooks

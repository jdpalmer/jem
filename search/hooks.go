package search

import "github.com/jdpalmer/jem/session"

type Hooks struct {
	MBWrite            func(format string, args ...interface{})
	MBClear            func()
	MBReadString       func(prompt, initial string) (string, session.PromptResult)
	MBWritePromptStyle func(prompt string, text []byte, cpos int, style session.TextStyle)
	MBHistoryAdd       func(text string)
	MBEditKeyHistory   func(buf []byte, cpos *int, nbuf int, initial []byte, historyPos *int16, haveSavedEdit *bool, savedEdit []byte, k uint32) session.MinibufferEditResult
	DisplayUpdate      func()
	MarkPushCurrent    func()
	ReadKey            func() (uint32, bool)
	IsPasteRedrawKey   func(k uint32) bool
	SetText            func(bp *session.Buffer, begin, end session.Location, newText []byte, newLen uint, newEndOut *session.Location, kill bool) bool
	Beep               func()
}

var PackageHooks Hooks

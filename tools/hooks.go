package tools

import "github.com/jdpalmer/jem/app"

type Hooks struct {
	MBWrite                 func(format string, args ...interface{})
	MBClear                 func()
	MBHistoryAdd            func(text string)
	MBReadString            func(prompt, initial string) (string, app.PromptResult)
	MBReadStringCap         func(prompt, initial string, capacity int) (string, app.PromptResult)
	MBReadFuzzyListExString func(prompt string, provider app.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter app.MbMatchFormatter, displayCtx any) (string, app.PromptResult)

	MarkPushCurrent func()
	VisitLocation   func(path string, line, column uint32) bool
	SwitchBuffer    func(bp *app.Buffer)
	Abort           func()

	TermFreezeInput func() bool
	TermThawInput   func()
	ReadKey         func() (uint32, bool)
	WindowRetile    func()
}

var PackageHooks Hooks

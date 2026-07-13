package tools

import sess "github.com/jdpalmer/jem/session"

type Hooks struct {
	MBWrite                 func(format string, args ...interface{})
	MBClear                 func()
	MBHistoryAdd            func(text string)
	MBReadString            func(prompt, initial string) (string, sess.PromptResult)
	MBReadStringCap         func(prompt, initial string, capacity int) (string, sess.PromptResult)
	MBReadFuzzyListExString func(prompt string, provider sess.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter sess.MbMatchFormatter, displayCtx any) (string, sess.PromptResult)

	MarkPushCurrent func()
	VisitLocation   func(path string, line, column uint32) bool
	SwitchBuffer    func(bp *sess.Buffer)
	Abort           func()

	TermFreezeInput func() bool
	TermThawInput   func()
	ReadKey         func() (uint32, bool)
	WindowRetile    func()
}

var PackageHooks Hooks

package display

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/minibuffer"
)

// Hooks are editor-owned callbacks view cannot import directly (cycle).
type Hooks struct {
	ApplyCtlxPrefix             func(second uint32) uint32
	GitLineDiff                 func(bp *buffer.Buffer, lineNumber uint) int
	GitModelineText             func(bp *buffer.Buffer) string
	MacroRecordMinibufferResult func(text []byte)
	TakeMacroPromptReply        func() (string, minibuffer.PromptResult, bool)
	// BeginMinibuf / EndMinibuf / WaitKey route blocking prompts through the
	// editor listener stack (view must not import editor). Prefer Ask* on the
	// main loop; WaitKey remains for test/hook fallbacks.
	BeginMinibuf func()
	EndMinibuf   func()
	WaitKey      func() (uint32, bool)
	// Async prompt APIs (preferred on the main loop path).
	AskString    func(prompt, initial string, onDone func(string, minibuffer.PromptResult))
	AskStringCap func(prompt, initial string, capacity int, onDone func(string, minibuffer.PromptResult))
	AskFuzzy     func(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount uint, onDone func(string, minibuffer.PromptResult))
	AskFuzzyEx   func(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter minibuffer.MbMatchFormatter, displayCtx any, onDone func(string, minibuffer.PromptResult))
	AskFilename  func(prompt, initial string, onDone func(string, minibuffer.PromptResult))
	AskChoose    func(prompt string, ctx any, labelFn minibuffer.MLChoiceLabelFn, count uint8, defaultIdx uint8, onDone func(int16))
}

var PackageHooks Hooks

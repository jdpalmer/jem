package view

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/model"
)

// Hooks are editor-owned callbacks view cannot import directly (cycle).
type Hooks struct {
	ApplyCtlxPrefix             func(second uint32) uint32
	GitLineDiff                 func(bp *buffer.Buffer, lineNumber uint) model.GitLineDiff
	GitModelineText             func(bp *buffer.Buffer) string
	MacroRecordMinibufferResult func(text []byte)
	// BeginMinibuf / EndMinibuf / WaitKey route blocking prompts through the
	// editor listener stack (view must not import editor). Prefer Ask* on the
	// main loop; WaitKey remains for test/hook fallbacks.
	BeginMinibuf func()
	EndMinibuf   func()
	WaitKey      func() (uint32, bool)
	// Async prompt APIs (preferred on the main loop path).
	AskString    func(prompt, initial string, onDone func(string, model.PromptResult))
	AskStringCap func(prompt, initial string, capacity int, onDone func(string, model.PromptResult))
	AskFuzzy     func(prompt string, provider model.MbNameProviderFn, providerCtx any, providerCount uint, onDone func(string, model.PromptResult))
	AskFuzzyEx   func(prompt string, provider model.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter model.MbMatchFormatter, displayCtx any, onDone func(string, model.PromptResult))
	AskFilename  func(prompt, initial string, onDone func(string, model.PromptResult))
	AskChoose    func(prompt string, ctx any, labelFn model.MLChoiceLabelFn, count uint8, defaultIdx uint8, onDone func(int16))
}

var PackageHooks Hooks

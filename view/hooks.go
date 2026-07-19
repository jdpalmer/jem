package view

import (
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
)

type CommandFunc func(f bool, n int) bool

// Hooks are editor-owned callbacks view cannot import directly (cycle).
// Kill ring, key decode/meta, and paste live in view/model;
// git modeline stays hooked (tools↔view cycle).
// Macro play replies are consumed via model.TakeMacroPromptReply.
type Hooks struct {
	ApplyCtlxPrefix             func(second uint32) uint32
	RunCommandByName            func(name string) bool
	Abort                       func()
	GitLineDiff                 func(bp *buffer.Buffer, lineNumber uint) model.GitLineDiff
	GitModelineText             func(bp *buffer.Buffer) string
	MacroRecordMinibufferResult func(text []byte)
	CommandsProvider            func(ctx any, idx uint) []byte
	BuildCommandList            func() []string
	// BeginMinibuf / EndMinibuf / WaitKey route blocking prompts through the
	// editor listener stack and single event bus (view must not import editor).
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

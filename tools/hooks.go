package tools

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/minibuffer"
)

// Hooks are runtime-owned callbacks that tools cannot import directly (cycle).
type Hooks struct {
	VisitLocation func(path string, line, column int) bool
	SwitchBuffer  func(buf *buffer.Buffer)
	Abort         func()
	ReadKey       func() (uint32, bool)
	AskString     func(prompt, initial string, onDone func(string, minibuffer.PromptResult))
	AskStringCap  func(prompt, initial string, capacity int, onDone func(string, minibuffer.PromptResult))
	AskFuzzyEx    func(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount int, displayFormatter minibuffer.MbMatchFormatter, displayCtx any, onDone func(string, minibuffer.PromptResult))
}

var PackageHooks Hooks

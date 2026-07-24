package runtime

import (
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/minibuffer"
)

// PromptDone is called when an async minibuffer prompt finishes.
type PromptDone = func(text string, pr minibuffer.PromptResult)

// ChooseDone is called when an async choose menu finishes.
// sel ≥0 selected, -1 cancel, -2 abort.
type ChooseDone = func(sel int)

type textPrompt interface {
	HandleKey(k uint32) (done bool, text string, pr minibuffer.PromptResult)
	Close()
}

type promptListener struct {
	prompt textPrompt
	onDone PromptDone
}

func (l *promptListener) Handle(_ *ProcState, e event.Event) ListenerResult {
	ke, ok := e.(event.KeyEvent)
	if !ok {
		return PassThrough
	}
	done, text, pr := l.prompt.HandleKey(ke.Code)
	if !done {
		return Consumed
	}
	l.prompt.Close()
	if l.onDone != nil {
		l.onDone(text, pr)
	}
	return ConsumedAndPop
}

type chooseListener struct {
	prompt *display.ChoosePrompt
	onDone ChooseDone
}

func (l *chooseListener) Handle(_ *ProcState, e event.Event) ListenerResult {
	ke, ok := e.(event.KeyEvent)
	if !ok {
		return PassThrough
	}
	done, sel := l.prompt.HandleKey(ke.Code)
	if !done {
		return Consumed
	}
	l.prompt.Close()
	if l.onDone != nil {
		l.onDone(sel)
	}
	return ConsumedAndPop
}

// AskString pushes a string prompt listener. onDone runs on the next tick after answer.
func AskString(prompt, initial string, onDone PromptDone) {
	AskStringCap(prompt, initial, PatternCapacity, onDone)
}

// AskStringCap is AskString with an explicit capacity.
func AskStringCap(prompt, initial string, capacity int, onDone PromptDone) {
	if text, pr, played := TakeMacroPromptReply(); played {
		if onDone != nil {
			onDone(text, pr)
		}
		return
	}
	p := display.NewStringPrompt(prompt, initial, capacity)
	p.Open()
	PushListener(&promptListener{prompt: p, onDone: onDone})
}

// AskFuzzy pushes a fuzzy-list prompt listener.
func AskFuzzy(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount int, onDone PromptDone) {
	AskFuzzyEx(prompt, provider, providerCtx, providerCount, nil, nil, onDone)
}

// AskFuzzyEx is AskFuzzy with a custom match formatter.
func AskFuzzyEx(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount int, displayFormatter minibuffer.MbMatchFormatter, displayCtx any, onDone PromptDone) {
	if text, pr, played := TakeMacroPromptReply(); played {
		if onDone != nil {
			onDone(text, pr)
		}
		return
	}
	p := display.NewFuzzyPrompt(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx)
	p.Open()
	PushListener(&promptListener{prompt: p, onDone: onDone})
}

// AskFilename pushes a filename prompt listener.
func AskFilename(prompt, initial string, onDone PromptDone) {
	if text, pr, played := TakeMacroPromptReply(); played {
		if onDone != nil {
			onDone(text, pr)
		}
		return
	}
	p := display.NewFilenamePrompt(prompt, initial, 0)
	p.Open()
	PushListener(&promptListener{prompt: p, onDone: onDone})
}

// AskChoose pushes a horizontal choice menu listener.
func AskChoose(prompt string, ctx any, labelFn minibuffer.MLChoiceLabelFn, count int, defaultIdx int, onDone ChooseDone) {
	p := display.NewChoosePrompt(prompt, ctx, labelFn, count, defaultIdx)
	if p == nil {
		if onDone != nil {
			onDone(-1)
		}
		return
	}
	p.Open()
	PushListener(&chooseListener{prompt: p, onDone: onDone})
}

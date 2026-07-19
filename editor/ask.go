package editor

import (
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/view"
)

// PromptDone is called when an async minibuffer prompt finishes.
type PromptDone func(text string, pr model.PromptResult)

// ChooseDone is called when an async choose menu finishes.
// sel ≥0 selected, -1 cancel, -2 abort.
type ChooseDone func(sel int16)

type stringListener struct {
	prompt *view.StringPrompt
	onDone PromptDone
}

func (l *stringListener) Handle(_ *model.AppState, e event.Event) ListenerResult {
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

type fuzzyListener struct {
	prompt *view.FuzzyPrompt
	onDone PromptDone
}

func (l *fuzzyListener) Handle(_ *model.AppState, e event.Event) ListenerResult {
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

type filenameListener struct {
	prompt *view.FilenamePrompt
	onDone PromptDone
}

func (l *filenameListener) Handle(_ *model.AppState, e event.Event) ListenerResult {
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
	prompt *view.ChoosePrompt
	onDone ChooseDone
}

func (l *chooseListener) Handle(_ *model.AppState, e event.Event) ListenerResult {
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
	AskStringCap(prompt, initial, model.PatternCapacity, onDone)
}

// AskStringCap is AskString with an explicit capacity.
func AskStringCap(prompt, initial string, capacity int, onDone PromptDone) {
	if text, pr, played := model.TakeMacroPromptReply(); played {
		if onDone != nil {
			onDone(text, pr)
		}
		return
	}
	p := view.NewStringPrompt(prompt, initial, capacity)
	p.OpenAsync()
	PushListener(&stringListener{prompt: p, onDone: onDone})
}

// AskFuzzy pushes a fuzzy-list prompt listener.
func AskFuzzy(prompt string, provider model.MbNameProviderFn, providerCtx any, providerCount uint, onDone PromptDone) {
	AskFuzzyEx(prompt, provider, providerCtx, providerCount, nil, nil, onDone)
}

// AskFuzzyEx is AskFuzzy with a custom match formatter.
func AskFuzzyEx(prompt string, provider model.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter model.MbMatchFormatter, displayCtx any, onDone PromptDone) {
	if text, pr, played := model.TakeMacroPromptReply(); played {
		if onDone != nil {
			onDone(text, pr)
		}
		return
	}
	p := view.NewFuzzyPrompt(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx, model.PatternCapacity)
	p.OpenAsync()
	PushListener(&fuzzyListener{prompt: p, onDone: onDone})
}

// AskFilename pushes a filename prompt listener.
func AskFilename(prompt, initial string, onDone PromptDone) {
	if text, pr, played := model.TakeMacroPromptReply(); played {
		if onDone != nil {
			onDone(text, pr)
		}
		return
	}
	p := view.NewFilenamePrompt(prompt, initial, 0)
	p.OpenAsync()
	PushListener(&filenameListener{prompt: p, onDone: onDone})
}

// AskChoose pushes a horizontal choice menu listener.
func AskChoose(prompt string, ctx any, labelFn model.MLChoiceLabelFn, count uint8, defaultIdx uint8, onDone ChooseDone) {
	p := view.NewChoosePrompt(prompt, ctx, labelFn, count, defaultIdx)
	if p == nil {
		if onDone != nil {
			onDone(-1)
		}
		return
	}
	p.OpenAsync()
	PushListener(&chooseListener{prompt: p, onDone: onDone})
}

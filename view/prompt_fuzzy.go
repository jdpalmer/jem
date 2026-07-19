package view

import (
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/term"
)

// FuzzyPrompt is a live-filtering fuzzy list picker driven one key at a time.
type FuzzyPrompt struct {
	prompt           string
	provider         model.MbNameProviderFn
	providerCtx      any
	providerCount    uint
	displayFormatter model.MbMatchFormatter
	displayCtx       any
	state            model.MinibufferState
	sel              int
	matches          []uint
	fctx             *fuzzyMatchCtx
	async            bool
}

// NewFuzzyPrompt builds a fuzzy list prompt.
func NewFuzzyPrompt(prompt string, provider model.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter model.MbMatchFormatter, displayCtx any, capacity int) *FuzzyPrompt {
	if capacity <= 0 {
		capacity = model.PatternCapacity
	}
	p := &FuzzyPrompt{
		prompt:           prompt,
		provider:         provider,
		providerCtx:      providerCtx,
		providerCount:    providerCount,
		displayFormatter: displayFormatter,
		displayCtx:       displayCtx,
		state: model.MinibufferState{
			Prompt:     prompt,
			Text:       make([]byte, 0, capacity),
			Nbuf:       uint(capacity),
			HistoryPos: -1,
		},
		fctx: &fuzzyMatchCtx{
			provider:         provider,
			providerCtx:      providerCtx,
			displayFormatter: displayFormatter,
			displayCtx:       displayCtx,
		},
	}
	p.matches = fuzzyMatches(provider, providerCtx, providerCount, p.state.Text, fuzzyMaxMatches)
	return p
}

// OpenAsync shows the prompt for listener-driven use.
func (p *FuzzyPrompt) OpenAsync() {
	p.async = true
	ShowMinibuffer(&p.state)
	p.redraw()
}

// OpenBlocking shows the prompt with nested key capture.
func (p *FuzzyPrompt) OpenBlocking() {
	p.async = false
	ActivateMinibuffer(&p.state)
	p.redraw()
}

// Close tears down the prompt UI.
func (p *FuzzyPrompt) Close() {
	if p.async {
		HideMinibuffer()
	} else {
		DeactivateMinibuffer()
	}
	model.HideMatchWindow()
	DisplayUpdate()
}

func (p *FuzzyPrompt) redraw() {
	fuzzyListRedraw(p.prompt, &p.state, p.fctx, p.matches, p.sel)
}

// HandleKey applies one key. On success, text is the selected label.
func (p *FuzzyPrompt) HandleKey(k uint32) (done bool, text string, pr model.PromptResult) {
	if IsPasteRedrawKey(k) {
		DisplayUpdate()
		p.redraw()
		return false, "", 0
	}

	changed := false

	switch {
	case k == term.KeyEnter || k == '\r' || k == '\n' || k == (term.CTL|'M') || k == (term.CTL|'J'):
		if len(p.matches) > 0 && p.sel >= 0 && p.sel < len(p.matches) {
			label := p.provider(p.providerCtx, p.matches[p.sel])
			if label != nil {
				if PackageHooks.MacroRecordMinibufferResult != nil {
					PackageHooks.MacroRecordMinibufferResult(label)
				}
				MBClear()
				return true, string(label), model.PromptResultYes
			}
		}
		MBClear()
		return true, "", model.PromptResultAbort

	case k == (term.CTL|'G') || k == 0x07 || k == 0x1B:
		MBClear()
		return true, "", model.PromptResultAbort

	case k == term.KeyUp || k == (term.CTL|'P'):
		if len(p.matches) == 0 {
			term.Beep()
		} else {
			p.sel = (p.sel + len(p.matches) - 1) % len(p.matches)
		}
	case k == term.KeyDown || k == (term.CTL|'N'):
		if len(p.matches) == 0 {
			term.Beep()
		} else {
			p.sel = (p.sel + 1) % len(p.matches)
		}

	case k == (term.CTL|'A') || k == term.KeyHome:
		if !p.state.GotoBol() {
			term.Beep()
		}
	case k == (term.CTL|'E') || k == term.KeyEnd:
		if !p.state.GotoEol() {
			term.Beep()
		}
	case k == (term.CTL|'B') || k == term.KeyLeft:
		if !p.state.BackwardChar() {
			term.Beep()
		}
	case k == (term.CTL|'F') || k == term.KeyRight:
		if !p.state.ForwardChar() {
			term.Beep()
		}
	case k == (term.META|'B') || k == (term.SHIFT|term.KeyLeft):
		if !p.state.BackwardWord() {
			term.Beep()
		}
	case k == (term.META|'F') || k == (term.SHIFT|term.KeyRight):
		if !p.state.ForwardWord() {
			term.Beep()
		}

	case k == 0x7F || k == (term.CTL|'H'):
		changed = p.state.DeleteBackward()
		if !changed {
			term.Beep()
		}
	case k == (term.CTL | 'D'):
		changed = p.state.DeleteForward()
		if !changed {
			term.Beep()
		}
	case k == (term.CTL | 'U'):
		changed = p.state.ClearText()
		if !changed {
			term.Beep()
		}

	default:
		if k < term.UnicodeLimit && k >= 0x20 && (k&term.KeyMask) == 0 {
			if p.state.InsertChar(rune(k)) {
				changed = true
			} else {
				term.Beep()
			}
		} else {
			term.Beep()
		}
	}

	if changed {
		p.matches = fuzzyMatches(p.provider, p.providerCtx, p.providerCount, p.state.Text, fuzzyMaxMatches)
		p.sel = 0
	}
	p.redraw()
	return false, "", 0
}

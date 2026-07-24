package display

import (
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/window"
)

// FuzzyPrompt is a live-filtering fuzzy list picker driven one key at a time.
type FuzzyPrompt struct {
	prompt           string
	provider         minibuffer.MbNameProviderFn
	providerCtx      any
	providerCount    int
	displayFormatter minibuffer.MbMatchFormatter
	displayCtx       any
	state            minibuffer.MinibufferState
	sel              int
	matches          []int
	fctx             *fuzzyMatchCtx
}

// NewFuzzyPrompt builds a fuzzy list prompt.
func NewFuzzyPrompt(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount int, displayFormatter minibuffer.MbMatchFormatter, displayCtx any) *FuzzyPrompt {
	p := &FuzzyPrompt{
		prompt:           prompt,
		provider:         provider,
		providerCtx:      providerCtx,
		providerCount:    providerCount,
		displayFormatter: displayFormatter,
		displayCtx:       displayCtx,
		state: minibuffer.MinibufferState{
			Text:       make([]byte, 0, PatternCapacity),
			Nbuf:       PatternCapacity,
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

// Open shows the prompt for listener-driven use.
func (p *FuzzyPrompt) Open() {
	minibuffer.Active = &p.state
	p.redraw()
}

// Close tears down the prompt UI.
func (p *FuzzyPrompt) Close() {
	minibuffer.Active = nil
	window.HideMatchWindow()
	DisplayUpdate()
}

func (p *FuzzyPrompt) redraw() {
	fuzzyListRedraw(p.prompt, &p.state, p.fctx, p.matches, p.sel)
}

// HandleKey applies one key. On success, text is the selected label.
func (p *FuzzyPrompt) HandleKey(k uint32) (done bool, text string, pr minibuffer.PromptResult) {
	changed := false

	switch {
	case k == term.KeyEnter || k == '\r' || k == '\n' || k == (term.CTL|'M') || k == (term.CTL|'J'):
		if len(p.matches) > 0 && p.sel >= 0 && p.sel < len(p.matches) {
			label := p.provider(p.providerCtx, p.matches[p.sel])
			if label != nil {
				maybeRecordMinibufferResult(label)
				MBClear()
				return true, string(label), minibuffer.PromptResultYes
			}
		}
		MBClear()
		return true, "", minibuffer.PromptResultAbort

	case k == (term.CTL|'G') || k == 0x07 || k == 0x1B:
		MBClear()
		return true, "", minibuffer.PromptResultAbort

	case k == term.KeyUp || k == (term.CTL|'P') || k == term.KeyDown || k == (term.CTL|'N'):
		if len(p.matches) == 0 {
			term.Beep()
		} else {
			delta := 1
			if k == term.KeyUp || k == (term.CTL|'P') {
				delta = -1
			}
			p.sel = (p.sel + len(p.matches) + delta) % len(p.matches)
		}

	default:
		var handled bool
		handled, changed = promptLineEditKey(&p.state, k)
		if !handled {
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

package display

import (
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/term"
)

// ChoosePrompt is a horizontal choice menu driven one key at a time.
// Selected index is ≥0 on confirm; -1 cancel; -2 abort (C-g).
type ChoosePrompt struct {
	prompt      string
	ctx         any
	labelFn     minibuffer.MLChoiceLabelFn
	count       int
	selected    int
	promptWidth int
	avail       int
	state       minibuffer.MinibufferState
}

// NewChoosePrompt builds a choice menu. defaultIdx is clamped into range.
func NewChoosePrompt(prompt string, ctx any, labelFn minibuffer.MLChoiceLabelFn, count int, defaultIdx int) *ChoosePrompt {
	n := int(count)
	if n <= 0 {
		return nil
	}
	selected := int(defaultIdx)
	if selected >= n {
		selected = 0
	}
	promptWidth := displayWidthBytes([]byte(prompt), len(prompt))
	avail := term.Cols() - 1 - promptWidth
	if avail < 1 {
		avail = 1
	}
	return &ChoosePrompt{
		prompt:      prompt,
		ctx:         ctx,
		labelFn:     labelFn,
		count:       n,
		selected:    selected,
		promptWidth: promptWidth,
		avail:       avail,
	}
}

// Open shows the menu for listener-driven use.
func (p *ChoosePrompt) Open() {
	minibuffer.Active = &p.state
	p.redraw()
}

// Close tears down the menu UI.
func (p *ChoosePrompt) Close() {
	minibuffer.Active = nil
}

func (p *ChoosePrompt) redraw() {
	start, end := mlChoiceWindow(p.ctx, p.labelFn, p.count, p.selected, p.avail)
	mlChoiceRender(p.prompt, p.ctx, p.labelFn, p.count, start, end, p.selected)
}

// HandleKey applies one key. When done, sel is the result (-1/-2/≥0).
func (p *ChoosePrompt) HandleKey(k uint32) (done bool, sel int) {
	switch {
	case k == 0x0D || k == 0x0A || k == term.KeyEnter || k == (term.CTL|'M') || k == (term.CTL|'J'):
		MBClear()
		return true, p.selected
	case k == 0x07 || k == (term.CTL|'G') || k == 0x1B:
		MBClear()
		return true, -2
	case k == term.KeyLeft || k == (term.CTL|'B') || k == term.KeyUp:
		if p.selected > 0 {
			p.selected--
		}
	case k == term.KeyRight || k == (term.CTL|'F') || k == term.KeyDown:
		if p.selected < p.count-1 {
			p.selected++
		}
	default:
		if k&0x20000000 != 0 {
			return false, 0
		}
	}
	p.redraw()
	return false, 0
}

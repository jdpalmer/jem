package display

import (
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/term"
)

// StringPrompt is a line-edit minibuffer session driven one key at a time.
type StringPrompt struct {
	prompt string
	state  minibuffer.MinibufferState
}

// NewStringPrompt builds a string prompt. capacity defaults to PatternCapacity.
func NewStringPrompt(prompt, initial string, capacity int) *StringPrompt {
	if capacity <= 0 {
		capacity = PatternCapacity
	}
	p := &StringPrompt{
		prompt: prompt,
		state: minibuffer.MinibufferState{
			Text:       make([]byte, 0, capacity),
			Nbuf:       capacity,
			HistoryPos: -1,
		},
	}
	if initial != "" {
		p.state.SetText([]byte(initial))
	}
	return p
}

// Open shows the prompt for listener-driven use.
func (p *StringPrompt) Open() {
	minibuffer.Active = &p.state
	p.redraw()
}

// Close tears down the prompt UI.
func (p *StringPrompt) Close() {
	minibuffer.Active = nil
}

func (p *StringPrompt) redraw() {
	MBWritePrompt(p.prompt, p.state.Text, p.state.CursorPos)
	DisplayUpdate()
}

// HandleKey applies one key. When done is true, text/pr are the result and the
// caller must Close the prompt.
func (p *StringPrompt) HandleKey(k uint32) (done bool, text string, pr minibuffer.PromptResult) {
	switch {
	case k == term.KeyEnter || k == '\r' || k == '\n' || k == (term.CTL|'M') || k == (term.CTL|'J'):
		MBHistoryAdd(string(p.state.Text))
		if PackageHooks.MacroRecordMinibufferResult != nil {
			PackageHooks.MacroRecordMinibufferResult(p.state.Text)
		}
		MBClear()
		return true, string(p.state.Text), minibuffer.PromptResultYes

	case k == (term.CTL|'G') || k == 0x07 || k == 0x1B:
		MBWrite("^G")
		MBClear()
		return true, "", minibuffer.PromptResultAbort

	case k == (term.CTL|'P') || k == term.KeyUp:
		if !p.state.StepHistory(-1) {
			term.Beep()
		}
	case k == (term.CTL|'N') || k == term.KeyDown:
		if !p.state.StepHistory(1) {
			term.Beep()
		}
	default:
		if k&0x20000000 != 0 {
			return false, "", 0
		}
		if handled, _ := promptLineEditKey(&p.state, k); !handled {
			term.Beep()
		}
	}

	p.redraw()
	return false, "", 0
}

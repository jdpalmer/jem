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
			Prompt:     prompt,
			Text:       make([]byte, 0, capacity),
			Nbuf:       uint(capacity),
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
	MBWritePrompt(p.prompt, p.state.Text, int(p.state.CursorPos))
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
		if !p.state.DeleteBackward() {
			term.Beep()
		}
	case k == (term.CTL|'D') || k == term.KeyDelete:
		if !p.state.DeleteForward() {
			term.Beep()
		}
	case k == (term.CTL | 'U'):
		if !p.state.ClearText() {
			term.Beep()
		}
	case k == (term.CTL | 'K'):
		if !p.state.Kill() {
			term.Beep()
		}
	case k == (term.CTL | 'Y'):
		if !p.state.Yank() {
			term.Beep()
		}
	case k == (term.META | 'D'):
		if !p.state.DeleteWordForward() {
			term.Beep()
		}
	case k == (term.META|'H') || k == (term.META|0x7F):
		if !p.state.DeleteWordBackward() {
			term.Beep()
		}
	default:
		if k&0x20000000 != 0 {
			return false, "", 0
		}
		if k < term.UnicodeLimit && k >= 0x20 && (k&term.KeyMask) == 0 {
			if !p.state.InsertChar(rune(k)) {
				term.Beep()
			}
		} else {
			term.Beep()
		}
	}

	p.redraw()
	return false, "", 0
}

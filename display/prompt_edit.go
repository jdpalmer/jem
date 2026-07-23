package display

import (
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/term"
)

// promptLineEditKey handles motion and text-edit keys shared by line-edit prompts.
// Returns handled=true if the key was consumed; changed=true when text was modified.
func promptLineEditKey(state *minibuffer.MinibufferState, k uint32) (handled, changed bool) {
	beepUnless := func(ok bool) {
		if !ok {
			term.Beep()
		}
	}
	switch {
	case k == (term.CTL|'A') || k == term.KeyHome:
		beepUnless(state.GotoBol())
		return true, false
	case k == (term.CTL|'E') || k == term.KeyEnd:
		beepUnless(state.GotoEol())
		return true, false
	case k == (term.CTL|'B') || k == term.KeyLeft:
		beepUnless(state.BackwardChar())
		return true, false
	case k == (term.CTL|'F') || k == term.KeyRight:
		beepUnless(state.ForwardChar())
		return true, false
	case k == (term.META|'B') || k == (term.SHIFT|term.KeyLeft):
		beepUnless(state.BackwardWord())
		return true, false
	case k == (term.META|'F') || k == (term.SHIFT|term.KeyRight):
		beepUnless(state.ForwardWord())
		return true, false
	case k == 0x7F || k == (term.CTL|'H'):
		changed = state.DeleteBackward()
		beepUnless(changed)
		return true, changed
	case k == (term.CTL|'D') || k == term.KeyDelete:
		changed = state.DeleteForward()
		beepUnless(changed)
		return true, changed
	case k == (term.CTL | 'U'):
		changed = state.ClearText()
		beepUnless(changed)
		return true, changed
	case k == (term.CTL | 'K'):
		changed = state.Kill()
		beepUnless(changed)
		return true, changed
	case k == (term.CTL | 'Y'):
		changed = state.Yank()
		beepUnless(changed)
		return true, changed
	case k == (term.META | 'D'):
		changed = state.DeleteWordForward()
		beepUnless(changed)
		return true, changed
	case k == (term.META|'H') || k == (term.META|0x7F):
		changed = state.DeleteWordBackward()
		beepUnless(changed)
		return true, changed
	default:
		if k < term.UnicodeLimit && k >= 0x20 && (k&term.KeyMask) == 0 {
			changed = state.InsertChar(rune(k))
			beepUnless(changed)
			return true, changed
		}
		return false, false
	}
}

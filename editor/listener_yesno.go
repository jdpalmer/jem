package editor

import (
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/view"
)

// yesNoListener consumes a single y/n/C-g key for a prompt, then pops.
type yesNoListener struct {
	prompt  string
	onYes   func()
	onNo    func()
	onAbort func()
}

func (l *yesNoListener) Handle(s *model.AppState, e event.Event) ListenerResult {
	ke, ok := e.(event.KeyEvent)
	if !ok {
		return PassThrough
	}
	code := ke.Code
	// Normalize letter keys.
	if code < 128 {
		c := byte(code)
		if c >= 'A' && c <= 'Z' {
			c = c - 'A' + 'a'
		}
		switch c {
		case 'y':
			view.MBClear()
			if l.onYes != nil {
				l.onYes()
			}
			return ConsumedAndPop
		case 'n':
			view.MBClear()
			if l.onNo != nil {
				l.onNo()
			}
			return ConsumedAndPop
		}
	}
	if code == (term.CTL|'G') || code == 0x1B {
		view.MBClear()
		if l.onAbort != nil {
			l.onAbort()
		} else if l.onNo != nil {
			l.onNo()
		}
		return ConsumedAndPop
	}
	// Ignore other keys while prompting.
	view.MBWrite("%s (y/n)", l.prompt)
	return Consumed
}

// AskYesNo pushes a yes/no listener and shows the prompt. Continuations run
// when the user answers (next tick). Replaces blocking view.MBYesNo for loop paths.
func AskYesNo(prompt string, onYes, onNo func()) {
	PushListener(&yesNoListener{
		prompt: prompt,
		onYes:  onYes,
		onNo:   onNo,
	})
	view.MBWrite("%s (y/n)", prompt)
}

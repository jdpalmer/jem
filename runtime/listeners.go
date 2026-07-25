package runtime

import (
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/search"
	"github.com/jdpalmer/jem/term"
)

// quoteListener waits for the next key after C-q and inserts it literally.
type quoteListener struct {
	n int // insert count; 0 means consume and discard
}

func (l *quoteListener) Handle(_ *ProcState, e event.Event) ListenerResult {
	ke, ok := e.(event.KeyEvent)
	if !ok {
		return PassThrough
	}
	if State.IsRecording() && l.n != 0 {
		_ = macroRecordKey(int(ke.Code), false, 1)
	}
	if l.n != 0 {
		_ = quoteInsertKey(ke.Code, l.n)
	}
	return ConsumedAndPop
}

// beginQuote installs a one-key quote listener (interactive path).
func beginQuote(n int) {
	PushListener(&quoteListener{n: n})
}

// keySessionListener drives a search.KeySession from the event bus.
type keySessionListener struct {
	sess search.KeySession
}

func (l *keySessionListener) Handle(_ *ProcState, e event.Event) ListenerResult {
	ke, ok := e.(event.KeyEvent)
	if !ok {
		return PassThrough
	}
	if l.sess.HandleKey(ke.Code) {
		l.sess.Close()
		return ConsumedAndPop
	}
	return Consumed
}

// PushKeySession opens a multi-key modal and installs it on the listener stack.
func PushKeySession(s search.KeySession) {
	if s == nil {
		return
	}
	if s.Open() {
		s.Close()
		return
	}
	PushListener(&keySessionListener{sess: s})
}

// yesNoListener consumes a single y/n/C-g/Esc key for a prompt, then pops.
type yesNoListener struct {
	prompt  string
	mbState minibuffer.MinibufferState
	onYes   func()
	onNo    func()
	onAbort func()
}

func (l *yesNoListener) Handle(s *ProcState, e event.Event) ListenerResult {
	ke, ok := e.(event.KeyEvent)
	if !ok {
		return PassThrough
	}
	code := ke.Code
	finish := func(fn func()) ListenerResult {
		minibuffer.Active = nil
		display.MBClear()
		if fn != nil {
			fn()
		}
		return ConsumedAndPop
	}
	// Normalize letter keys.
	if code < 128 {
		c := byte(code)
		if c >= 'A' && c <= 'Z' {
			c = c - 'A' + 'a'
		}
		switch c {
		case 'y':
			return finish(l.onYes)
		case 'n':
			return finish(l.onNo)
		}
	}
	if code == (term.CTL|'G') || code == 0x1B {
		if l.onAbort != nil {
			return finish(l.onAbort)
		}
		return finish(l.onNo)
	}
	// Ignore other keys while prompting.
	display.MBWrite("%s (y/n)", l.prompt)
	return Consumed
}

// AskYesNo pushes a yes/no listener and shows the prompt. Continuations run
// when the user answers (next tick).
func AskYesNo(prompt string, onYes, onNo func()) {
	l := &yesNoListener{
		prompt: prompt,
		onYes:  onYes,
		onNo:   onNo,
	}
	minibuffer.Active = &l.mbState
	PushListener(l)
	display.MBWrite("%s (y/n)", prompt)
}

// universalArgListener collects a C-u numeric argument, then dispatches the
// terminating key with (f=true, n=arg) and pops.
type universalArgListener struct {
	n     int
	mflag int // 0 = none, 1 = digits started, -1 = negative
}

func (l *universalArgListener) Handle(_ *ProcState, e event.Event) ListenerResult {
	ke, ok := e.(event.KeyEvent)
	if !ok {
		return PassThrough
	}
	next := ke.Code
	if !((next >= '0' && next <= '9') || next == (term.CTL|'U') || next == '-') {
		n := l.finalize()
		runBoundKey(next, true, n)
		return ConsumedAndPop
	}
	if next == (term.CTL | 'U') {
		l.n *= 4
	} else if next == '-' {
		if l.mflag != 0 {
			n := l.finalize()
			runBoundKey(next, true, n)
			return ConsumedAndPop
		}
		l.n = 0
		l.mflag = -1
	} else {
		if l.mflag == 0 {
			l.n = 0
			l.mflag = 1
		}
		l.n = 10*l.n + int(next-'0')
	}
	display.MBWrite("Arg: %d", l.displayN())
	return Consumed
}

func (l *universalArgListener) displayN() int {
	if l.mflag < 0 {
		if l.n == 0 {
			return -1
		}
		return -l.n
	}
	return l.n
}

func (l *universalArgListener) finalize() int {
	n := l.n
	if l.mflag == -1 {
		if n == 0 {
			n++
		}
		n = -n
	}
	return n
}

// beginUniversalArg pushes the C-u argument collector.
func beginUniversalArg() {
	PushListener(&universalArgListener{n: 4})
	display.MBWrite("Arg: 4")
}

package runtime

import (
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/term"
)

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

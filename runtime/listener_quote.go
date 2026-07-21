package runtime

import (
	"github.com/jdpalmer/jem/event"
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

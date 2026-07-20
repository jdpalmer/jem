package runtime

import (
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/search"
)

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

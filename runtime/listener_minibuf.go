package runtime

import (
	"github.com/jdpalmer/jem/event"
)

// keyCaptureListener sits on the stack while a blocking minibuffer (or C-u)
// needs keys from the single event bus. KeyEvents are Consumed and queued for
// WaitKey; other events PassThrough so WaitKey can defer them.
type keyCaptureListener struct {
	keys chan uint32
}

func (l *keyCaptureListener) Handle(_ *ProcState, e event.Event) ListenerResult {
	ke, ok := e.(event.KeyEvent)
	if !ok {
		return PassThrough
	}
	l.keys <- ke.Code
	return Consumed
}

var (
	minibufCaptureDepth int
	deferredEvents      []event.Event
)

// BeginMinibuf pushes a key-capture listener (once per nested depth) and
// discards any KeyEvents already queued on the bus.
func BeginMinibuf() {
	if minibufCaptureDepth == 0 {
		drainQueuedKeys()
		PushListener(&keyCaptureListener{keys: make(chan uint32, 64)})
	}
	minibufCaptureDepth++
}

// EndMinibuf pops the capture listener when the outermost minibuf exits and
// re-enqueues any non-key events deferred during WaitKey.
func EndMinibuf() {
	if minibufCaptureDepth == 0 {
		return
	}
	minibufCaptureDepth--
	if minibufCaptureDepth > 0 {
		return
	}
	if n := len(listenerStack); n > 0 {
		if _, ok := listenerStack[n-1].(*keyCaptureListener); ok {
			PopListener()
		}
	}
	flushDeferredEvents()
}

// WaitKey reads the next key from the event bus, running the listener stack
// first. Used by blocking minibuffer loops and C-u argument collection while
// nested inside Handle (the outer loop is not selecting).
func WaitKey() (uint32, bool) {
	for {
		if n := len(listenerStack); n > 0 {
			if cap, ok := listenerStack[n-1].(*keyCaptureListener); ok {
				select {
				case k := <-cap.keys:
					return k, true
				default:
				}
			}
		}

		e, ok := <-event.Chan()
		if !ok {
			return 0, false
		}

		if n := len(listenerStack); n > 0 {
			top := listenerStack[n-1]
			switch top.Handle(State, e) {
			case ConsumedAndPop:
				listenerStack = listenerStack[:n-1]
				continue
			case Consumed:
				if cap, ok := top.(*keyCaptureListener); ok {
					k := <-cap.keys
					return k, true
				}
				continue
			case PassThrough:
				// fall through
			}
		}

		if ke, ok := e.(event.KeyEvent); ok {
			return ke.Code, true
		}
		deferredEvents = append(deferredEvents, e)
	}
}

func drainQueuedKeys() {
	var keep []event.Event
	for {
		select {
		case e := <-event.Chan():
			if _, isKey := e.(event.KeyEvent); isKey {
				continue
			}
			keep = append(keep, e)
		default:
			for _, e := range keep {
				event.Enqueue(e)
			}
			return
		}
	}
}

func flushDeferredEvents() {
	for _, e := range deferredEvents {
		event.Enqueue(e)
	}
	deferredEvents = nil
}

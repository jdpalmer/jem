package event

// bus is the main-loop inbound queue. Producers only Enqueue; the editor loop receives.
var bus = make(chan Event, 64)

// Enqueue posts an event for a later tick. Prefer blocking over drop for correctness.
func Enqueue(e Event) {
	if e == nil {
		return
	}
	bus <- e
}

// Chan returns the receive-only event bus for the editor loop.
func Chan() <-chan Event {
	return bus
}

// DrainForTest discards queued events (test helpers only).
func DrainForTest() {
	for {
		select {
		case <-bus:
		default:
			return
		}
	}
}

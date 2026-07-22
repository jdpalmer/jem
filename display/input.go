package display

// input.go — coordinates the background key reader with shell spawn.

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/term"
)

var (
	termInputFreezeReq   = make(chan struct{}, 1)
	termInputFreezeAck   = make(chan struct{}, 1)
	termKeyReaderWG      sync.WaitGroup
	termKeyReaderRunning int32
)

// TermFreezeInput stops the key reader goroutine before handing stdin to a shell.
// Returns false if the reader does not stop within the timeout.
func TermFreezeInput() bool {
	if atomic.LoadInt32(&termKeyReaderRunning) == 0 {
		return true
	}

	// Drain any stale ack from a previous freeze cycle.
	select {
	case <-termInputFreezeAck:
	default:
	}
	// Drain any undelivered freeze request.
	select {
	case <-termInputFreezeReq:
	default:
	}

	select {
	case termInputFreezeReq <- struct{}{}:
	default:
	}

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&termKeyReaderRunning) != 0 {
		select {
		case <-termInputFreezeAck:
		default:
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
	return true
}

// TermThawInput starts a fresh key reader goroutine after a shell returns.
func TermThawInput() {
	term.ResetReader()
	select {
	case <-termInputFreezeReq:
	default:
	}
	StartKeyReader()
}

// StartKeyReader launches the background key reader goroutine if not already running.
func StartKeyReader() {
	if !atomic.CompareAndSwapInt32(&termKeyReaderRunning, 0, 1) {
		return
	}
	termKeyReaderWG.Add(1)
	go runKeyReader()
}

func deliverDecodedKey(k uint32) {
	event.Enqueue(event.KeyEvent{Code: k})
}

func decodeAndDeliver(raw uint32) {
	k := DecodeKeyChar(raw, false)
	if escapePending {
		escapePending = false
		deliverDecodedKey(ApplyMetaPrefixToKey(k))
		return
	}
	if k == 0x1B {
		escapePending = true
		return
	}
	if ctlxPending {
		ctlxPending = false
		if PackageHooks.ApplyCtlxPrefix == nil {
			return
		}
		deliverDecodedKey(PackageHooks.ApplyCtlxPrefix(k))
		return
	}
	if k == (term.CTL | 'X') {
		ctlxPending = true
		return
	}
	deliverDecodedKey(k)
}

var (
	escapePending bool
	ctlxPending   bool
)

// runKeyReader is the background input loop for the runtime.
func runKeyReader() {
	defer func() {
		atomic.StoreInt32(&termKeyReaderRunning, 0)
		termKeyReaderWG.Done()
	}()

	handleKey := func(k uint32, ok bool) bool {
		if !ok {
			return false
		}
		if k == term.KeyPasteComplete {
			// Paste bytes are already on the bus via PasteEvent (OnPaste).
			return true
		}
		decodeAndDeliver(k)
		return true
	}

	for {
		select {
		case <-termInputFreezeReq:
			termInputFreezeAck <- struct{}{}
			return
		default:
		}

		// Drain any bytes already buffered (burst scroll / paste).
		for {
			k, ok := term.TryReadKey(0)
			if !handleKey(k, ok) || k == 0 {
				break
			}
		}

		select {
		case <-termInputFreezeReq:
			termInputFreezeAck <- struct{}{}
			return
		default:
		}

		k, ok := term.TryReadKey(50 * time.Millisecond)
		handleKey(k, ok)
	}
}

// pasteRepaintPending is set when a bracketed paste was applied this cycle so
// DisplayUpdate can nudge terminals (e.g. iTerm2) that defer painting
// already-flushed output until another input round-trip occurs.
var pasteRepaintPending bool

// NotePasteApplied marks that paste was applied on this tick (repaint nudge).
func NotePasteApplied() {
	pasteRepaintPending = true
}

package ui

// input.go — coordinates the background key reader with shell spawn.

import (
	"github.com/jdpalmer/jem/app"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jdpalmer/jem/term"
)

var (
	termInputFreezeReq   = make(chan struct{}, 1)
	termInputFreezeAck   = make(chan struct{}, 1)
	termKeyReaderWG      sync.WaitGroup
	termKeyReaderRunning int32
	pendingPasteCh       chan []byte
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
	startKeyReader()
}

func startKeyReader() {
	if !atomic.CompareAndSwapInt32(&termKeyReaderRunning, 0, 1) {
		return
	}
	termKeyReaderWG.Add(1)
	go runKeyReader()
}

func deliverDecodedKey(k uint32) {
	if app.State.ActiveMinibuffer != nil || app.State.Dispatching {
		select {
		case GlobalMinibufKeyCh <- k:
		default:
		}
		return
	}
	select {
	case GlobalKeyCh <- k:
	default:
	}
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

// runKeyReader is the background input loop for the editor.
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
			RequestDisplayRefresh()
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

// pasteRedrawKey is injected on the key channel after bracketed paste so the
// waiting input loop runs DisplayUpdate (paste has no real key event).
const pasteRedrawKey uint32 = 0x00FF0001

func isPasteRedrawKey(k uint32) bool {
	return k == pasteRedrawKey
}

// RequestDisplayRefresh wakes the main or minibuffer input loop to redraw.
func RequestDisplayRefresh() {
	var ch chan uint32
	if app.State.ActiveMinibuffer != nil {
		ch = GlobalMinibufKeyCh
	} else {
		ch = GlobalKeyCh
	}
	if ch == nil {
		return
	}
	ch <- pasteRedrawKey
}

// queuePaste stores a bracketed-paste payload for application on the main thread.
func queuePaste(data []byte) {
	if len(data) == 0 || pendingPasteCh == nil {
		return
	}
	p := append([]byte(nil), data...)
	select {
	case pendingPasteCh <- p:
	default:
		select {
		case <-pendingPasteCh:
		default:
		}
		pendingPasteCh <- p
	}
}

// pasteRepaintPending is set when a bracketed paste was applied this cycle so
// DisplayUpdate can nudge terminals (e.g. iTerm2) that defer painting
// already-flushed output until another input round-trip occurs.
var pasteRepaintPending bool

// applyPendingPaste inserts any queued bracketed paste on the main thread.
// Called from DisplayUpdate before painting (mirrors C editor_insert_paste).
func applyPendingPaste() {
	for {
		select {
		case data := <-pendingPasteCh:
			var ok bool
			if app.State.ActiveMinibuffer != nil {
				ok = editorMinibufferPaste(data)
			} else {
				ok = editorInsertPaste(data)
			}
			if ok {
				markPasteDirty()
				pasteRepaintPending = true
			}
		default:
			return
		}
	}
}

func markPasteDirty() {
	if app.State.ActiveMinibuffer != nil {
		return
	}
	app.State.ScreenDirty = true
	for i := 0; i < int(len(app.State.WINDOWS)); i++ {
		wp := app.State.WINDOWS[i]
		if wp != nil {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
	}
}

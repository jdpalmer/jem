package runtime

import (
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
)

// runBoundKey records a macro step (if any) and Execute's the key with arg f/n.
func runBoundKey(k uint32, f bool, n int) {
	if k == 0x03 {
		return
	}
	State.Dispatching = true
	if State.IsRecording() {
		if !macroRecordKey(int(k), f, n) {
			State.Dispatching = false
			return
		}
	}
	_ = Execute(int(k), f, n)
	State.Dispatching = false
	_ = afterKeyCommand()
}

// afterKeyCommand runs post-command bookkeeping. Returns false when the editor
// should exit (quit with no unsaved buffers).
func afterKeyCommand() bool {
	if !display.Active.MessagePresent {
		tools.MaybeShowCallHint()
	}
	if Current == nil || !Current.QuitRequested {
		return true
	}
	if anyUnsavedBuffers() {
		Current.QuitRequested = false
		AskYesNo("Quit with unsaved buffers?", func() {
			Current.QuitRequested = true
			event.Enqueue(event.QuitEvent{Force: true})
		}, nil)
		return true
	}
	event.Enqueue(event.QuitEvent{Force: true})
	return false
}

// handleEditorKey dispatches one window key. Returns false when the loop should exit.
func handleEditorKey(k uint32) bool {
	if display.IsPasteRedrawKey(k) {
		return true
	}
	if k == term.MouseWheelDown {
		ApplyWheelTicks(1)
		return true
	}
	if k == term.MouseWheelUp {
		ApplyWheelTicks(-1)
		return true
	}
	if display.Active.MessagePresent {
		display.MBClear()
	}
	if k == 0x03 { // Ctrl-C
		if anyUnsavedBuffers() {
			AskYesNo("Quit with unsaved buffers?", func() {
				event.Enqueue(event.QuitEvent{Force: true})
			}, nil)
			return true
		}
		return false
	}
	if k == (term.CTL | 'U') {
		beginUniversalArg()
		return true
	}
	runBoundKey(k, false, 1)
	return true
}

func handleCommandEvent(ev event.CommandEvent) bool {
	if ev.Name == "" {
		return true
	}
	cmd := commandByName(ev.Name)
	if cmd == nil || cmd.Fn == nil {
		display.MBWrite("[unknown command: %s]", ev.Name)
		return true
	}
	n := ev.N
	if n == 0 && !ev.F {
		n = 1
	}
	State.Dispatching = true
	_ = cmd.Fn(ev.F, n)
	State.Dispatching = false
	_ = afterKeyCommand()
	return true
}

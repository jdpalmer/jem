package runtime

import (
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
	"github.com/jdpalmer/jem/window"
)

// ListenerResult is how a stacked listener responds to an event.
type ListenerResult int

const (
	// PassThrough means the listener does not handle this event.
	PassThrough ListenerResult = iota
	// Consumed means the event was handled; keep the listener installed.
	Consumed
	// ConsumedAndPop means the event was handled; uninstall the listener.
	ConsumedAndPop
)

// Listener is a temporary top-of-stack event handler (prompts, menus, …).
type Listener interface {
	Handle(s *ProcState, e event.Event) ListenerResult
}

var listenerStack []Listener

// PushListener installs a listener that receives events before window dispatch.
func PushListener(l Listener) {
	listenerStack = append(listenerStack, l)
}

func clearListeners() {
	listenerStack = nil
}

// Handle applies one event on a tick. Returns false when the process should exit.
// Follow-up work must Enqueue, not call Handle synchronously.
func Handle(s *ProcState, e event.Event) bool {
	if s == nil {
		s = State
	}
	if n := len(listenerStack); n > 0 {
		top := listenerStack[n-1]
		switch top.Handle(s, e) {
		case ConsumedAndPop:
			listenerStack = listenerStack[:n-1]
			return true
		case Consumed:
			return true
		case PassThrough:
			// fall through to default dispatch
		}
	}

	switch ev := e.(type) {
	case event.KeyEvent:
		return handleEditorKey(ev.Code)
	case event.CommandEvent:
		return handleCommandEvent(ev)
	case event.PasteEvent:
		return handlePasteEvent(ev)
	case event.MouseEvent:
		display.Active.Mouse.Col = ev.Col
		display.Active.Mouse.Row = ev.Row
		return true
	case event.ResumeEvent:
		if term.RefreshSize() {
			display.DisplayInitHeadless(term.Rows(), term.Cols())
		}
		return true
	case event.PromptReplyEvent:
		if State.IsRecording() {
			_ = macroAppend(ev)
		}
		return true
	case event.QuitEvent:
		return handleQuitEvent(s, ev)
	case event.JobDoneEvent:
		return handleJobDoneEvent(ev)
	default:
		return true
	}
}

func handlePasteEvent(ev event.PasteEvent) bool {
	if len(ev.Data) == 0 {
		return true
	}
	applied := false
	if minibuffer.Active != nil {
		applied = minibuffer.InsertPaste(ev.Data)
	} else if err := window.InsertPaste(window.Active.CurrentWindow, ev.Data); err == nil {
		applied = true
	}
	if applied {
		MarkPasteDirty()
		display.NotePasteApplied()
	}
	return true
}

func handleQuitEvent(s *ProcState, ev event.QuitEvent) bool {
	if !ev.Force && anyUnsavedBuffers() {
		PushListener(&yesNoListener{
			prompt: "Quit with unsaved buffers?",
			onYes: func() {
				event.Enqueue(event.QuitEvent{Force: true})
			},
			onNo: func() {
				if Current != nil {
					Current.QuitRequested = false
				}
			},
		})
		display.MBWrite("%s (y/n)", "Quit with unsaved buffers?")
		return true
	}
	return false
}

func handleJobDoneEvent(ev event.JobDoneEvent) bool {
	if done, ok := ev.Raw.(tools.BackgroundJobDone); ok {
		tools.HandleBackgroundJobDone(done)
		fileCheckReload()
		return true
	}
	return true
}

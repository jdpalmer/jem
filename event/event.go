// Package event defines editor loop events and the inbound event bus.
//
// Events are queued via Enqueue and applied on a later tick of the main loop.
// This package must not import editor or display.
package event

// Event is anything the main loop can apply on a tick.
type Event interface{ isEvent() }

// KeyEvent is a decoded editor key (jem encoding: CTL, META, CTLX, …).
type KeyEvent struct {
	Code uint32
}

func (KeyEvent) isEvent() {}

// ResizeEvent is a terminal size change.
type ResizeEvent struct {
	Rows, Cols int
}

func (ResizeEvent) isEvent() {}

// QuitEvent requests process exit (after optional unsaved-buffer confirmation).
type QuitEvent struct {
	Force bool
}

func (QuitEvent) isEvent() {}

// CommandEvent names a command to run (phase 3+).
type CommandEvent struct {
	Name string
	F    bool
	N    int
}

func (CommandEvent) isEvent() {}

// JobDoneEvent is posted when a background tools job finishes.
type JobDoneEvent struct {
	Kind string
	// Raw holds the tools package payload until tools is fully event-oriented.
	Raw any
}

func (JobDoneEvent) isEvent() {}

// PasteEvent carries bracketed-paste bytes for application on a later tick.
type PasteEvent struct {
	Data []byte
}

func (PasteEvent) isEvent() {}

// MacroStepEvent is one recorded keystroke with optional universal argument.
type MacroStepEvent struct {
	Code uint32
	F    bool
	N    int
}

func (MacroStepEvent) isEvent() {}

// PromptReplyEvent supplies the answer for the next Ask* during macro play.
type PromptReplyEvent struct {
	Text string
}

func (PromptReplyEvent) isEvent() {}

package buffer

// EditSession holds post-mutation side effects for interactive editing.
//
// Buffer owns the text replace and local syntax invalidation. The session
// supplies editor-facing work that buffer cannot import (window cursor/mark
// adjustment, redraw flags, and incremental syntax reparse).
//
// Prefer Buffer.SetText / edit.SetText for interactive edits; ReplaceRaw is for
// undo replay and tests that skip session effects when unset.
//
// Install once via SetEditSession (editor.Services). Not safe for concurrent use.
type EditSession struct {
	NoteEdit                    func(bp *Buffer, isStructural bool)
	AdjustLocationsAfterReplace func(bp *Buffer, begin, end, newEnd Location)
	ReparseFrom                 func(bp *Buffer, lineNumber uint)
}

var editSession EditSession

// SetEditSession installs the active session used by SetText / ReplaceRaw.
func SetEditSession(s EditSession) {
	editSession = s
}

// ActiveEditSession returns the currently installed session.
func ActiveEditSession() EditSession {
	return editSession
}

// WithEditSession runs fn with s active, then restores the previous session.
func WithEditSession(s EditSession, fn func()) {
	old := editSession
	editSession = s
	defer func() { editSession = old }()
	fn()
}

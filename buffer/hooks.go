package buffer

// Hooks connects buffer mutations to the editor shell.
//
// Buffer owns the text replace and local syntax invalidation. Hooks supply
// work buffer cannot import (window cursor/mark adjustment, redraw flags,
// incremental syntax reparse).
//
// Prefer Buffer.SetText for interactive edits; ReplaceRaw is for
// undo replay and tests that skip hooks when unset.
//
// Set once during editor init via runtime.Services. Not safe for concurrent use.
type Hooks struct {
	NoteEdit                    func(bp *Buffer, isStructural bool)
	AdjustLocationsAfterReplace func(bp *Buffer, begin, end, newEnd Location)
	ReparseFrom                 func(bp *Buffer, lineNumber uint)
	OnBufferCreate              func(bp *Buffer)
	// OnBufferKill is called after bp is removed from All.
	// replacement may be nil when no buffers remain.
	OnBufferKill     func(bp, replacement *Buffer)
	UndoForgetBuffer func(bp *Buffer)
}

// PackageHooks is set by the editor during init.
var PackageHooks Hooks

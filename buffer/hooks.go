package buffer

// Hooks connects buffer mutations to the editor shell.
//
// Required for correct interactive editing:
//   - AdjustLocationsAfterReplace (cursor, mark, viewport)
//   - ReparseFrom (syntax after structural edits)
//
// Optional enhancements:
//   - NoteEdit (redraw and mode-line updates; IsChanged is always set by buffer)
//
// Set once during editor init via initBufferSyntaxHooks. Not safe for concurrent use.
type Hooks struct {
	NoteEdit                    func(bp *Buffer, isStructural bool)
	AdjustLocationsAfterReplace func(bp *Buffer, begin, end, newEnd Location)
	InvalidateSyntaxFrom        func(bp *Buffer, lineNumber uint)
	ReparseFrom                 func(bp *Buffer, lineNumber uint)
}

// PackageHooks is set by the editor during init.
var PackageHooks Hooks

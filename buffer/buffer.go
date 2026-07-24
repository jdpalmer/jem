package buffer

import "time"

type Buffer struct {
	Lines             []Line
	Serial            uint32
	SavedUndoSerial   uint32
	IsChanged         bool
	IsReadonly        bool
	EolMode           EolMode
	LangMode          LangMode
	FillCol           int
	Indent            IndentConfig
	WhitespaceCleanup bool
	Name              string
	FileName          string
	FileModTime       time.Time
	NotifiedModTime   time.Time
	Cursor            Location // last-known cursor; windows own live cursor state
	Mark              Location // Line == 0 means unset; otherwise 1-based line index
}

// New creates and returns a new Buffer with default settings.
// Every buffer always has at least one line (empty when newly created).
func New() *Buffer {
	buf := &Buffer{
		EolMode:  EModeLF,
		LangMode: LModeNone,
	}
	buf.EnsureMinLines()
	return buf
}

// Clear removes all content and resets the buffer to a single empty line.
func (buf *Buffer) Clear() {
	buf.Lines = nil
	buf.EnsureMinLines()
	buf.IsChanged = false
	buf.Cursor = Location{Line: 1, Offset: 0}
	buf.Mark = Location{Line: 0, Offset: 0}
}

// DiscardLines drops all lines ahead of a full content rebuild.
// Callers must finish with EnsureMinLines (or AppendLineBytes) so the
// ≥1-line invariant holds.
func (buf *Buffer) DiscardLines() {
	buf.Lines = nil
}

// EnsureMinLines restores the invariant that buf always has ≥1 line.
func (buf *Buffer) EnsureMinLines() {
	if len(buf.Lines) == 0 {
		_ = buf.AppendLineBytes(nil)
	}
}

// EOF returns the location just past the last line (1-based lines).
// For an empty buffer this is line 1; with N lines it is line N+1.
func (buf *Buffer) EOF() int {
	return len(buf.Lines) + 1
}

// Line returns line lineNumber (1-based). The pointer is invalidated if
// buf.Lines is reallocated; prefer line numbers across edits.
func (buf *Buffer) Line(lineNumber int) *Line {
	if lineNumber <= 0 || lineNumber > len(buf.Lines) {
		return nil
	}
	return &buf.Lines[lineNumber-1]
}

// TrimTrailingWhitespace removes trailing spaces and tabs from the given line.
func (buf *Buffer) TrimTrailingWhitespace(lineNumber int) bool {
	if lineNumber <= 0 || lineNumber > len(buf.Lines) {
		return false
	}
	line := &buf.Lines[lineNumber-1]
	newLen := len(line.Data)
	for newLen > 0 {
		c := line.Data[newLen-1]
		if c != ' ' && c != '\t' {
			break
		}
		newLen--
	}
	if newLen == len(line.Data) {
		return false
	}
	begin := Location{Line: lineNumber, Offset: newLen}
	end := Location{Line: lineNumber, Offset: len(line.Data)}
	_, err := buf.ReplaceRaw(begin, end, nil, nil)
	if err != nil {
		return false
	}
	buf.IsChanged = true
	return true
}

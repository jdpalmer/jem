package buffer

import "time"

type Buffer struct {
	Lines                   []Line
	LineCount               uint
	Serial                  uint32
	SavedUndoSerial         uint32
	IsChanged               bool
	IsReadonly              bool
	EolMode                 EolMode
	LangMode                LangMode
	FillCol                 uint32
	CIndent                 uint32
	CBrace                  uint32
	CColonOffset            uint32
	PyIndent                uint32
	PyContinuedOffset       uint32
	WhitespaceCleanup       bool
	Name                    string
	FileName                string
	FileMtime               time.Time
	DiskChangeNotifiedMtime time.Time
	Cursor                  Location // last-known cursor; windows own live cursor state
	Mark                    Location // Line == 0 means unset; otherwise 1-based line index
}

func New() *Buffer {
	return &Buffer{
		EolMode:  EModeLF,
		LangMode: LModeNone,
	}
}

func (buf *Buffer) Clear() bool {
	if buf == nil {
		return false
	}
	buf.Lines = nil
	buf.LineCount = 0
	buf.IsChanged = false
	buf.Cursor = Location{Line: 1, Offset: 0}
	buf.Mark = Location{Line: 0, Offset: 0}
	return true
}

// EOF returns the location just past the last line (1-based lines).
// For an empty buffer this is line 1; with N lines it is line N+1.
func (buf *Buffer) EOF() uint {
	if buf == nil {
		return 1
	}
	return buf.LineCount + 1
}

// Line returns line lineNumber (1-based). The pointer is invalidated if
// buf.Lines is reallocated; prefer line numbers across edits.
func (buf *Buffer) Line(lineNumber uint) *Line {
	if buf == nil || lineNumber == 0 || lineNumber > buf.LineCount {
		return nil
	}
	return &buf.Lines[lineNumber-1]
}

func (buf *Buffer) TrimTrailingWhitespace(lineNumber uint) bool {
	if lineNumber == 0 || lineNumber > buf.LineCount {
		return false
	}
	line := &buf.Lines[lineNumber-1]
	newLen := uint(len(line.Data))
	for newLen > 0 {
		c := line.Data[newLen-1]
		if c != ' ' && c != '\t' {
			break
		}
		newLen--
	}
	if newLen == uint(len(line.Data)) {
		return false
	}
	begin := Location{Line: lineNumber, Offset: newLen}
	end := Location{Line: lineNumber, Offset: uint(len(line.Data))}
	return buf.SetText(nil, begin, end, nil, nil) == nil
}

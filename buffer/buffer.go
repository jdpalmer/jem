package buffer

import "time"

const BufferNameCapacity = 16

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

func (bp *Buffer) Clear() bool {
	if bp == nil {
		return false
	}
	bp.Lines = nil
	bp.LineCount = 0
	bp.IsChanged = false
	bp.Cursor = Location{Line: 1, Offset: 0}
	bp.Mark = Location{Line: 0, Offset: 0}
	return true
}

// EOF returns the location just past the last line (1-based lines).
// For an empty buffer this is line 1; with N lines it is line N+1.
func (bp *Buffer) EOF() uint {
	if bp == nil {
		return 1
	}
	return bp.LineCount + 1
}

// Line returns line lineNumber (1-based). The pointer is invalidated if
// bp.Lines is reallocated; prefer line numbers across edits.
func (bp *Buffer) Line(lineNumber uint) *Line {
	if bp == nil || lineNumber == 0 || lineNumber > bp.LineCount {
		return nil
	}
	return &bp.Lines[lineNumber-1]
}

func (bp *Buffer) TrimTrailingWhitespace(lineNumber uint) bool {
	if lineNumber == 0 || lineNumber > bp.LineCount {
		return false
	}
	line := &bp.Lines[lineNumber-1]
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
	return bp.SetText(nil, begin, end, nil, nil) == nil
}

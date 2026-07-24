package buffer

import (
	"slices"
	"strings"
)

// MaxBuffers caps the open-buffer list. The choose UI indexes choices with
// uint8, so this matches that presentation limit rather than a storage need.
const MaxBuffers = 255

// List is the process-wide open-buffer registry.
type List struct {
	Buffers    []*Buffer
	Current    *Buffer
	NextSerial uint32
}

var defaultList List

// All points at the active buffer list. Bound by runtime.App.Activate.
var All *List = &defaultList

// BindList points All at l. Pass nil to restore the package default.
func BindList(l *List) {
	All = l
}

// Create allocates a buffer and appends it to All.
func Create() *Buffer {
	if len(All.Buffers) >= MaxBuffers {
		return nil
	}
	buf := New()
	buf.FillCol = DefaultFillCol
	buf.Indent = DefaultIndent
	buf.WhitespaceCleanup = DefaultWhitespaceCleanup
	buf.Serial = All.NextSerial
	All.NextSerial++
	All.Buffers = append(All.Buffers, buf)
	return buf
}

// indexOf returns the index of buf in All.Buffers, or -1 if absent.
func indexOf(buf *Buffer) int {
	for i, b := range All.Buffers {
		if b == buf {
			return i
		}
	}
	return -1
}

// Release removes buf from All, forgets its undo history, and retargets
// All.Current. Returns the replacement buffer (may be nil).
// Callers that own windows should follow with window.RetargetAfterBufferKill.
func Release(buf *Buffer) *Buffer {
	if idx := indexOf(buf); idx != -1 {
		All.Buffers = slices.Delete(All.Buffers, idx, idx+1)
	}

	replacement := (*Buffer)(nil)
	if len(All.Buffers) > 0 {
		replacement = All.Buffers[0]
	}

	if All.Current == buf {
		All.Current = replacement
	}

	if History != nil {
		History.ForgetBuffer(buf)
	}
	return replacement
}

// SetCurrent makes buf the current buffer and moves it to the front of All.Buffers.
func SetCurrent(buf *Buffer) {
	if index := indexOf(buf); index != -1 {
		for i := index; i > 0; i-- {
			All.Buffers[i] = All.Buffers[i-1]
		}
		All.Buffers[0] = buf
	}
	All.Current = buf
}

// Find returns the first buffer whose name equals name (case-insensitive).
func Find(name string) *Buffer {
	for _, buf := range All.Buffers {
		if buf != nil && strings.EqualFold(buf.Name, name) {
			return buf
		}
	}
	return nil
}

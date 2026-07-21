package buffer

import "strings"

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
	if l == nil {
		All = &defaultList
		return
	}
	All = l
}

// Create allocates a buffer and appends it to All.
func Create() *Buffer {
	if len(All.Buffers) >= MaxBuffers {
		return nil
	}
	buf := New()
	buf.Serial = All.NextSerial
	All.NextSerial++
	All.Buffers = append(All.Buffers, buf)
	if PackageHooks.OnBufferCreate != nil {
		PackageHooks.OnBufferCreate(buf)
	}
	return buf
}

// Release removes buf from All and retargets dependents via hooks.
// The buffer is left for the garbage collector once no longer referenced.
func Release(buf *Buffer) {
	if buf == nil {
		return
	}
	idx := -1
	for i, b := range All.Buffers {
		if b == buf {
			idx = i
			break
		}
	}
	if idx != -1 {
		copy(All.Buffers[idx:], All.Buffers[idx+1:])
		All.Buffers[len(All.Buffers)-1] = nil
		All.Buffers = All.Buffers[:len(All.Buffers)-1]
	}

	replacement := (*Buffer)(nil)
	if len(All.Buffers) > 0 {
		replacement = All.Buffers[0]
	}

	if PackageHooks.OnBufferKill != nil {
		PackageHooks.OnBufferKill(buf, replacement)
	}

	if All.Current == buf {
		All.Current = replacement
	}

	if PackageHooks.UndoForgetBuffer != nil {
		PackageHooks.UndoForgetBuffer(buf)
	}
}

// SetCurrent makes buf the current buffer and moves it to the front of All.Buffers.
func SetCurrent(buf *Buffer) {
	if buf == nil {
		return
	}
	index := -1
	for i, b := range All.Buffers {
		if b == buf {
			index = i
			break
		}
	}
	if index != -1 {
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

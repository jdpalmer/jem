package buffer

import "strings"

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

// TruncateName truncates a buffer name to BufferNameCapacity.
func TruncateName(name string) string {
	if len(name) >= BufferNameCapacity {
		return name[:BufferNameCapacity-1]
	}
	return name
}

// Create allocates a buffer and appends it to All.
func Create() *Buffer {
	if len(All.Buffers) >= MaxBuffers {
		return nil
	}
	bp := New()
	bp.Serial = All.NextSerial
	All.NextSerial++
	All.Buffers = append(All.Buffers, bp)
	if PackageHooks.OnBufferCreate != nil {
		PackageHooks.OnBufferCreate(bp)
	}
	return bp
}

// Release removes bp from All and retargets dependents via hooks.
// The buffer is left for the garbage collector once no longer referenced.
func Release(bp *Buffer) {
	if bp == nil {
		return
	}
	idx := -1
	for i, b := range All.Buffers {
		if b == bp {
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
		PackageHooks.OnBufferKill(bp, replacement)
	}

	if All.Current == bp {
		All.Current = replacement
	}

	if PackageHooks.UndoForgetBuffer != nil {
		PackageHooks.UndoForgetBuffer(bp)
	}
}

// SetCurrent makes bp the current buffer and moves it to the front of All.Buffers.
func SetCurrent(bp *Buffer) {
	if bp == nil {
		return
	}
	index := -1
	for i, b := range All.Buffers {
		if b == bp {
			index = i
			break
		}
	}
	if index != -1 {
		for i := index; i > 0; i-- {
			All.Buffers[i] = All.Buffers[i-1]
		}
		All.Buffers[0] = bp
	}
	All.Current = bp
}

// Find returns the first buffer whose name equals name (case-insensitive).
func Find(name string) *Buffer {
	for _, bp := range All.Buffers {
		if bp != nil && strings.EqualFold(bp.Name, name) {
			return bp
		}
	}
	return nil
}

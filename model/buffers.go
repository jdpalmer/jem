package model

import (
	"strings"

	"github.com/jdpalmer/jem/buffer"
)

func TruncateBufferName(name string) string {
	if len(name) >= BufferNameCapacity {
		return name[:BufferNameCapacity-1]
	}
	return name
}

func BufferCreate(ed *EditorRuntimeState) *buffer.Buffer {
	if len(ed.Buffers) >= MaxBuffers {
		return nil
	}
	bp := buffer.New()
	bp.Serial = ed.NextBufferSerial
	ed.NextBufferSerial++
	ed.Buffers = append(ed.Buffers, bp)
	return bp
}

func BufferRelease(bp *buffer.Buffer) {
	if bp == nil {
		return
	}
	idx := -1
	for i, b := range State.Buffers {
		if b == bp {
			idx = i
			break
		}
	}
	if idx != -1 {
		copy(State.Buffers[idx:], State.Buffers[idx+1:])
		State.Buffers[len(State.Buffers)-1] = nil
		State.Buffers = State.Buffers[:len(State.Buffers)-1]
	}

	replacement := (*buffer.Buffer)(nil)
	if len(State.Buffers) > 0 {
		replacement = State.Buffers[0]
	}

	for _, wp := range State.Windows {
		if wp == nil {
			continue
		}
		if wp.Buffer == bp {
			if replacement == nil {
				replacement = BufferCreate(&State.EditorRuntimeState)
				if replacement == nil {
					wp.Buffer = nil
					continue
				}
			}
			wp.Buffer = replacement
			if replacement.Cursor.Line >= 1 {
				wp.Cursor = replacement.Cursor
			} else {
				wp.Cursor = buffer.Location{Line: 1, Offset: 0}
			}
			wp.Mark = replacement.Mark
			wp.TopLine = 1
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
	}

	if State.CurrentBuffer == bp {
		if replacement != nil {
			State.CurrentBuffer = replacement
		} else {
			State.CurrentBuffer = nil
		}
	}

	if PackageHooks.UndoForgetBuffer != nil {
		PackageHooks.UndoForgetBuffer(bp)
	}
	bp.Destroy()
}

func SetCurrentBuffer(bp *buffer.Buffer) {
	if bp == nil {
		return
	}
	index := -1
	for i, b := range State.Buffers {
		if b == bp {
			index = i
			break
		}
	}
	if index != -1 {
		for i := index; i > 0; i-- {
			State.Buffers[i] = State.Buffers[i-1]
		}
		State.Buffers[0] = bp
	}
	State.CurrentBuffer = bp
}

func BufferFind(name string) *buffer.Buffer {
	for _, bp := range State.Buffers {
		if bp != nil && strings.EqualFold(bp.Name, name) {
			return bp
		}
	}
	return nil
}

// SwitchBuffer makes bp current in the active window, restoring cursor/mark
// from another window showing bp when possible, else from the buffer.
func SwitchBuffer(bp *buffer.Buffer) {
	if bp == nil {
		return
	}
	cw := State.CurrentWindow
	if cw == nil {
		return
	}

	cw.SaveState()

	SetCurrentBuffer(bp)
	cw.Buffer = bp
	cw.ShouldUpdateModeLine = true
	cw.ShouldReframe = true
	cw.ShouldRedraw = true
	cw.SetTopLine(1)
	cw.HScroll = 0

	for i := 0; i < int(len(State.Windows)); i++ {
		wp := State.Windows[i]
		if wp != nil && wp != cw && wp.Buffer == bp {
			cw.TopLine = wp.TopLine
			cw.Cursor = wp.Cursor
			cw.Mark = wp.Mark
			cw.HScroll = wp.HScroll
			return
		}
	}

	if bp.Cursor.Line >= 1 {
		cw.SetCursor(bp.Cursor)
	} else {
		cw.SetCursor(buffer.Location{Line: 1, Offset: 0})
	}
	cw.Mark = bp.Mark
}

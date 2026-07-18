package app

import "github.com/jdpalmer/jem/buffer"

func TruncateBufferName(name string) string {
	if len(name) >= BufferNameCapacity {
		return name[:BufferNameCapacity-1]
	}
	return name
}

func BufferCreate(ed *EditorRuntimeState) *Buffer {
	if ed.BufferCount >= MaxBuffers {
		return nil
	}
	bp := buffer.New()
	bp.Serial = ed.NextBufferSerial
	ed.NextBufferSerial++
	ed.Buffers[ed.BufferCount] = bp
	ed.BufferCount++
	return bp
}

func BufferRelease(bp *Buffer) {
	if bp == nil {
		return
	}
	idx := -1
	for i := 0; i < int(State.BufferCount); i++ {
		if State.Buffers[i] == bp {
			idx = i
			break
		}
	}
	if idx != -1 {
		for i := idx; i < int(State.BufferCount)-1; i++ {
			State.Buffers[i] = State.Buffers[i+1]
		}
		State.Buffers[State.BufferCount-1] = nil
		State.BufferCount--
	}

	replacement := (*Buffer)(nil)
	if State.BufferCount > 0 {
		replacement = State.Buffers[0]
	}

	for i := 0; i < int(State.WindowCount); i++ {
		wp := State.WINDOWS[i]
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
				wp.Cursor = Location{Line: 1, Offset: 0}
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

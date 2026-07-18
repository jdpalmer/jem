package app

import "github.com/jdpalmer/jem/buffer"

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

	for _, wp := range State.WINDOWS {
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

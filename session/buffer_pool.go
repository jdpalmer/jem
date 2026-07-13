package session

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
	for i := 0; i < int(App.BufferCount); i++ {
		if App.Buffers[i] == bp {
			idx = i
			break
		}
	}
	if idx != -1 {
		for i := idx; i < int(App.BufferCount)-1; i++ {
			App.Buffers[i] = App.Buffers[i+1]
		}
		App.Buffers[App.BufferCount-1] = nil
		App.BufferCount--
	}

	replacement := (*Buffer)(nil)
	if App.BufferCount > 0 {
		replacement = App.Buffers[0]
	}

	for i := 0; i < int(App.WindowCount); i++ {
		wp := App.WINDOWS[i]
		if wp == nil {
			continue
		}
		if wp.Buffer == bp {
			if replacement == nil {
				replacement = BufferCreate(&App.EditorRuntimeState)
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

	if App.CurrentBuffer == bp {
		if replacement != nil {
			App.CurrentBuffer = replacement
		} else {
			App.CurrentBuffer = nil
		}
	}

	if PackageHooks.UndoForgetBuffer != nil {
		PackageHooks.UndoForgetBuffer(bp)
	}
	buffer.Destroy(bp)
}

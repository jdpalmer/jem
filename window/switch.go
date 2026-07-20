package window

import "github.com/jdpalmer/jem/buffer"

// SwitchBuffer makes bp current in the active window, restoring cursor/mark
// from another window showing bp when possible, else from the buffer.
func SwitchBuffer(bp *buffer.Buffer) {
	if bp == nil {
		return
	}
	cw := Active.CurrentWindow
	if cw == nil {
		return
	}

	cw.SaveState()

	if PackageHooks.SetCurrentBuffer != nil {
		PackageHooks.SetCurrentBuffer(bp)
	} else {
		buffer.SetCurrent(bp)
	}
	cw.Buffer = bp
	cw.ShouldUpdateModeLine = true
	cw.ShouldReframe = true
	cw.ShouldRedraw = true
	cw.SetTopLine(1)
	cw.HScroll = 0

	for i := 0; i < int(len(Active.Windows)); i++ {
		wp := Active.Windows[i]
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

// RetargetAfterBufferKill updates windows that showed killed to use replacement.
// If replacement is nil, creates one via BufferCreate when possible.
func RetargetAfterBufferKill(killed, replacement *buffer.Buffer) {
	if killed == nil {
		return
	}
	for _, wp := range Active.Windows {
		if wp == nil || wp.Buffer != killed {
			continue
		}
		rep := replacement
		if rep == nil {
			if PackageHooks.BufferCreate != nil {
				rep = PackageHooks.BufferCreate()
			}
			if rep == nil {
				wp.Buffer = nil
				continue
			}
			replacement = rep // reuse for subsequent windows
		}
		wp.Buffer = rep
		if rep.Cursor.Line >= 1 {
			wp.Cursor = rep.Cursor
		} else {
			wp.Cursor = buffer.Location{Line: 1, Offset: 0}
		}
		wp.Mark = rep.Mark
		wp.TopLine = 1
		wp.ShouldRedraw = true
		wp.ShouldUpdateModeLine = true
	}
}

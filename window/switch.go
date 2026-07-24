package window

import "github.com/jdpalmer/jem/buffer"

// SwitchBuffer makes buf current in the active window, restoring cursor/mark
// from another window showing buf when possible, else from the buffer.
func SwitchBuffer(buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	cw := Active.CurrentWindow
	if cw == nil {
		return
	}

	cw.SaveState()

	buffer.SetCurrent(buf)
	cw.Buffer = buf
	cw.ShouldUpdateModeLine = true
	cw.ShouldReframe = true
	cw.ShouldRedraw = true
	cw.SetTopLine(1)
	cw.HScroll = 0

	for i := 0; i < len(Active.Windows); i++ {
		win := Active.Windows[i]
		if win != nil && win != cw && win.Buffer == buf {
			cw.TopLine = win.TopLine
			cw.Cursor = win.Cursor
			cw.Mark = win.Mark
			cw.HScroll = win.HScroll
			return
		}
	}

	if buf.Cursor.Line >= 1 {
		cw.SetCursor(buf.Cursor)
	} else {
		cw.SetCursor(buffer.Location{Line: 1, Offset: 0})
	}
	cw.Mark = buf.Mark
}

// RetargetAfterBufferKill updates windows that showed killed to use replacement.
// If replacement is nil, creates one via buffer.Create when possible.
func RetargetAfterBufferKill(killed, replacement *buffer.Buffer) {
	if killed == nil {
		return
	}
	for _, win := range Active.Windows {
		if win == nil || win.Buffer != killed {
			continue
		}
		rep := replacement
		if rep == nil {
			rep = buffer.Create()
			if rep == nil {
				win.Buffer = nil
				continue
			}
			replacement = rep // reuse for subsequent windows
		}
		win.Buffer = rep
		if rep.Cursor.Line >= 1 {
			win.Cursor = rep.Cursor
		} else {
			win.Cursor = buffer.Location{Line: 1, Offset: 0}
		}
		win.Mark = rep.Mark
		win.TopLine = 1
		win.ShouldRedraw = true
		win.ShouldUpdateModeLine = true
	}
}

// ReleaseBuffer releases buf and retargets windows that showed it.
func ReleaseBuffer(buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	rep := buffer.Release(buf)
	RetargetAfterBufferKill(buf, rep)
}

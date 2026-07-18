package app

import "github.com/jdpalmer/jem/buffer"

// AdjustLocationsAfterReplace updates cursor, mark, and TopLine for every
// window showing bp after a replacement of [begin, end) ending at newEnd.
func AdjustLocationsAfterReplace(bp *buffer.Buffer, begin, end, newEnd buffer.Location) {
	AdjustWindowLocations(State.WINDOWS, bp, begin, end, newEnd)
}

// AdjustWindowLocations is the pure form of AdjustLocationsAfterReplace.
func AdjustWindowLocations(windows []*Window, bp *buffer.Buffer, begin, end, newEnd buffer.Location) {
	for i := 0; i < len(windows); i++ {
		wp := windows[i]
		if wp == nil || wp.Buffer != bp {
			continue
		}
		wp.Cursor.AdjustAfterReplace(begin, end, newEnd)
		wp.Mark.AdjustAfterReplace(begin, end, newEnd)
		if wp.TopLine >= begin.Line {
			if wp.TopLine > end.Line {
				if newEnd.Line >= end.Line {
					wp.TopLine += newEnd.Line - end.Line
				} else {
					removed := end.Line - newEnd.Line
					if wp.TopLine >= removed {
						wp.TopLine -= removed
					} else {
						wp.TopLine = 1
					}
				}
			} else {
				wp.TopLine = begin.Line
			}
		}
	}
}

// NoteBufferEdit marks windows showing bp for redraw / modeline after an edit.
// Called from the buffer EditSession before IsChanged is set, so first-change
// modeline updates work.
func NoteBufferEdit(bp *buffer.Buffer, isStructural bool) {
	NoteBufferEditOnWindows(State.WINDOWS, bp, isStructural)
}

// NoteBufferEditOnWindows is the pure form of NoteBufferEdit.
func NoteBufferEditOnWindows(windows []*Window, bp *buffer.Buffer, isStructural bool) {
	if bp == nil {
		return
	}
	firstChange := !bp.IsChanged
	shouldRedraw := isStructural
	count := 0
	for i := 0; i < len(windows); i++ {
		wp := windows[i]
		if wp != nil && wp.Buffer == bp {
			count++
		}
	}
	if count != 1 {
		shouldRedraw = true
	}
	for i := 0; i < len(windows); i++ {
		wp := windows[i]
		if wp == nil || wp.Buffer != bp {
			continue
		}
		if shouldRedraw {
			wp.ShouldRedraw = true
		} else {
			wp.DidEdit = true
		}
		if firstChange {
			wp.ShouldUpdateModeLine = true
		}
	}
}

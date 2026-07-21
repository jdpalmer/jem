package runtime

import (
	"bytes"
	"github.com/jdpalmer/jem/killring"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/window"
	"sort"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
)

// Mark/region commands (kill, copy, case, sort, fill bounds).

func getRegion(wp *window.Window, rp *window.Region) bool {
	if wp == nil || rp == nil {
		return false
	}
	if wp.Mark.Line == 0 {
		display.MBWrite("[no mark set in this window]")
		return false
	}
	cursor := wp.Cursor
	mark := wp.Mark
	cursorBefore := (cursor.Line < mark.Line) || (cursor.Line == mark.Line && cursor.Offset <= mark.Offset)
	if cursorBefore {
		rp.Start = cursor
		rp.End = mark
	} else {
		rp.Start = mark
		rp.End = cursor
	}
	return true
}

// CmdKillRegion deletes the active region and adds it to the kill ring.
func CmdKillRegion(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
	if wp == nil || wp.Buffer == nil {
		return false
	}
	var region window.Region
	if !getRegion(wp, &region) {
		return false
	}
	killring.KillBegin()
	// unset mark
	wp.Mark = buffer.Location{Line: 0, Offset: 0}
	wp.SetCursor(region.Start)
	return bufferSetText(wp.Buffer, region.Start, region.End, nil, nil, true)
}

// CmdCopyRegion copies the active region to the kill buffer and clipboard.
func CmdCopyRegion(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
	if wp == nil || wp.Buffer == nil {
		return false
	}
	var region window.Region
	if !getRegion(wp, &region) {
		return false
	}
	killring.KillBegin()
	text := wp.Buffer.GetText(region.Start, region.End)
	length := uint(len(text))
	if length > 0 && text == nil {
		display.MBWrite("[out of memory]")
		return false
	}
	if !killring.KillAppend(text) {
		display.MBWrite("[out of memory]")
		return false
	}
	killring.KillWriteClipboard()
	display.MBWrite("[region copied]")
	// unset mark and mark window for redraw
	wp.Mark = buffer.Location{Line: 0, Offset: 0}
	wp.ShouldRedraw = true
	return true
}

// CmdYank inserts the most recently killed text at point.
func CmdYank(f bool, n int) bool {
	_ = f
	if n < 0 {
		return false
	}
	// Prefer the system clipboard, but fall back to the in-process kill ring
	// when yanking immediately after a kill in environments without clipboard access.
	if !killring.KillReadClipboard() && !killring.InSequence() {
		return false
	}
	kb := killring.KillBytes()
	klen := uint(len(kb))
	if klen == 0 {
		return false
	}
	if klen > (1 << 31) {
		display.MBWrite("[kill buffer too large]")
		return false
	}
	for i := 0; i < n; i++ {
		// insert kb at point.
		if window.Active.CurrentWindow == nil {
			return false
		}
		if !window.InsertText(window.Active.CurrentWindow, kb) {
			return false
		}
	}
	return true
}

// CmdSetMark sets or unsets the window mark at point.
func CmdSetMark(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
	if wp == nil {
		return false
	}
	if wp.Mark.Line != 0 {
		wp.Mark = buffer.Location{Line: 0, Offset: 0}
		wp.ShouldRedraw = true
		display.MBWrite("[mark unset]")
		return true
	}
	wp.Mark = wp.Cursor
	wp.ShouldRedraw = true
	display.MBWrite("[mark set]")
	return true
}

// CmdSwapMark exchanges point and mark.
func CmdSwapMark(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
	if wp == nil {
		return false
	}
	if wp.Mark.Line == 0 {
		display.MBWrite("[no mark in this window]")
		return false
	}
	temp := wp.Cursor
	wp.SetCursor(wp.Mark)
	wp.Mark = temp
	wp.DidMove = true
	return true
}

// CmdMarkWholeBuffer marks the entire buffer as the active region.
func CmdMarkWholeBuffer(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil {
		return false
	}
	wp.Mark = buffer.MakeLocation(bp.EOF(), 0)
	wp.SetCursor(buffer.MakeLocation(1, 0))
	wp.ShouldRedraw = true
	display.MBWrite("[mark set]")
	return true
}

func transformRegionCase(upper bool) bool {
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	var region window.Region
	if !getRegion(wp, &region) {
		return false
	}
	text := bp.GetText(region.Start, region.End)
	length := uint(len(text))
	if text == nil && length > 0 {
		display.MBWrite("[out of memory]")
		return false
	}
	changed := false
	for i := range text {
		var t byte
		if upper {
			t = u8upper(text[i])
		} else {
			t = u8lower(text[i])
		}
		if t != text[i] {
			text[i] = t
			changed = true
		}
	}
	if !changed {
		return true
	}
	return bufferSetText(bp, region.Start, region.End, text, nil, false)
}

// CmdLowerRegion lowercases the active region.
func CmdLowerRegion(f bool, n int) bool {
	_ = f
	_ = n
	return transformRegionCase(false)
}

// CmdUpperRegion uppercases the active region.
func CmdUpperRegion(f bool, n int) bool {
	_ = f
	_ = n
	return transformRegionCase(true)
}

type sortLine struct {
	text []byte
}

// CmdSortRegion sorts complete lines in the active region alphabetically.
func CmdSortRegion(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	var region window.Region
	if !getRegion(wp, &region) {
		return false
	}
	lastLine := region.End.Line
	nlines := lastLine - region.Start.Line + 1
	if nlines < 2 {
		display.MBWrite("[sort needs at least 2 lines]")
		return false
	}
	start := buffer.MakeLocation(region.Start.Line, 0)
	end := buffer.MakeLocation(lastLine+1, 0)
	text := bp.GetText(start, end)
	total := uint(len(text))
	if text == nil && total > 0 {
		display.MBWrite("[out of memory]")
		return false
	}
	slines := make([]sortLine, 0, nlines)
	p := 0
	for i := uint(0); i < nlines; i++ {
		nl := bytes.IndexByte(text[p:], '\n')
		var llen int
		if nl >= 0 {
			llen = nl
		} else {
			llen = len(text) - p
		}
		slines = append(slines, sortLine{text: append([]byte(nil), text[p:p+llen]...)})
		if nl < 0 {
			break
		}
		p += nl + 1
	}
	sort.Slice(slines, func(i, j int) bool {
		a, b := slines[i].text, slines[j].text
		cmp := bytes.Compare(a, b)
		if cmp != 0 {
			return cmp < 0
		}
		return len(a) < len(b)
	})
	sorted := make([]byte, 0, len(text))
	for _, sl := range slines {
		sorted = append(sorted, sl.text...)
		sorted = append(sorted, '\n')
	}
	savedCursor := wp.Cursor
	if !bufferSetText(bp, start, end, sorted, nil, false) {
		return false
	}
	wp.Cursor = savedCursor
	wp.Mark = buffer.Location{Line: 0, Offset: 0}
	display.MBWrite("[region sorted]")
	return true
}

// markPopOnce restores one mark from the stack, or reports if empty.
func markPopOnce() bool {
	if !markring.PopOnce() {
		display.MBWrite("[no saved mark]")
		return false
	}
	return true
}

// CmdMarkPush saves the current location on the mark stack.
func CmdMarkPush(f bool, n int) bool {
	_ = f
	_ = n
	markring.PushCurrent()
	display.MBWrite("[mark pushed]")
	return true
}

// CmdMarkPop restores the most recently pushed mark.
func CmdMarkPop(f bool, n int) bool {
	_ = f
	_ = n
	return markPopOnce()
}

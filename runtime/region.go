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

func getRegion(win *window.Window, rp *window.Region) bool {
	if win.Mark.Line == 0 {
		display.MBWrite("[no mark set in this window]")
		return false
	}
	cursor := win.Cursor
	mark := win.Mark
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
	win := window.Active.CurrentWindow
	var region window.Region
	if !getRegion(win, &region) {
		return false
	}
	killring.KillBegin()
	// unset mark
	win.Mark = buffer.Location{Line: 0, Offset: 0}
	win.SetCursor(region.Start)
	return bufferSetText(win.Buffer, region.Start, region.End, nil, nil, true)
}

// CmdCopyRegion copies the active region to the kill buffer and clipboard.
func CmdCopyRegion(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	var region window.Region
	if !getRegion(win, &region) {
		return false
	}
	killring.KillBegin()
	text := win.Buffer.GetText(region.Start, region.End)
	if !killring.KillAppend(text) {
		display.MBWrite("[out of memory]")
		return false
	}
	killring.KillWriteClipboard()
	display.MBWrite("[region copied]")
	// unset mark and mark window for redraw
	win.Mark = buffer.Location{Line: 0, Offset: 0}
	win.ShouldRedraw = true
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
	klen := len(kb)
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
		if err := window.InsertText(window.Active.CurrentWindow, kb); err != nil {
			return false
		}
	}
	return true
}

// CmdSetMark sets or unsets the window mark at point.
func CmdSetMark(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	if win.Mark.Line != 0 {
		win.Mark = buffer.Location{Line: 0, Offset: 0}
		win.ShouldRedraw = true
		display.MBWrite("[mark unset]")
		return true
	}
	win.Mark = win.Cursor
	win.ShouldRedraw = true
	display.MBWrite("[mark set]")
	return true
}

// CmdSwapMark exchanges point and mark.
func CmdSwapMark(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	if win.Mark.Line == 0 {
		display.MBWrite("[no mark in this window]")
		return false
	}
	temp := win.Cursor
	win.SetCursor(win.Mark)
	win.Mark = temp
	win.DidMove = true
	return true
}

// CmdMarkWholeBuffer marks the entire buffer as the active region.
func CmdMarkWholeBuffer(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	win.Mark = buffer.MakeLocation(buf.EOF(), 0)
	win.SetCursor(buffer.MakeLocation(1, 0))
	win.ShouldRedraw = true
	display.MBWrite("[mark set]")
	return true
}

func transformRegionCase(upper bool) bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	var region window.Region
	if !getRegion(win, &region) {
		return false
	}
	text := buf.GetText(region.Start, region.End)
	changed := false
	for i := range text {
		var t byte
		if upper {
			t = buffer.ToUpperASCII(text[i])
		} else {
			t = buffer.ToLowerASCII(text[i])
		}
		if t != text[i] {
			text[i] = t
			changed = true
		}
	}
	if !changed {
		return true
	}
	return bufferSetText(buf, region.Start, region.End, text, nil, false)
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

// CmdSortRegion sorts complete lines in the active region alphabetically.
func CmdSortRegion(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	var region window.Region
	if !getRegion(win, &region) {
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
	text := buf.GetText(start, end)
	slines := bytes.Split(text, []byte{'\n'})
	if len(slines) > 0 && len(slines[len(slines)-1]) == 0 {
		slines = slines[:len(slines)-1]
	}
	if len(slines) < 2 {
		display.MBWrite("[sort needs at least 2 lines]")
		return false
	}
	sort.Slice(slines, func(i, j int) bool {
		return bytes.Compare(slines[i], slines[j]) < 0
	})
	sorted := bytes.Join(slines, []byte{'\n'})
	sorted = append(sorted, '\n')
	savedCursor := win.Cursor
	if !bufferSetText(buf, start, end, sorted, nil, false) {
		return false
	}
	win.Cursor = savedCursor
	win.Mark = buffer.Location{Line: 0, Offset: 0}
	display.MBWrite("[region sorted]")
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
	if !markring.PopOnce() {
		display.MBWrite("[no saved mark]")
		return false
	}
	return true
}

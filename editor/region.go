package editor

import (
	"bytes"
	"github.com/jdpalmer/jem/buffer"
	"sort"

	sess "github.com/jdpalmer/jem/session"
)

// region.go - port of cmd_region.c: mark/region related commands

func getRegion(wp *Window, rp *Region) bool {
	if wp == nil || rp == nil {
		return false
	}
	if wp.Mark.Line == 0 {
		mbWrite("[no mark set in this window]")
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
	wp := session.App.CurrentWindow
	if wp == nil || wp.Buffer == nil {
		return false
	}
	var region Region
	if !getRegion(wp, &region) {
		return false
	}
	killBegin()
	// unset mark
	wp.Mark = Location{Line: 0, Offset: 0}
	sess.WindowSetCursor(wp, region.Start)
	return bufferSetText(wp.Buffer, region.Start, region.End, nil, 0, nil, true)
}

// CmdCopyRegion copies the active region to the kill buffer and clipboard.
func CmdCopyRegion(f bool, n int) bool {
	_ = f
	_ = n
	wp := session.App.CurrentWindow
	if wp == nil || wp.Buffer == nil {
		return false
	}
	var region Region
	if !getRegion(wp, &region) {
		return false
	}
	killBegin()
	var length uint
	text := buffer.GetText(wp.Buffer, region.Start, region.End, &length)
	if length > 0 && text == nil {
		mbWrite("[out of memory]")
		return false
	}
	if !killAppend(text, length) {
		mbWrite("[out of memory]")
		return false
	}
	killWriteClipboard()
	mbWrite("[region copied]")
	// unset mark and mark window for redraw
	wp.Mark = Location{Line: 0, Offset: 0}
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
	if !killReadClipboard() && session.App.KillState == CmdStateNone {
		return false
	}
	var klen uint
	kb := killBytes(&klen)
	if klen == 0 {
		return false
	}
	if klen > (1 << 31) {
		mbWrite("[kill buffer too large]")
		return false
	}
	for i := 0; i < n; i++ {
		// insert kb at point.
		if session.App.CurrentWindow == nil {
			return false
		}
		if !windowInsertText(session.App.CurrentWindow, kb, int(klen)) {
			return false
		}
	}
	return true
}

// CmdSetMark sets or unsets the window mark at point.
func CmdSetMark(f bool, n int) bool {
	_ = f
	_ = n
	wp := session.App.CurrentWindow
	if wp == nil {
		return false
	}
	if wp.Mark.Line != 0 {
		wp.Mark = Location{Line: 0, Offset: 0}
		wp.ShouldRedraw = true
		mbWrite("[mark unset]")
		return true
	}
	wp.Mark = wp.Cursor
	wp.ShouldRedraw = true
	mbWrite("[mark set]")
	return true
}

// CmdSwapMark exchanges point and mark.
func CmdSwapMark(f bool, n int) bool {
	_ = f
	_ = n
	wp := session.App.CurrentWindow
	if wp == nil {
		return false
	}
	if wp.Mark.Line == 0 {
		mbWrite("[no mark in this window]")
		return false
	}
	temp := wp.Cursor
	sess.WindowSetCursor(wp, wp.Mark)
	wp.Mark = temp
	wp.DidMove = true
	return true
}

// CmdMarkWholeBuffer marks the entire buffer as the active region.
func CmdMarkWholeBuffer(f bool, n int) bool {
	_ = f
	_ = n
	wp := session.App.CurrentWindow
	bp := session.App.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	wp.Mark = buffer.MakeLocation(buffer.EOF(bp), 0)
	sess.WindowSetCursor(wp, buffer.MakeLocation(1, 0))
	wp.ShouldRedraw = true
	mbWrite("[mark set]")
	return true
}

func transformRegionCase(upper bool) bool {
	wp := session.App.CurrentWindow
	bp := session.App.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	var region Region
	if !getRegion(wp, &region) {
		return false
	}
	var length uint
	text := buffer.GetText(bp, region.Start, region.End, &length)
	if text == nil && length > 0 {
		mbWrite("[out of memory]")
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
	return bufferSetText(bp, region.Start, region.End, text, length, nil, false)
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
	wp := session.App.CurrentWindow
	bp := session.App.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	var region Region
	if !getRegion(wp, &region) {
		return false
	}
	lastLine := region.End.Line
	nlines := lastLine - region.Start.Line + 1
	if nlines < 2 {
		mbWrite("[sort needs at least 2 lines]")
		return false
	}
	start := buffer.MakeLocation(region.Start.Line, 0)
	end := buffer.MakeLocation(lastLine+1, 0)
	var total uint
	text := buffer.GetText(bp, start, end, &total)
	if text == nil && total > 0 {
		mbWrite("[out of memory]")
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
	if !bufferSetText(bp, start, end, sorted, uint(len(sorted)), nil, false) {
		return false
	}
	wp.Cursor = savedCursor
	wp.Mark = Location{Line: 0, Offset: 0}
	mbWrite("[region sorted]")
	return true
}

// CmdCopyRegister copies the active region to a named register.
func CmdCopyRegister(f bool, n int) bool {
	_ = f
	_ = n
	wp := session.App.CurrentWindow
	bp := session.App.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	var region Region
	if !getRegion(wp, &region) {
		return false
	}
	name, pr := mbReadString("Register Name: ", "")
	if pr != PromptResultYes {
		return false
	}
	var length uint
	text := buffer.GetText(bp, region.Start, region.End, &length)
	if text == nil && length > 0 {
		mbWrite("[out of memory]")
		return false
	}
	if !RegisterSetText(name, text) {
		return false
	}
	mbWrite("Register '%s' copied.", name)
	wp.Mark = Location{Line: 0, Offset: 0}
	wp.ShouldRedraw = true
	return true
}

// CmdInsertRegister inserts a named register at point.
func CmdInsertRegister(f bool, n int) bool {
	_ = f
	_ = n
	wp := session.App.CurrentWindow
	if wp == nil {
		return false
	}
	name, pr := mbReadString("Register Name: ", "")
	if pr != PromptResultYes {
		return false
	}
	text, ok := RegisterGetText(name)
	if !ok {
		mbWrite("[register '%s' not found]", name)
		return false
	}
	if !windowInsertText(wp, text, len(text)) {
		return false
	}
	mbWrite("Register '%s' inserted.", name)
	return true
}

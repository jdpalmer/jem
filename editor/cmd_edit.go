package editor

import (
	"github.com/jdpalmer/jem/buffer"
	"time"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/term"
)

// cmd_edit.go — editing commands ported from src/cmd_edit.c

func windowDeleteBytes(wp *Window, n int, kill bool) bool {
	if wp == nil || wp.Buffer == nil || n <= 0 {
		return n >= 0
	}
	bp := wp.Buffer
	beginLoc := wp.Cursor
	endLoc := beginLoc
	remaining := n
	for remaining > 0 {
		lp := bp.Line(endLoc.Line)
		if lp == nil {
			break
		}
		avail := len(lp.Data) - int(endLoc.Offset)
		take := remaining
		if take > avail {
			take = avail
		}
		if take > 0 {
			endLoc.Offset += uint(take)
			remaining -= take
			continue
		}
		if endLoc.Line < bp.LineCount {
			endLoc.Line++
			endLoc.Offset = 0
			remaining--
			continue
		}
		if len(lp.Data) == 0 && beginLoc.Line == endLoc.Line && beginLoc.Line > 1 {
			prev := bp.Line(beginLoc.Line-1)
			if prev != nil {
				beginLoc = buffer.MakeLocation(beginLoc.Line-1, prev.Len())
			}
		}
		break
	}
	if beginLoc.Line == endLoc.Line && beginLoc.Offset == endLoc.Offset {
		return true
	}
	return bufferSetText(bp, beginLoc, endLoc, nil, 0, nil, kill)
}

// CmdKill kills text from point to end of line (Emacs kill-line semantics).
func CmdKill(f bool, n int) bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}

	killBegin()
	chunk := 0
	if !f {
		lp := bp.Line(wp.Cursor.Line)
		if lp == nil {
			return false
		}
		chunk = int(lp.Len() - wp.Cursor.Offset)
		if chunk == 0 {
			chunk = 1
		}
	} else if n == 0 {
		chunk = int(wp.Cursor.Offset)
		wp.Cursor.Offset = 0
	} else if n > 0 {
		lineNumber := wp.Cursor.Line
		lp := bp.Line(lineNumber)
		if lp == nil {
			return false
		}
		chunk = int(lp.Len()-wp.Cursor.Offset) + 1
		for i := 1; i < n; i++ {
			lineNumber++
			if lineNumber > bp.LineCount {
				return false
			}
			nlp := bp.Line(lineNumber)
			if nlp == nil {
				return false
			}
			chunk += int(nlp.Len()) + 1
		}
	} else {
		mbWrite("[neg kill]")
		return false
	}
	ok := windowDeleteBytes(wp, chunk, true)
	if ok {
		killWriteClipboard()
		wp.DidEdit = true
	}
	return ok
}

// CmdOpenLine opens blank lines below point and moves back over them.
func CmdOpenLine(f bool, n int) bool {
	if n < 0 {
		return false
	}
	if n == 0 {
		return true
	}
	wp := app.State.CurrentWindow
	if wp == nil {
		return false
	}
	for i := 0; i < n; i++ {
		if !windowInsertNewline(wp) {
			return false
		}
	}
	return CmdBackwardChar(f, n)
}

// CmdQuote reads the next key and inserts it literally n times.
func CmdQuote(f bool, n int) bool {
	_ = f
	if n < 0 {
		return false
	}
	if n == 0 {
		k, ok := term.ReadKey()
		if !ok {
			return false
		}
		_ = k
		return true
	}
	k, ok := term.ReadKey()
	if !ok {
		return false
	}
	wp := app.State.CurrentWindow
	if wp == nil {
		return false
	}
	if k == '\n' || k == '\r' || k == KeyEnter {
		for i := 0; i < n; i++ {
			if !windowInsertNewline(wp) {
				return false
			}
		}
		return true
	}
	for i := 0; i < n; i++ {
		if k < 0x80 {
			if !windowInsertText(wp, []byte{byte(k)}, 1) {
				return false
			}
		} else if !windowInsertCodepoint(wp, rune(k)) {
			return false
		}
	}
	return true
}

// CmdTransposeChars transposes the two characters around point.
func CmdTransposeChars(f bool, n int) bool {
	_ = f
	_ = n
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	lp := bp.Line(wp.Cursor.Line)
	if lp == nil {
		return false
	}
	offset := wp.Cursor.Offset
	var rightStart, rightEnd uint
	if offset == lp.Len() {
		rightEnd = offset
		rightStart = utf8PrevOffset(lp.Data, rightEnd)
	} else {
		rightStart = offset
		rightEnd = utf8NextOffset(lp.Data, rightStart)
	}
	if rightStart == rightEnd {
		return false
	}
	leftEnd := rightStart
	leftStart := utf8PrevOffset(lp.Data, leftEnd)
	if leftStart == leftEnd {
		return false
	}
	leftLen := leftEnd - leftStart
	rightLen := rightEnd - rightStart
	swapped := make([]byte, 0, leftLen+rightLen)
	swapped = append(swapped, lp.Data[rightStart:rightEnd]...)
	swapped = append(swapped, lp.Data[leftStart:leftEnd]...)
	begin := buffer.MakeLocation(wp.Cursor.Line, leftStart)
	end := buffer.MakeLocation(wp.Cursor.Line, rightEnd)
	return bufferSetText(bp, begin, end, swapped, uint(len(swapped)), nil, false)
}

// CmdDeleteBlankLines collapses runs of blank lines around point.
func CmdDeleteBlankLines(f bool, n int) bool {
	_ = f
	_ = n
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	cur := wp.Cursor.Line
	if cur > bp.LineCount {
		return true
	}
	lp := bp.Line(cur)
	if lp != nil && lp.Len() == 0 {
		start := cur
		for start > 1 && bp.Line(start-1).Len() == 0 {
			start--
		}
		end := cur
		for end < bp.LineCount && bp.Line(end+1).Len() == 0 {
			end++
		}
		if end-start+1 <= 1 {
			return true
		}
		return bufferSetText(bp, buffer.MakeLocation(start+1, 0), buffer.MakeLocation(end+1, 0), nil, 0, nil, false)
	}
	nld := uint(0)
	line := cur
	for line < bp.LineCount && bp.Line(line+1).Len() == 0 {
		line++
		nld++
	}
	if nld == 0 {
		return true
	}
	return bufferSetText(bp, buffer.MakeLocation(cur+1, 0), buffer.MakeLocation(cur+nld+1, 0), nil, 0, nil, false)
}

// CmdInsertDate inserts the current date at point.
func CmdInsertDate(f bool, n int) bool {
	_ = f
	_ = n
	wp := app.State.CurrentWindow
	if wp == nil {
		return false
	}
	now := time.Now()
	date := now.Format("Jan 2, 2006")
	return windowInsertText(wp, []byte(date), len(date))
}

// CmdTrimWhitespace deletes horizontal whitespace surrounding point.
func CmdTrimWhitespace(f bool, n int) bool {
	_ = f
	_ = n
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	lp := bp.Line(wp.Cursor.Line)
	if lp == nil {
		return false
	}
	pos := wp.Cursor.Offset
	length := lp.Len()
	start := pos
	for start > 0 {
		c := lp.Byte(start-1)
		if c != ' ' && c != '\t' {
			break
		}
		start--
	}
	end := pos
	for end < length {
		c := lp.Byte(end)
		if c != ' ' && c != '\t' {
			break
		}
		end++
	}
	if start == end {
		return true
	}
	wp.Cursor.Offset = start
	return windowDeleteBytes(wp, int(end-start), false)
}

// CmdTransposeLines swaps the current line with the one above it.
func CmdTransposeLines(f bool, n int) bool {
	_ = f
	_ = n
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	curr := wp.Cursor.Line
	if curr <= 1 || curr == bp.EOF() {
		return false
	}
	p0 := buffer.MakeLocation(curr-1, 0)
	p2 := buffer.MakeLocation(curr+1, 0)
	var total uint
	original := bp.GetText(p0, p2, &total)
	if original == nil && total > 0 {
		mbWrite("[out of memory]")
		return false
	}
	prevLp := bp.Line(curr-1)
	if prevLp == nil {
		return false
	}
	len1 := prevLp.Len() + 1
	if uint(len(original)) < len1 {
		return false
	}
	swapped := make([]byte, 0, len(original))
	swapped = append(swapped, original[len1:]...)
	swapped = append(swapped, original[:len1]...)
	if !bufferSetText(bp, p0, p2, swapped, uint(len(swapped)), nil, false) {
		return false
	}
	wp.SetCursor(buffer.MakeLocation(curr-1, 0))
	return true
}

// bufferCharStats returns the character under point, chars before point, and total chars.
func bufferCharStats(bp *Buffer, wp *Window) (charAt int, before, total uint) {
	if bp == nil || wp == nil {
		return '\n', 0, 0
	}
	cline := uint(1)
	cbo := uint(0)
	var nch uint
	for {
		lp := bp.Line(cline)
		if cline == wp.Cursor.Line && cbo == wp.Cursor.Offset {
			before = nch
			if lp == nil || cbo == lp.Len() {
				charAt = '\n'
			} else {
				charAt = int(lp.Byte(cbo))
			}
		}
		if lp == nil || cbo == lp.Len() {
			if cline >= bp.LineCount {
				break
			}
			cline++
			cbo = 0
		} else {
			cbo++
		}
		nch++
	}
	return charAt, before, nch
}

// CmdShowPosition displays cursor line/column, character code, and buffer progress.
func CmdShowPosition(f bool, n int) bool {
	_ = f
	_ = n
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		mbWrite("[no buffer]")
		return false
	}
	charAt, before, total := bufferCharStats(bp, wp)
	col := windowCursorScreenCol(wp)
	ratio := uint(0)
	if total > 0 {
		ratio = (100 * before) / total
	}
	row := int(app.State.Cursor.Row) + 1
	mbWrite("X=%d Y=%d CH=0x%x .=%d (%d%% of %d)", col+1, row, charAt, before, ratio, total)
	return true
}

package runtime

import (
	"github.com/jdpalmer/jem/killring"
	"github.com/jdpalmer/jem/window"
	"time"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/mode"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
)

// cmd_edit_text.go — character/line editing commands (sibling of cmd_edit_word.go).

func bufferSetText(bp *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location, kill bool) bool {
	if kill {
		oldText := bp.GetText(begin, end)
		if len(oldText) > 0 && !killring.KillAppend(oldText) {
			return false
		}
	}
	err := SetText(bp, begin, end, newText, newEndOut)
	if err != nil {
		return false
	}
	if kill {
		killring.KillWriteClipboard()
	}
	return true
}

// CmdModeNewlineAndIndent inserts a newline with mode indent, or visits a
// grep/compile match when Enter is pressed in those jump buffers.
func CmdModeNewlineAndIndent(f bool, n int) bool {
	bp := buffer.All.Current
	if bp != nil {
		switch bp.Name {
		case tools.GrepBufferName:
			return tools.VisitGrepMatch()
		case tools.CompileBufferName:
			return tools.VisitCompileDiag()
		}
	}
	return mode.CmdModeNewlineAndIndent(f, n)
}

func windowDeleteBytes(wp *window.Window, n int, kill bool) bool {
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
			prev := bp.Line(beginLoc.Line - 1)
			if prev != nil {
				beginLoc = buffer.MakeLocation(beginLoc.Line-1, prev.Len())
			}
		}
		break
	}
	if beginLoc.Line == endLoc.Line && beginLoc.Offset == endLoc.Offset {
		return true
	}
	return bufferSetText(bp, beginLoc, endLoc, nil, nil, kill)
}

// CmdKill kills text from point to end of line (Emacs kill-line semantics).
func CmdKill(f bool, n int) bool {
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}

	killring.KillBegin()
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
		display.MBWrite("[neg kill]")
		return false
	}
	ok := windowDeleteBytes(wp, chunk, true)
	if ok {
		killring.KillWriteClipboard()
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
	wp := window.Active.CurrentWindow
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
	wp := window.Active.CurrentWindow
	if wp == nil {
		return false
	}
	if k == '\n' || k == '\r' || k == term.KeyEnter {
		for i := 0; i < n; i++ {
			if !windowInsertNewline(wp) {
				return false
			}
		}
		return true
	}
	for i := 0; i < n; i++ {
		if k < 0x80 {
			if !windowInsertText(wp, []byte{byte(k)}) {
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
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
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
	return bufferSetText(bp, begin, end, swapped, nil, false)
}

// CmdDeleteBlankLines collapses runs of blank lines around point.
func CmdDeleteBlankLines(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
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
		return bufferSetText(bp, buffer.MakeLocation(start+1, 0), buffer.MakeLocation(end+1, 0), nil, nil, false)
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
	return bufferSetText(bp, buffer.MakeLocation(cur+1, 0), buffer.MakeLocation(cur+nld+1, 0), nil, nil, false)
}

// CmdInsertDate inserts the current date at point.
func CmdInsertDate(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
	if wp == nil {
		return false
	}
	now := time.Now()
	date := now.Format("Jan 2, 2006")
	return windowInsertText(wp, []byte(date))
}

// CmdTrimWhitespace deletes horizontal whitespace surrounding point.
func CmdTrimWhitespace(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
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
		c := lp.Byte(start - 1)
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
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	curr := wp.Cursor.Line
	if curr <= 1 || curr == bp.EOF() {
		return false
	}
	p0 := buffer.MakeLocation(curr-1, 0)
	p2 := buffer.MakeLocation(curr+1, 0)
	original := bp.GetText(p0, p2)
	total := uint(len(original))
	if original == nil && total > 0 {
		display.MBWrite("[out of memory]")
		return false
	}
	prevLp := bp.Line(curr - 1)
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
	if !bufferSetText(bp, p0, p2, swapped, nil, false) {
		return false
	}
	wp.SetCursor(buffer.MakeLocation(curr-1, 0))
	return true
}

// bufferCharStats returns the character under point, chars before point, and total chars.
func bufferCharStats(bp *buffer.Buffer, wp *window.Window) (charAt int, before, total uint) {
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
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil {
		display.MBWrite("[no buffer]")
		return false
	}
	charAt, before, total := bufferCharStats(bp, wp)
	col := display.WindowCursorScreenCol(wp)
	ratio := uint(0)
	if total > 0 {
		ratio = (100 * before) / total
	}
	row := int(display.Active.Cursor.Row) + 1
	display.MBWrite("X=%d Y=%d CH=0x%x .=%d (%d%% of %d)", col+1, row, charAt, before, ratio, total)
	return true
}

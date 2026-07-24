package runtime

import (
	"github.com/jdpalmer/jem/killring"
	"github.com/jdpalmer/jem/window"
	"time"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/mode"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
)

// cmd_edit_text.go — character/line editing commands (sibling of cmd_edit_word.go).

func bufferSetText(buf *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location, kill bool) bool {
	if kill {
		oldText := buf.GetText(begin, end)
		if len(oldText) > 0 && !killring.KillAppend(oldText) {
			return false
		}
	}
	err := window.SetText(buf, begin, end, newText, newEndOut)
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
	buf := buffer.All.Current
	if buf != nil {
		switch buf.Name {
		case tools.GrepBufferName:
			return CmdGrepVisitMatch()
		case tools.CompileBufferName:
			return CmdCompileVisitDiag()
		}
	}
	return mode.CmdModeNewlineAndIndent(f, n)
}

func windowDeleteBytes(win *window.Window, n int, kill bool) bool {
	if win == nil || win.Buffer == nil || n <= 0 {
		return n >= 0
	}
	buf := win.Buffer
	beginLoc := win.Cursor
	endLoc := beginLoc
	remaining := n
	for remaining > 0 {
		line := buf.Line(endLoc.Line)
		if line == nil {
			break
		}
		avail := len(line.Data) - int(endLoc.Offset)
		take := remaining
		if take > avail {
			take = avail
		}
		if take > 0 {
			endLoc.Offset += take
			remaining -= take
			continue
		}
		if endLoc.Line < len(buf.Lines) {
			endLoc.Line++
			endLoc.Offset = 0
			remaining--
			continue
		}
		if len(line.Data) == 0 && beginLoc.Line == endLoc.Line && beginLoc.Line > 1 {
			prev := buf.Line(beginLoc.Line - 1)
			if prev != nil {
				beginLoc = buffer.MakeLocation(beginLoc.Line-1, prev.Len())
			}
		}
		break
	}
	if beginLoc.Line == endLoc.Line && beginLoc.Offset == endLoc.Offset {
		return true
	}
	return bufferSetText(buf, beginLoc, endLoc, nil, nil, kill)
}

// CmdKill kills text from point to end of line (Emacs kill-line semantics).
func CmdKill(f bool, n int) bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}

	killring.KillBegin()
	chunk := 0
	if !f {
		line := buf.Line(win.Cursor.Line)
		if line == nil {
			return false
		}
		chunk = int(line.Len() - win.Cursor.Offset)
		if chunk == 0 {
			chunk = 1
		}
	} else if n == 0 {
		chunk = int(win.Cursor.Offset)
		win.Cursor.Offset = 0
	} else if n > 0 {
		lineNumber := win.Cursor.Line
		line := buf.Line(lineNumber)
		if line == nil {
			return false
		}
		chunk = int(line.Len()-win.Cursor.Offset) + 1
		for i := 1; i < n; i++ {
			lineNumber++
			if lineNumber > len(buf.Lines) {
				return false
			}
			nlp := buf.Line(lineNumber)
			if nlp == nil {
				return false
			}
			chunk += int(nlp.Len()) + 1
		}
	} else {
		display.MBWrite("[neg kill]")
		return false
	}
	ok := windowDeleteBytes(win, chunk, true)
	if ok {
		killring.KillWriteClipboard()
		win.DidEdit = true
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
	win := window.Active.CurrentWindow
	for i := 0; i < n; i++ {
		if err := window.InsertNewline(win); err != nil {
			return false
		}
	}
	return CmdBackwardChar(f, n)
}

// CmdQuote reads the next key and inserts it literally n times.
// Interactively this pushes a listener; during macro play it consumes the next
// MacroStepEvent from the tape (so playback does not block on the terminal).
func CmdQuote(f bool, n int) bool {
	_ = f
	if n < 0 {
		return false
	}
	if State.IsPlaying() {
		if State.PlayPos >= len(State.Macro) {
			return false
		}
		ev, ok := State.Macro[State.PlayPos].(event.MacroStepEvent)
		if !ok {
			return false
		}
		State.PlayPos++
		if n == 0 {
			return true
		}
		return quoteInsertKey(ev.Code, n)
	}
	beginQuote(n)
	return true
}

// quoteInsertKey inserts k literally n times (newline or character/codepoint).
func quoteInsertKey(k uint32, n int) bool {
	if n <= 0 {
		return true
	}
	win := window.Active.CurrentWindow
	if k == '\n' || k == '\r' || k == term.KeyEnter {
		for i := 0; i < n; i++ {
			if err := window.InsertNewline(win); err != nil {
				return false
			}
		}
		return true
	}
	for i := 0; i < n; i++ {
		if k < 0x80 {
			if err := window.InsertText(win, []byte{byte(k)}); err != nil {
				return false
			}
		} else if err := window.InsertCodepoint(win, rune(k)); err != nil {
			return false
		}
	}
	return true
}

// CmdTransposeChars transposes the two characters around point.
func CmdTransposeChars(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}
	line := buf.Line(win.Cursor.Line)
	if line == nil {
		return false
	}
	offset := win.Cursor.Offset
	var rightStart, rightEnd int
	if offset == line.Len() {
		rightEnd = offset
		rightStart = buffer.PrevOffset(line.Data, rightEnd)
	} else {
		rightStart = offset
		rightEnd = buffer.NextOffset(line.Data, rightStart)
	}
	if rightStart == rightEnd {
		return false
	}
	leftEnd := rightStart
	leftStart := buffer.PrevOffset(line.Data, leftEnd)
	if leftStart == leftEnd {
		return false
	}
	leftLen := leftEnd - leftStart
	rightLen := rightEnd - rightStart
	swapped := make([]byte, 0, leftLen+rightLen)
	swapped = append(swapped, line.Data[rightStart:rightEnd]...)
	swapped = append(swapped, line.Data[leftStart:leftEnd]...)
	begin := buffer.MakeLocation(win.Cursor.Line, leftStart)
	end := buffer.MakeLocation(win.Cursor.Line, rightEnd)
	return bufferSetText(buf, begin, end, swapped, nil, false)
}

// CmdDeleteBlankLines collapses runs of blank lines around point.
func CmdDeleteBlankLines(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}
	cur := win.Cursor.Line
	if cur > len(buf.Lines) {
		return true
	}
	line := buf.Line(cur)
	if line != nil && line.Len() == 0 {
		start := cur
		for start > 1 && buf.Line(start-1).Len() == 0 {
			start--
		}
		end := cur
		for end < len(buf.Lines) && buf.Line(end+1).Len() == 0 {
			end++
		}
		if end-start+1 <= 1 {
			return true
		}
		return bufferSetText(buf, buffer.MakeLocation(start+1, 0), buffer.MakeLocation(end+1, 0), nil, nil, false)
	}
	nld := 0
	lineNum := cur
	for lineNum < len(buf.Lines) && buf.Line(lineNum+1).Len() == 0 {
		lineNum++
		nld++
	}
	if nld == 0 {
		return true
	}
	return bufferSetText(buf, buffer.MakeLocation(cur+1, 0), buffer.MakeLocation(cur+nld+1, 0), nil, nil, false)
}

// CmdInsertDate inserts the current date at point.
func CmdInsertDate(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	now := time.Now()
	date := now.Format("Jan 2, 2006")
	return window.InsertText(win, []byte(date)) == nil
}

// CmdTrimWhitespace deletes horizontal whitespace surrounding point.
func CmdTrimWhitespace(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}
	line := buf.Line(win.Cursor.Line)
	if line == nil {
		return false
	}
	pos := win.Cursor.Offset
	length := line.Len()
	start := pos
	for start > 0 {
		c := line.Byte(start - 1)
		if c != ' ' && c != '\t' {
			break
		}
		start--
	}
	end := pos
	for end < length {
		c := line.Byte(end)
		if c != ' ' && c != '\t' {
			break
		}
		end++
	}
	if start == end {
		return true
	}
	win.Cursor.Offset = start
	return windowDeleteBytes(win, int(end-start), false)
}

// CmdTransposeLines swaps the current line with the one above it.
func CmdTransposeLines(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}
	curr := win.Cursor.Line
	if curr <= 1 || curr == buf.EOF() {
		return false
	}
	p0 := buffer.MakeLocation(curr-1, 0)
	p2 := buffer.MakeLocation(curr+1, 0)
	original := buf.GetText(p0, p2)
	prevLp := buf.Line(curr - 1)
	if prevLp == nil {
		return false
	}
	len1 := prevLp.Len() + 1
	if len(original) < len1 {
		return false
	}
	swapped := make([]byte, 0, len(original))
	swapped = append(swapped, original[len1:]...)
	swapped = append(swapped, original[:len1]...)
	if !bufferSetText(buf, p0, p2, swapped, nil, false) {
		return false
	}
	win.SetCursor(buffer.MakeLocation(curr-1, 0))
	return true
}

// bufferCharStats returns the character under point, chars before point, and total chars.
func bufferCharStats(buf *buffer.Buffer, win *window.Window) (charAt int, before, total int) {
	cline := 1
	cbo := 0
	var nch int
	for {
		line := buf.Line(cline)
		if cline == win.Cursor.Line && cbo == win.Cursor.Offset {
			before = nch
			if line == nil || cbo == line.Len() {
				charAt = '\n'
			} else {
				charAt = int(line.Byte(cbo))
			}
		}
		if line == nil || cbo == line.Len() {
			if cline >= len(buf.Lines) {
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
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	charAt, before, total := bufferCharStats(buf, win)
	col := display.WindowCursorScreenCol(win)
	ratio := 0
	if total > 0 {
		ratio = (100 * before) / total
	}
	row := display.Active.Cursor.Row + 1
	display.MBWrite("X=%d Y=%d CH=0x%x .=%d (%d%% of %d)", col+1, row, charAt, before, ratio, total)
	return true
}

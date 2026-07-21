package mode

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/window"
)

// indentBytesForCol builds gofmt-style leading whitespace: tabs for each
// full 8-column step, then spaces for the remainder.
func indentBytesForCol(col int) []byte {
	if col <= 0 {
		return nil
	}
	tabs := col / 8
	spaces := col % 8
	out := make([]byte, 0, tabs+spaces)
	for i := 0; i < tabs; i++ {
		out = append(out, '\t')
	}
	for i := 0; i < spaces; i++ {
		out = append(out, ' ')
	}
	return out
}

func calcIndentGo(buf *buffer.Buffer, lineNumber int) int {
	if buf == nil {
		return 0
	}
	saved := buf.Indent.Width
	buf.Indent.Width = goIndentCols
	ind := calcIndent(buf, lineNumber)
	buf.Indent.Width = saved
	return ind
}

func setLineIndentGo(win *window.Window, col int) bool {
	if win == nil || win.Buffer == nil || col < 0 {
		return false
	}
	buf := win.Buffer
	ln := win.Cursor.Line
	line := buf.Line(ln)
	if line == nil {
		return false
	}
	oldFirst := line.FirstNonblank()
	prefix := indentBytesForCol(col)
	begin := buffer.MakeLocation(ln, 0)
	end := buffer.MakeLocation(ln, oldFirst)
	PackageHooks.BeginCommand()
	err := PackageHooks.SetText(buf, begin, end, prefix, nil)
	ok := err == nil
	PackageHooks.EndCommand()
	if ok {
		win.DidEdit = true
		// Park cursor after the new indent when it was in the old indent region.
		if win.Cursor.Offset <= oldFirst {
			win.Cursor.Offset = len(prefix)
		} else {
			delta := len(prefix) - oldFirst
			off := win.Cursor.Offset + delta
			if off < 0 {
				off = 0
			}
			win.Cursor.Offset = off
		}
	}
	return ok
}

func cmdGoNewlineAndIndent(f bool, n int) bool {
	_ = f
	if n < 0 {
		return false
	}
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	for i := 0; i < n; i++ {
		if err := window.InsertNewline(win); err != nil {
			return false
		}
		indent := calcIndentGo(buf, win.Cursor.Line)
		setLineIndentGo(win, indent)
	}
	return true
}

func cmdGoIndentLine(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	col := calcIndentGo(buf, win.Cursor.Line)
	setLineIndentGo(win, col)
	win.DidEdit = true
	return true
}

func cmdGoCloseBrace(f bool, n int) bool {
	_ = f
	if n <= 0 {
		n = 1
	}
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	// Insert closers first so calcIndent sees '}' and aligns to the open brace.
	if !setLineIndentGo(win, 0) {
		return false
	}
	win.Cursor.Offset = 0
	for i := 0; i < n; i++ {
		if err := window.InsertCodepoint(win, '}'); err != nil {
			return false
		}
	}
	col := calcIndentGo(buf, win.Cursor.Line)
	if !setLineIndentGo(win, col) {
		return false
	}
	line := buf.Line(win.Cursor.Line)
	if line != nil {
		win.Cursor.Offset = line.Len()
	}
	win.DidEdit = true
	return true
}

// cmdGoMakeComment inserts or jumps into a // line comment (gofmt style).
func cmdGoMakeComment(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	line := buf.Line(win.Cursor.Line)
	if line != nil {
		prefix := []byte("//")
		if lineHasCommentPrefix(line, prefix) {
			pos := line.FirstNonblank()
			win.Cursor.Offset = pos + len(prefix)
			if win.Cursor.Offset < line.Len() && line.Byte(win.Cursor.Offset) == ' ' {
				win.Cursor.Offset++
			}
			win.DidMove = true
			return true
		}
	}
	if win.Mark.Line != 0 && win.Mark.Line != win.Cursor.Line {
		info := CurrentModeInfo()
		startLine := win.Mark.Line
		endLine := win.Cursor.Line
		if startLine > endLine {
			startLine, endLine = endLine, startLine
		}
		return modeToggleCommentRegion(win, buf, info, []byte("//"), startLine, endLine)
	}
	if line != nil {
		win.Cursor.Offset = line.Len()
	} else {
		win.Cursor.Offset = 0
	}
	if err := window.InsertText(win, []byte("  // ")); err != nil {
		return false
	}
	win.DidMove = true
	return true
}

func init() {
	for i := range modeTable {
		if modeTable[i].Mode != buffer.LModeGo {
			continue
		}
		modeTable[i].NewlineAndIndent = cmdGoNewlineAndIndent
		modeTable[i].IndentLine = cmdGoIndentLine
		modeTable[i].CloseBrace = cmdGoCloseBrace
		modeTable[i].MakeComment = cmdGoMakeComment
		modeTable[i].TopOfFunction = cmdCTopOfFunction
		modeTable[i].EndOfFunction = cmdCEndOfFunction
		modeTable[i].MarkFunction = cmdCMarkFunction
	}
}

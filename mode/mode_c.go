package mode

import (
	"bytes"
	"github.com/jdpalmer/jem/window"

	"github.com/jdpalmer/jem/buffer"
)

func lineColOfOffset(line *buffer.Line, offset uint) int {
	if line == nil {
		return 0
	}
	if offset > line.Len() {
		offset = line.Len()
	}
	col := 0
	for i := uint(0); i < offset; i++ {
		c := line.Data[i]
		if c == '\t' {
			col += 8 - (col % 8)
		} else {
			col++
		}
	}
	return col
}

func lineFirstNonblankOffset(line *buffer.Line) (uint, byte) {
	if line == nil {
		return 0, 0
	}
	for i := uint(0); i < line.Len(); i++ {
		c := line.Data[i]
		if c != ' ' && c != '\t' {
			return i, c
		}
	}
	return line.Len(), 0
}

func lineStartsWith(line *buffer.Line, text string) bool {
	if line == nil {
		return false
	}
	off, _ := lineFirstNonblankOffset(line)
	pat := []byte(text)
	if off >= line.Len() {
		return false
	}
	if off+uint(len(pat)) > line.Len() {
		return false
	}
	return bytes.Equal(line.Data[off:off+uint(len(pat))], pat)
}

func lineIsCommentOrPreproc(buf *buffer.Buffer, lineNumber uint) bool {
	line := buf.Line(lineNumber)
	if line == nil {
		return false
	}
	if line.IsBlank() {
		return false
	}
	off, ch := lineFirstNonblankOffset(line)
	if off >= line.Len() {
		return false
	}
	if ch == '*' || ch == '#' {
		return true
	}
	if ch == '/' && off+1 < line.Len() {
		next := line.Data[off+1]
		if next == '/' || next == '*' {
			return true
		}
	}
	return false
}

func lineIsPreproc(buf *buffer.Buffer, lineNumber uint) bool {
	line := buf.Line(lineNumber)
	if line == nil || line.IsBlank() {
		return false
	}
	_, ch := lineFirstNonblankOffset(line)
	return ch == '#'
}

func prevCodeLineNumber(buf *buffer.Buffer, lineNumber uint) uint {
	for ln := lineNumber; ln > 1; {
		ln--
		p := buf.Line(ln)
		if p == nil {
			continue
		}
		if !p.IsBlank() && !lineIsCommentOrPreproc(buf, ln) {
			return ln
		}
	}
	return 0
}

func lineIsCaseLabel(line *buffer.Line) bool {
	if line == nil {
		return false
	}
	off := line.FirstNonblank()
	if off >= line.Len() {
		return false
	}
	if off+4 <= line.Len() {
		if bytes.Equal(line.Data[off:off+4], []byte("case")) {
			if off+4 == line.Len() {
				return true
			}
			nc := line.Data[off+4]
			return nc == ' ' || nc == '\t' || nc == '('
		}
	}
	if off+7 <= line.Len() && bytes.Equal(line.Data[off:off+7], []byte("default")) {
		return true
	}
	return false
}

func lineEndsWithContinuation(line *buffer.Line) bool {
	if line == nil || line.Len() == 0 {
		return false
	}
	last := line.Data[line.Len()-1]
	switch last {
	case ',', '\\', '?', ':', '+', '-', '*', '/', '%', '&', '|', '^', '=', '<', '>', '!':
		return true
	default:
		return false
	}
}

func calcCommentIndent(buf *buffer.Buffer, lineNumber uint) int {
	prevLine := lineNumber
	for prevLine > 1 {
		prevLine--
		prev := buf.Line(prevLine)
		if prev == nil || prev.IsBlank() {
			continue
		}
		if lineStartsWith(prev, "/*") {
			return int(prev.IndentColumn()) + 1
		}
		if prev.FirstNonblank() < prev.Len() {
			ch := prev.Data[prev.FirstNonblank()]
			if ch == '*' || lineStartsWith(prev, "*/") {
				return int(prev.IndentColumn())
			}
		}
		break
	}
	return 0
}

func findCaseIndent(buf *buffer.Buffer, lineNumber uint, offset uint) int {
	cIndent := buf.CIndent
	cColonOffset := buf.CColonOffset
	for ln := int(lineNumber); ln >= 1; ln-- {
		line := buf.Line(uint(ln))
		if line == nil {
			continue
		}
		if bytes.IndexByte(line.Data, '{') != -1 {
			base := int(line.IndentColumn()) + int(cIndent) + int(cColonOffset)
			if base < 0 {
				return 0
			}
			return base
		}
	}
	return 0
}

func findClosingDelimiterIndent(buf *buffer.Buffer, lineNumber uint, offset uint) int {
	line := buf.Line(lineNumber)
	if line == nil || offset >= line.Len() {
		return 0
	}
	ch := line.Data[offset]
	var open byte
	switch ch {
	case '}':
		open = '{'
	case ')':
		open = '('
	case ']':
		open = '['
	default:
		return 0
	}
	// Search strictly before the closer so we do not depth++ the delimiter we are matching.
	depth := 0
	for ln := int(lineNumber); ln >= 1; ln-- {
		line := buf.Line(uint(ln))
		if line == nil {
			continue
		}
		start := len(line.Data) - 1
		if ln == int(lineNumber) {
			if offset == 0 {
				continue
			}
			start = int(offset) - 1
		}
		for i := start; i >= 0; i-- {
			c := line.Data[i]
			if c == ch {
				depth++
				continue
			}
			if c == open {
				if depth > 0 {
					depth--
					continue
				}
				// Braces align to the opening line's indent (gofmt / K&R), not the '{' column.
				if open == '{' {
					return int(line.IndentColumn())
				}
				return lineColOfOffset(line, uint(i))
			}
		}
	}
	return 0
}

func findUnmatchedOpenDelim(buf *buffer.Buffer, lineNumber, offset uint) (buffer.Location, byte, bool) {
	depthParen := 0
	depthBracket := 0
	for ln := int(lineNumber); ln >= 1; ln-- {
		line := buf.Line(uint(ln))
		if line == nil {
			continue
		}
		limit := len(line.Data)
		if ln == int(lineNumber) {
			limit = int(offset)
		}
		for i := limit - 1; i >= 0; i-- {
			switch line.Data[i] {
			case ')':
				depthParen++
			case '(':
				if depthParen > 0 {
					depthParen--
				} else {
					return buffer.MakeLocation(uint(ln), uint(i)), '(', true
				}
			case ']':
				depthBracket++
			case '[':
				if depthBracket > 0 {
					depthBracket--
				} else {
					return buffer.MakeLocation(uint(ln), uint(i)), '[', true
				}
			}
		}
	}
	return buffer.Location{}, 0, false
}

func findDelimiterContinuationIndent(buf *buffer.Buffer, lineNumber uint, offset uint) int {
	open, _, ok := findUnmatchedOpenDelim(buf, lineNumber, offset)
	if !ok {
		return -1
	}
	line := buf.Line(open.Line)
	if line == nil {
		return -1
	}
	tail := open.Offset + 1
	for tail < line.Len() {
		ch := line.Data[tail]
		if ch != ' ' && ch != '\t' {
			return lineColOfOffset(line, tail)
		}
		tail++
	}
	return int(line.IndentColumn()) + int(buf.CIndent)
}

func findEnclosingBlockIndent(buf *buffer.Buffer, lineNumber uint, offset uint) int {
	open, ok := findUnmatchedOpenBrace(buf, lineNumber, offset)
	if !ok {
		return -1
	}
	line := buf.Line(open.Line)
	if line == nil {
		return -1
	}
	return int(line.IndentColumn()) + int(buf.CIndent)
}

func findUnmatchedOpenBrace(buf *buffer.Buffer, lineNumber, offset uint) (buffer.Location, bool) {
	depth := 0
	for ln := int(lineNumber); ln >= 1; ln-- {
		line := buf.Line(uint(ln))
		if line == nil {
			continue
		}
		limit := len(line.Data)
		if ln == int(lineNumber) {
			limit = int(offset)
		}
		for i := limit - 1; i >= 0; i-- {
			switch line.Data[i] {
			case '}':
				depth++
			case '{':
				if depth > 0 {
					depth--
				} else {
					return buffer.MakeLocation(uint(ln), uint(i)), true
				}
			}
		}
	}
	return buffer.Location{}, false
}

func calcIndent(buf *buffer.Buffer, lineNumber uint) int {
	line := buf.Line(lineNumber)
	if line == nil {
		return 0
	}
	cIndent := buf.CIndent
	cBrace := buf.CBrace
	first, fc := lineFirstNonblankOffset(line)
	if lineIsPreproc(buf, lineNumber) {
		return 0
	}
	if !line.IsBlank() {
		if fc == '*' || fc == '/' {
			return calcCommentIndent(buf, lineNumber)
		}
	}
	if fc == '}' || fc == ')' || fc == ']' {
		return findClosingDelimiterIndent(buf, lineNumber, first)
	}
	if lineIsCaseLabel(line) {
		return findCaseIndent(buf, lineNumber, first)
	}
	indent := findDelimiterContinuationIndent(buf, lineNumber, first)
	if indent >= 0 {
		return indent
	}
	indent = findEnclosingBlockIndent(buf, lineNumber, first)
	if indent >= 0 {
		return indent
	}
	refLine := prevCodeLineNumber(buf, lineNumber)
	if refLine == 0 {
		return 0
	}
	ref := buf.Line(refLine)
	if ref == nil {
		return 0
	}
	ind := int(ref.IndentColumn())
	if ref.LastByte() == ':' && lineIsCaseLabel(ref) {
		ind += int(cIndent)
	} else if lineEndsWithContinuation(ref) {
		ind += int(cIndent)
	}
	if fc == '{' {
		ind += int(cBrace)
	}
	if ind < 0 {
		return 0
	}
	return ind
}

func setLineIndent(win *window.Window, col int) bool {
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
	spaces := make([]byte, col)
	for i := range spaces {
		spaces[i] = ' '
	}
	begin := buffer.MakeLocation(ln, 0)
	end := buffer.MakeLocation(ln, oldFirst)
	PackageHooks.BeginCommand()
	err := PackageHooks.SetText(buf, begin, end, spaces, nil)
	ok := err == nil
	PackageHooks.EndCommand()
	if ok {
		win.DidEdit = true
	}
	return ok
}

func cmdCNewlineAndIndent(f bool, n int) bool {
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
		indent := calcIndent(buf, win.Cursor.Line)
		setLineIndent(win, indent)
	}
	return true
}

func cmdCIndentLine(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	col := calcIndent(buf, win.Cursor.Line)
	setLineIndent(win, col)
	win.DidEdit = true
	return true
}

func cmdCMakeComment(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	if win.Mark.Line != 0 && win.Mark.Line != win.Cursor.Line {
		startLine := win.Mark.Line
		endLine := win.Cursor.Line
		if startLine > endLine {
			startLine, endLine = endLine, startLine
		}
		var endOffset uint
		if win.Mark.Line > win.Cursor.Line {
			endOffset = win.Mark.Offset
		} else {
			endOffset = buf.Line(endLine).Len()
		}
		orig := win.Cursor
		PackageHooks.BeginCommand()
		for ln := startLine; ln <= endLine; ln++ {
			lineLp := buf.Line(ln)
			if lineLp == nil {
				continue
			}
			start := lineLp.FirstNonblank()
			insLoc := buffer.MakeLocation(ln, start)
			if err := PackageHooks.SetText(buf, insLoc, insLoc, []byte("/*"), nil); err != nil {
				win.Cursor = orig
				PackageHooks.EndCommand()
				return false
			}
			lineLp = buf.Line(ln)
			closeLoc := buffer.MakeLocation(ln, lineLp.Len())
			if err := PackageHooks.SetText(buf, closeLoc, closeLoc, []byte("*/"), nil); err != nil {
				win.Cursor = orig
				PackageHooks.EndCommand()
				return false
			}
		}
		PackageHooks.EndCommand()
		win.Cursor.Line = endLine
		win.Cursor.Offset = endOffset
		win.DidMove = true
		return true
	}
	insLoc := win.Cursor
	cmt := []byte("  /* */")
	if err := PackageHooks.SetText(buf, insLoc, insLoc, cmt, nil); err != nil {
		return false
	}
	line := buf.Line(win.Cursor.Line)
	if line == nil {
		return false
	}
	if line.Len() < 3 {
		win.Cursor.Offset = line.Len()
	} else {
		win.Cursor.Offset = line.Len() - 3
	}
	win.DidMove = true
	return true
}

func cmdCTopOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	lineNumber := win.Cursor.Line
	if buf.LineCount == 0 {
		if PackageHooks.Message != nil {
			PackageHooks.Message("[Not in a function]")
		}
		return false
	}
	if lineNumber == buf.EOF() {
		lineNumber = buf.LineCount
	}
	depth := 0
	for ln := int(lineNumber); ln >= 1; ln-- {
		line := buf.Line(uint(ln))
		if line == nil {
			continue
		}
		found := false
		for i := len(line.Data) - 1; i >= 0; i-- {
			c := line.Data[i]
			if c == '}' {
				depth++
				continue
			}
			if c == '{' {
				if depth > 0 {
					depth--
					continue
				}
				if line.IndentColumn() == 0 {
					found = true
					break
				}
			}
		}
		if found {
			sigLine := uint(ln)
			for sigLine > 1 && buf.Line(sigLine-1).IsBlank() {
				sigLine--
			}
			if sigLine > 1 {
				sigLine--
			}
			win.SetCursor(buffer.MakeLocation(sigLine, 0))
			win.DidMove = true
			return true
		}
	}
	if PackageHooks.Message != nil {
		PackageHooks.Message("[Not in a function]")
	}
	return false
}

func cmdCEndOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	lineNumber := win.Cursor.Line
	if buf.LineCount == 0 {
		if PackageHooks.Message != nil {
			PackageHooks.Message("[Not in a function]")
		}
		return false
	}
	if lineNumber == buf.EOF() {
		lineNumber = buf.LineCount
	}
	depth := 0
	for ln := int(lineNumber); ln <= int(buf.LineCount); ln++ {
		line := buf.Line(uint(ln))
		if line == nil {
			continue
		}
		for i := 0; i < len(line.Data); i++ {
			c := line.Data[i]
			if c == '{' {
				depth++
			} else if c == '}' {
				if depth > 0 {
					depth--
					continue
				}
				win.SetCursor(buffer.MakeLocation(uint(ln), uint(i)))
				win.DidMove = true
				return true
			}
		}
	}
	if PackageHooks.Message != nil {
		PackageHooks.Message("[Not in a function]")
	}
	return false
}

func cmdCMarkFunction(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	if win == nil {
		return false
	}
	origLine := win.Cursor.Line
	origDoto := win.Cursor.Offset
	if !cmdCEndOfFunction(false, 1) {
		return false
	}
	win.Mark.Line = win.Cursor.Line
	win.Mark.Offset = win.Cursor.Offset
	win.SetCursor(buffer.MakeLocation(origLine, origDoto))
	if !cmdCTopOfFunction(false, 1) {
		win.Mark.Line = 0
		return false
	}
	win.DidMove = true
	if PackageHooks.Message != nil {
		PackageHooks.Message("[Function marked]")
	}
	return true
}

func cmdCCloseBrace(f bool, n int) bool {
	_ = f
	if n <= 0 {
		n = 1
	}
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	col := calcIndent(buf, win.Cursor.Line)
	setLineIndent(win, col)
	for i := 0; i < n; i++ {
		if err := window.InsertCodepoint(win, '}'); err != nil {
			return false
		}
	}
	line := buf.Line(win.Cursor.Line)
	if line == nil {
		return false
	}
	win.Cursor.Offset = uint(col + n)
	if win.Cursor.Offset > line.Len() {
		win.Cursor.Offset = line.Len()
	}
	win.DidEdit = true
	return true
}

func init() {
	for i := range modeTable {
		switch modeTable[i].Mode {
		case buffer.LModeC, buffer.LModeJava, buffer.LModeCSharp, buffer.LModeKotlin, buffer.LModeSwift, buffer.LModeJavaScript, buffer.LModeTypeScript, buffer.LModeActionScript, buffer.LModeDart, buffer.LModeRust:
			modeTable[i].NewlineAndIndent = cmdCNewlineAndIndent
			modeTable[i].IndentLine = cmdCIndentLine
			modeTable[i].CloseBrace = cmdCCloseBrace
			modeTable[i].MakeComment = cmdCMakeComment
			modeTable[i].TopOfFunction = cmdCTopOfFunction
			modeTable[i].EndOfFunction = cmdCEndOfFunction
			modeTable[i].MarkFunction = cmdCMarkFunction
		}
	}
}

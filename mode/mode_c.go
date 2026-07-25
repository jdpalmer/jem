package mode

import (
	"bytes"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/window"
)

// codeScanState tracks C-like string/comment context while scanning for delimiters.
// Indent must ignore brackets inside "...", '...', //, and /* */ (e.g. term.CTLX | '(').
type codeScanState uint8

const (
	codeScanCode codeScanState = iota
	codeScanString
	codeScanStringEsc
	codeScanChar
	codeScanCharEsc
	codeScanLineCmt
	codeScanBlockCmt
	codeScanBlockCmtStar
)

func codeScanStep(state codeScanState, c, next byte) (codeScanState, bool) {
	switch state {
	case codeScanString:
		switch c {
		case '\\':
			return codeScanStringEsc, false
		case '"':
			return codeScanCode, false
		default:
			return codeScanString, false
		}
	case codeScanStringEsc:
		return codeScanString, false
	case codeScanChar:
		switch c {
		case '\\':
			return codeScanCharEsc, false
		case '\'':
			return codeScanCode, false
		default:
			return codeScanChar, false
		}
	case codeScanCharEsc:
		return codeScanChar, false
	case codeScanLineCmt:
		return codeScanLineCmt, false
	case codeScanBlockCmt:
		if c == '*' {
			return codeScanBlockCmtStar, false
		}
		return codeScanBlockCmt, false
	case codeScanBlockCmtStar:
		switch c {
		case '/':
			return codeScanCode, false
		case '*':
			return codeScanBlockCmtStar, false
		default:
			return codeScanBlockCmt, false
		}
	default: // codeScanCode
		switch c {
		case '"':
			return codeScanString, false
		case '\'':
			return codeScanChar, false
		case '/':
			if next == '/' {
				return codeScanLineCmt, false
			}
			if next == '*' {
				return codeScanBlockCmt, false
			}
			return codeScanCode, true
		default:
			return codeScanCode, true
		}
	}
}

// scanDelimStacks walks code from the start of buf through endOffset on endLine
// (exclusive), pushing/popping structural (), [], {} while skipping strings and comments.
func scanDelimStacks(buf *buffer.Buffer, endLine, endOffset int) (parens, brackets, braces []buffer.Location) {
	if endLine < 1 {
		return nil, nil, nil
	}
	state := codeScanCode
	for ln := 1; ln <= endLine; ln++ {
		if state == codeScanLineCmt {
			state = codeScanCode
		}
		line := buf.Line(ln)
		if line == nil {
			continue
		}
		limit := len(line.Data)
		if ln == endLine {
			if endOffset < limit {
				limit = endOffset
			}
			if limit < 0 {
				limit = 0
			}
		}
		for i := 0; i < limit; i++ {
			c := line.Data[i]
			next := byte(0)
			if i+1 < len(line.Data) {
				next = line.Data[i+1]
			}
			var inCode bool
			state, inCode = codeScanStep(state, c, next)
			if !inCode {
				// Consume the second byte of // or /* starters.
				if c == '/' && (next == '/' || next == '*') &&
					(state == codeScanLineCmt || state == codeScanBlockCmt) {
					i++
				}
				continue
			}
			switch c {
			case '(':
				parens = append(parens, buffer.MakeLocation(ln, i))
			case ')':
				if n := len(parens); n > 0 {
					parens = parens[:n-1]
				}
			case '[':
				brackets = append(brackets, buffer.MakeLocation(ln, i))
			case ']':
				if n := len(brackets); n > 0 {
					brackets = brackets[:n-1]
				}
			case '{':
				braces = append(braces, buffer.MakeLocation(ln, i))
			case '}':
				if n := len(braces); n > 0 {
					braces = braces[:n-1]
				}
			}
		}
	}
	return parens, brackets, braces
}

func lineColOfOffset(line *buffer.Line, offset int) int {
	if offset > line.Len() {
		offset = line.Len()
	}
	col := 0
	for i := 0; i < offset; i++ {
		c := line.Data[i]
		if c == '\t' {
			col += 8 - (col % 8)
		} else {
			col++
		}
	}
	return col
}

func lineFirstNonblankOffset(line *buffer.Line) (int, byte) {
	for i := 0; i < line.Len(); i++ {
		c := line.Data[i]
		if c != ' ' && c != '\t' {
			return i, c
		}
	}
	return line.Len(), 0
}

func lineStartsWith(line *buffer.Line, text string) bool {
	off, _ := lineFirstNonblankOffset(line)
	pat := []byte(text)
	if off >= line.Len() {
		return false
	}
	if off+len(pat) > line.Len() {
		return false
	}
	return bytes.Equal(line.Data[off:off+len(pat)], pat)
}

func lineIsCommentOrPreproc(buf *buffer.Buffer, lineNumber int) bool {
	line := buf.Line(lineNumber)
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

func lineIsPreproc(buf *buffer.Buffer, lineNumber int) bool {
	line := buf.Line(lineNumber)
	if line.IsBlank() {
		return false
	}
	_, ch := lineFirstNonblankOffset(line)
	return ch == '#'
}

func prevCodeLineNumber(buf *buffer.Buffer, lineNumber int) int {
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
	if line.Len() == 0 {
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

func calcCommentIndent(buf *buffer.Buffer, lineNumber int) int {
	prevLine := lineNumber
	for prevLine > 1 {
		prevLine--
		prev := buf.Line(prevLine)
		if prev == nil || prev.IsBlank() {
			continue
		}
		if lineStartsWith(prev, "/*") {
			return prev.IndentColumn() + 1
		}
		if prev.FirstNonblank() < prev.Len() {
			ch := prev.Data[prev.FirstNonblank()]
			if ch == '*' || lineStartsWith(prev, "*/") {
				return prev.IndentColumn()
			}
		}
		break
	}
	return 0
}

func findCaseIndent(buf *buffer.Buffer, lineNumber int, offset int) int {
	cIndent := buf.Indent.Width
	cColonOffset := buf.Indent.Label
	for ln := lineNumber; ln >= 1; ln-- {
		line := buf.Line(ln)
		if line == nil {
			continue
		}
		if bytes.IndexByte(line.Data, '{') != -1 {
			base := line.IndentColumn() + cIndent + cColonOffset
			if base < 0 {
				return 0
			}
			return base
		}
	}
	return 0
}

func findClosingDelimiterIndent(buf *buffer.Buffer, lineNumber int, offset int) int {
	line := buf.Line(lineNumber)
	if line == nil || offset >= line.Len() {
		return 0
	}
	ch := line.Data[offset]
	parens, brackets, braces := scanDelimStacks(buf, lineNumber, offset)
	var open buffer.Location
	var ok bool
	switch ch {
	case '}':
		if n := len(braces); n > 0 {
			open, ok = braces[n-1], true
		}
		if !ok {
			return 0
		}
		if ol := buf.Line(open.Line); ol != nil {
			return ol.IndentColumn()
		}
		return 0
	case ')':
		if n := len(parens); n > 0 {
			open, ok = parens[n-1], true
		}
	case ']':
		if n := len(brackets); n > 0 {
			open, ok = brackets[n-1], true
		}
	default:
		return 0
	}
	if !ok {
		return 0
	}
	if ol := buf.Line(open.Line); ol != nil {
		return lineColOfOffset(ol, open.Offset)
	}
	return 0
}

func findUnmatchedOpenDelim(buf *buffer.Buffer, lineNumber, offset int) (buffer.Location, byte, bool) {
	parens, brackets, _ := scanDelimStacks(buf, lineNumber, offset)
	var best buffer.Location
	var kind byte
	found := false
	if n := len(parens); n > 0 {
		best, kind, found = parens[n-1], '(', true
	}
	if n := len(brackets); n > 0 {
		loc := brackets[n-1]
		if !found || loc.Line > best.Line || (loc.Line == best.Line && loc.Offset > best.Offset) {
			best, kind, found = loc, '[', true
		}
	}
	return best, kind, found
}

func findDelimiterContinuationIndent(buf *buffer.Buffer, lineNumber int, offset int) int {
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
	return line.IndentColumn() + buf.Indent.Width
}

func findEnclosingBlockIndent(buf *buffer.Buffer, lineNumber int, offset int) int {
	open, ok := findUnmatchedOpenBrace(buf, lineNumber, offset)
	if !ok {
		return -1
	}
	line := buf.Line(open.Line)
	if line == nil {
		return -1
	}
	return line.IndentColumn() + buf.Indent.Width
}

func findUnmatchedOpenBrace(buf *buffer.Buffer, lineNumber, offset int) (buffer.Location, bool) {
	_, _, braces := scanDelimStacks(buf, lineNumber, offset)
	if n := len(braces); n > 0 {
		return braces[n-1], true
	}
	return buffer.Location{}, false
}

func calcIndent(buf *buffer.Buffer, lineNumber int) int {
	line := buf.Line(lineNumber)
	cIndent := buf.Indent.Width
	cBrace := buf.Indent.Brace
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
	ind := ref.IndentColumn()
	if ref.LastByte() == ':' && lineIsCaseLabel(ref) {
		ind += cIndent
	} else if lineEndsWithContinuation(ref) {
		ind += cIndent
	}
	if fc == '{' {
		ind += cBrace
	}
	if ind < 0 {
		return 0
	}
	return ind
}

func setLineIndent(win *window.Window, col int) bool {
	if win.Buffer == nil || col < 0 {
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
	beginEdit()
	err := window.SetText(buf, begin, end, spaces, nil)
	ok := err == nil
	endEdit()
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
	if win == nil {
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
	if win == nil {
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
	if win == nil {
		return false
	}
	if win.Mark.Line != 0 && win.Mark.Line != win.Cursor.Line {
		startLine := win.Mark.Line
		endLine := win.Cursor.Line
		if startLine > endLine {
			startLine, endLine = endLine, startLine
		}
		var endOffset int
		if win.Mark.Line > win.Cursor.Line {
			endOffset = win.Mark.Offset
		} else {
			endOffset = buf.Line(endLine).Len()
		}
		orig := win.Cursor
		beginEdit()
		for ln := startLine; ln <= endLine; ln++ {
			lineLp := buf.Line(ln)
			if lineLp == nil {
				continue
			}
			start := lineLp.FirstNonblank()
			insLoc := buffer.MakeLocation(ln, start)
			if err := window.SetText(buf, insLoc, insLoc, []byte("/*"), nil); err != nil {
				win.Cursor = orig
				endEdit()
				return false
			}
			lineLp = buf.Line(ln)
			closeLoc := buffer.MakeLocation(ln, lineLp.Len())
			if err := window.SetText(buf, closeLoc, closeLoc, []byte("*/"), nil); err != nil {
				win.Cursor = orig
				endEdit()
				return false
			}
		}
		endEdit()
		win.Cursor.Line = endLine
		win.Cursor.Offset = endOffset
		win.DidMove = true
		return true
	}
	insLoc := win.Cursor
	cmt := []byte("  /* */")
	if err := window.SetText(buf, insLoc, insLoc, cmt, nil); err != nil {
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
	if win == nil {
		return false
	}
	lineNumber := win.Cursor.Line
	if len(buf.Lines) == 0 {
		Message("[Not in a function]")
		return false
	}
	if lineNumber == buf.EOF() {
		lineNumber = len(buf.Lines)
	}
	depth := 0
	for ln := lineNumber; ln >= 1; ln-- {
		line := buf.Line(ln)
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
			sigLine := ln
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
	Message("[Not in a function]")
	return false
}

func cmdCEndOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if win == nil {
		return false
	}
	lineNumber := win.Cursor.Line
	if len(buf.Lines) == 0 {
		Message("[Not in a function]")
		return false
	}
	if lineNumber == buf.EOF() {
		lineNumber = len(buf.Lines)
	}
	depth := 0
	for ln := lineNumber; ln <= len(buf.Lines); ln++ {
		line := buf.Line(ln)
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
				win.SetCursor(buffer.MakeLocation(ln, i))
				win.DidMove = true
				return true
			}
		}
	}
	Message("[Not in a function]")
	return false
}

func cmdCMarkFunction(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
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
	Message("[Function marked]")
	return true
}

func cmdCCloseBrace(f bool, n int) bool {
	_ = f
	if n <= 0 {
		n = 1
	}
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if win == nil {
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
	win.Cursor.Offset = (col + n)
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

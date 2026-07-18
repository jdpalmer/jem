package modes

import (
	"bytes"

	"github.com/jdpalmer/jem/app"
)

func lineColOfOffset(lp *Line, offset uint) int {
	if lp == nil {
		return 0
	}
	if offset > LineLength(lp) {
		offset = LineLength(lp)
	}
	return int(offset)
}

func lineFirstNonblankOffset(lp *Line) (uint, byte) {
	if lp == nil {
		return 0, 0
	}
	for i := uint(0); i < LineLength(lp); i++ {
		c := lp.Data[i]
		if c != ' ' && c != '\t' {
			return i, c
		}
	}
	return LineLength(lp), 0
}

func lineStartsWith(lp *Line, text string) bool {
	if lp == nil {
		return false
	}
	off, _ := lineFirstNonblankOffset(lp)
	pat := []byte(text)
	if off >= LineLength(lp) {
		return false
	}
	if off+uint(len(pat)) > LineLength(lp) {
		return false
	}
	return bytes.Equal(lp.Data[off:off+uint(len(pat))], pat)
}

func lineIsCommentOrPreproc(bp *Buffer, lineNumber uint) bool {
	lp := BufferGetLine(bp, lineNumber)
	if lp == nil {
		return false
	}
	if line_is_blank(lp) {
		return false
	}
	off, ch := lineFirstNonblankOffset(lp)
	if off >= LineLength(lp) {
		return false
	}
	if ch == '*' || ch == '#' {
		return true
	}
	if ch == '/' && off+1 < LineLength(lp) {
		next := lp.Data[off+1]
		if next == '/' || next == '*' {
			return true
		}
	}
	return false
}

func lineIsPreproc(bp *Buffer, lineNumber uint) bool {
	lp := BufferGetLine(bp, lineNumber)
	if lp == nil || line_is_blank(lp) {
		return false
	}
	_, ch := lineFirstNonblankOffset(lp)
	return ch == '#'
}

func prevCodeLineNumber(bp *Buffer, lineNumber uint) uint {
	for ln := lineNumber; ln > 1; {
		ln--
		p := BufferGetLine(bp, ln)
		if p == nil {
			continue
		}
		if !line_is_blank(p) && !lineIsCommentOrPreproc(bp, ln) {
			return ln
		}
	}
	return 0
}

func lineIsCaseLabel(lp *Line) bool {
	if lp == nil {
		return false
	}
	off := line_first_nonblank(lp)
	if off >= LineLength(lp) {
		return false
	}
	if off+4 <= LineLength(lp) {
		if bytes.Equal(lp.Data[off:off+4], []byte("case")) {
			if off+4 == LineLength(lp) {
				return true
			}
			nc := lp.Data[off+4]
			return nc == ' ' || nc == '\t' || nc == '('
		}
	}
	if off+7 <= LineLength(lp) && bytes.Equal(lp.Data[off:off+7], []byte("default")) {
		return true
	}
	return false
}

func lineEndsWithContinuation(lp *Line) bool {
	if lp == nil || LineLength(lp) == 0 {
		return false
	}
	last := lp.Data[LineLength(lp)-1]
	switch last {
	case ',', '\\', '?', ':', '+', '-', '*', '/', '%', '&', '|', '^', '=', '<', '>', '!':
		return true
	default:
		return false
	}
}

func calcCommentIndent(bp *Buffer, lineNumber uint) int {
	prevLine := lineNumber
	for prevLine > 1 {
		prevLine--
		prev := BufferGetLine(bp, prevLine)
		if prev == nil || line_is_blank(prev) {
			continue
		}
		if lineStartsWith(prev, "/*") {
			return int(line_indent_column(prev)) + 1
		}
		if line_first_nonblank(prev) < LineLength(prev) {
			ch := prev.Data[line_first_nonblank(prev)]
			if ch == '*' || lineStartsWith(prev, "*/") {
				return int(line_indent_column(prev))
			}
		}
		break
	}
	return 0
}

func findCaseIndent(bp *Buffer, lineNumber uint, offset uint) int {
	cIndent := bp.CIndent
	cColonOffset := bp.CColonOffset
	for ln := int(lineNumber); ln >= 1; ln-- {
		line := BufferGetLine(bp, uint(ln))
		if line == nil {
			continue
		}
		if bytes.IndexByte(line.Data, '{') != -1 {
			base := int(line_indent_column(line)) + int(cIndent) + int(cColonOffset)
			if base < 0 {
				return 0
			}
			return base
		}
	}
	return 0
}

func findClosingDelimiterIndent(bp *Buffer, lineNumber uint, offset uint) int {
	line := BufferGetLine(bp, lineNumber)
	if line == nil || offset >= LineLength(line) {
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
	depth := 0
	for ln := int(lineNumber); ln >= 1; ln-- {
		lp := BufferGetLine(bp, uint(ln))
		if lp == nil {
			continue
		}
		start := len(lp.Data)
		if ln == int(lineNumber) {
			start = int(offset)
		}
		for i := start; i >= 0; i-- {
			if i == len(lp.Data) {
				continue
			}
			c := lp.Data[i]
			if c == ch {
				depth++
				continue
			}
			if c == open {
				if depth > 0 {
					depth--
					continue
				}
				return lineColOfOffset(lp, uint(i))
			}
		}
	}
	return 0
}

func findUnmatchedOpenDelim(bp *Buffer, lineNumber, offset uint) (Location, byte, bool) {
	depthParen := 0
	depthBracket := 0
	for ln := int(lineNumber); ln >= 1; ln-- {
		lp := BufferGetLine(bp, uint(ln))
		if lp == nil {
			continue
		}
		limit := len(lp.Data)
		if ln == int(lineNumber) {
			limit = int(offset)
		}
		for i := limit - 1; i >= 0; i-- {
			switch lp.Data[i] {
			case ')':
				depthParen++
			case '(':
				if depthParen > 0 {
					depthParen--
				} else {
					return MakeLocation(uint(ln), uint(i)), '(', true
				}
			case ']':
				depthBracket++
			case '[':
				if depthBracket > 0 {
					depthBracket--
				} else {
					return MakeLocation(uint(ln), uint(i)), '[', true
				}
			}
		}
	}
	return Location{}, 0, false
}

func findDelimiterContinuationIndent(bp *Buffer, lineNumber uint, offset uint) int {
	open, _, ok := findUnmatchedOpenDelim(bp, lineNumber, offset)
	if !ok {
		return -1
	}
	lp := BufferGetLine(bp, open.Line)
	if lp == nil {
		return -1
	}
	tail := open.Offset + 1
	for tail < LineLength(lp) {
		ch := lp.Data[tail]
		if ch != ' ' && ch != '\t' {
			return lineColOfOffset(lp, tail)
		}
		tail++
	}
	return int(line_indent_column(lp)) + int(bp.CIndent)
}

func findEnclosingBlockIndent(bp *Buffer, lineNumber uint, offset uint) int {
	open, ok := findUnmatchedOpenBrace(bp, lineNumber, offset)
	if !ok {
		return -1
	}
	lp := BufferGetLine(bp, open.Line)
	if lp == nil {
		return -1
	}
	return int(line_indent_column(lp)) + int(bp.CIndent)
}

func findUnmatchedOpenBrace(bp *Buffer, lineNumber, offset uint) (Location, bool) {
	depth := 0
	for ln := int(lineNumber); ln >= 1; ln-- {
		lp := BufferGetLine(bp, uint(ln))
		if lp == nil {
			continue
		}
		limit := len(lp.Data)
		if ln == int(lineNumber) {
			limit = int(offset)
		}
		for i := limit - 1; i >= 0; i-- {
			switch lp.Data[i] {
			case '}':
				depth++
			case '{':
				if depth > 0 {
					depth--
				} else {
					return MakeLocation(uint(ln), uint(i)), true
				}
			}
		}
	}
	return Location{}, false
}

func calcIndent(bp *Buffer, lineNumber uint) int {
	lp := BufferGetLine(bp, lineNumber)
	if lp == nil {
		return 0
	}
	cIndent := bp.CIndent
	cBrace := bp.CBrace
	first, fc := lineFirstNonblankOffset(lp)
	if lineIsPreproc(bp, lineNumber) {
		return 0
	}
	if !line_is_blank(lp) {
		if fc == '*' || fc == '/' {
			return calcCommentIndent(bp, lineNumber)
		}
	}
	if fc == '}' || fc == ')' || fc == ']' {
		return findClosingDelimiterIndent(bp, lineNumber, first)
	}
	if lineIsCaseLabel(lp) {
		return findCaseIndent(bp, lineNumber, first)
	}
	indent := findDelimiterContinuationIndent(bp, lineNumber, first)
	if indent >= 0 {
		return indent
	}
	indent = findEnclosingBlockIndent(bp, lineNumber, first)
	if indent >= 0 {
		return indent
	}
	refLine := prevCodeLineNumber(bp, lineNumber)
	if refLine == 0 {
		return 0
	}
	ref := BufferGetLine(bp, refLine)
	if ref == nil {
		return 0
	}
	ind := int(line_indent_column(ref))
	if line_last_byte(ref) == ':' && lineIsCaseLabel(ref) {
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

func setLineIndent(wp *Window, col int) bool {
	if wp == nil || wp.Buffer == nil || col < 0 || PackageHooks.BufferSetText == nil {
		return false
	}
	bp := wp.Buffer
	ln := wp.Cursor.Line
	lp := BufferGetLine(bp, ln)
	if lp == nil {
		return false
	}
	oldFirst := line_first_nonblank(lp)
	spaces := make([]byte, col)
	for i := range spaces {
		spaces[i] = ' '
	}
	begin := MakeLocation(ln, 0)
	end := MakeLocation(ln, oldFirst)
	if PackageHooks.UndoBeginCommand != nil {
		PackageHooks.UndoBeginCommand()
	}
	ok := PackageHooks.BufferSetText(bp, begin, end, spaces, uint(len(spaces)), nil, false)
	if PackageHooks.UndoEndCommand != nil {
		PackageHooks.UndoEndCommand()
	}
	if ok {
		wp.DidEdit = true
	}
	return ok
}

func cmdCNewlineAndIndent(f bool, n int) bool {
	_ = f
	if n < 0 {
		return false
	}
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil || PackageHooks.WindowInsertNewline == nil {
		return false
	}
	for i := 0; i < n; i++ {
		if !PackageHooks.WindowInsertNewline(wp) {
			return false
		}
		indent := calcIndent(bp, wp.Cursor.Line)
		setLineIndent(wp, indent)
	}
	return true
}

func cmdCIndentLine(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	col := calcIndent(bp, wp.Cursor.Line)
	setLineIndent(wp, col)
	wp.DidEdit = true
	return true
}

func cmdCMakeComment(f bool, n int) bool {
	_ = f
	_ = n
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil || PackageHooks.BufferSetText == nil {
		return false
	}
	if wp.Mark.Line != 0 && wp.Mark.Line != wp.Cursor.Line {
		startLine := wp.Mark.Line
		endLine := wp.Cursor.Line
		if startLine > endLine {
			startLine, endLine = endLine, startLine
		}
		var endOffset uint
		if wp.Mark.Line > wp.Cursor.Line {
			endOffset = wp.Mark.Offset
		} else {
			endOffset = LineLength(BufferGetLine(bp, endLine))
		}
		orig := wp.Cursor
		if PackageHooks.UndoBeginCommand != nil {
			PackageHooks.UndoBeginCommand()
		}
		for ln := startLine; ln <= endLine; ln++ {
			lineLp := BufferGetLine(bp, ln)
			if lineLp == nil {
				continue
			}
			start := line_first_nonblank(lineLp)
			insLoc := MakeLocation(ln, start)
			if !PackageHooks.BufferSetText(bp, insLoc, insLoc, []byte("/*"), 2, nil, false) {
				wp.Cursor = orig
				if PackageHooks.UndoEndCommand != nil {
					PackageHooks.UndoEndCommand()
				}
				return false
			}
			lineLp = BufferGetLine(bp, ln)
			closeLoc := MakeLocation(ln, LineLength(lineLp))
			if !PackageHooks.BufferSetText(bp, closeLoc, closeLoc, []byte("*/"), 2, nil, false) {
				wp.Cursor = orig
				if PackageHooks.UndoEndCommand != nil {
					PackageHooks.UndoEndCommand()
				}
				return false
			}
		}
		if PackageHooks.UndoEndCommand != nil {
			PackageHooks.UndoEndCommand()
		}
		wp.Cursor.Line = endLine
		wp.Cursor.Offset = endOffset
		wp.DidMove = true
		return true
	}
	insLoc := wp.Cursor
	cmt := []byte("  /* */")
	if !PackageHooks.BufferSetText(bp, insLoc, insLoc, cmt, uint(len(cmt)), nil, false) {
		return false
	}
	lp := BufferGetLine(bp, wp.Cursor.Line)
	if lp == nil {
		return false
	}
	if LineLength(lp) < 3 {
		wp.Cursor.Offset = LineLength(lp)
	} else {
		wp.Cursor.Offset = LineLength(lp) - 3
	}
	wp.DidMove = true
	return true
}

func cmdCTopOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	lineNumber := wp.Cursor.Line
	if bp.LineCount == 0 {
		if PackageHooks.Message != nil {
			PackageHooks.Message("[Not in a function]")
		}
		return false
	}
	if lineNumber == BufferEOF(bp) {
		lineNumber = bp.LineCount
	}
	depth := 0
	for ln := int(lineNumber); ln >= 1; ln-- {
		lp := BufferGetLine(bp, uint(ln))
		if lp == nil {
			continue
		}
		found := false
		for i := len(lp.Data) - 1; i >= 0; i-- {
			c := lp.Data[i]
			if c == '}' {
				depth++
				continue
			}
			if c == '{' {
				if depth > 0 {
					depth--
					continue
				}
				if line_indent_column(lp) == 0 {
					found = true
					break
				}
			}
		}
		if found {
			sigLine := uint(ln)
			for sigLine > 1 && line_is_blank(BufferGetLine(bp, sigLine-1)) {
				sigLine--
			}
			if sigLine > 1 {
				sigLine--
			}
			if PackageHooks.WindowSetCursor != nil {
				PackageHooks.WindowSetCursor(wp, MakeLocation(sigLine, 0))
			}
			wp.DidMove = true
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
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	lineNumber := wp.Cursor.Line
	if bp.LineCount == 0 {
		if PackageHooks.Message != nil {
			PackageHooks.Message("[Not in a function]")
		}
		return false
	}
	if lineNumber == BufferEOF(bp) {
		lineNumber = bp.LineCount
	}
	depth := 0
	for ln := int(lineNumber); ln <= int(bp.LineCount); ln++ {
		lp := BufferGetLine(bp, uint(ln))
		if lp == nil {
			continue
		}
		for i := 0; i < len(lp.Data); i++ {
			c := lp.Data[i]
			if c == '{' {
				depth++
			} else if c == '}' {
				if depth > 0 {
					depth--
					continue
				}
				if PackageHooks.WindowSetCursor != nil {
					PackageHooks.WindowSetCursor(wp, MakeLocation(uint(ln), uint(i)))
				}
				wp.DidMove = true
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
	wp := app.State.CurrentWindow
	if wp == nil {
		return false
	}
	origLine := wp.Cursor.Line
	origDoto := wp.Cursor.Offset
	if !cmdCEndOfFunction(false, 1) {
		return false
	}
	wp.Mark.Line = wp.Cursor.Line
	wp.Mark.Offset = wp.Cursor.Offset
	if PackageHooks.WindowSetCursor != nil {
		PackageHooks.WindowSetCursor(wp, MakeLocation(origLine, origDoto))
	}
	if !cmdCTopOfFunction(false, 1) {
		wp.Mark.Line = 0
		return false
	}
	wp.DidMove = true
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
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil || PackageHooks.WindowInsertCodepoint == nil {
		return false
	}
	col := calcIndent(bp, wp.Cursor.Line)
	setLineIndent(wp, col)
	for i := 0; i < n; i++ {
		if !PackageHooks.WindowInsertCodepoint(wp, '}') {
			return false
		}
	}
	lp := BufferGetLine(bp, wp.Cursor.Line)
	if lp == nil {
		return false
	}
	wp.Cursor.Offset = uint(col + n)
	if wp.Cursor.Offset > LineLength(lp) {
		wp.Cursor.Offset = LineLength(lp)
	}
	wp.DidEdit = true
	return true
}

func init() {
	for i := range modeTable {
		switch modeTable[i].Mode {
		case app.LModeC, app.LModeJava, app.LModeCSharp, app.LModeGo, app.LModeKotlin, app.LModeSwift, app.LModeJavaScript, app.LModeTypeScript, app.LModeActionScript, app.LModeDart, app.LModeRust:
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

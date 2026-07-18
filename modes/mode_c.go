package modes

import (
	"bytes"

	"github.com/jdpalmer/jem/app"
)

func lineColOfOffset(lp *Line, offset uint) int {
	if lp == nil {
		return 0
	}
	if offset > lp.Len() {
		offset = lp.Len()
	}
	return int(offset)
}

func lineFirstNonblankOffset(lp *Line) (uint, byte) {
	if lp == nil {
		return 0, 0
	}
	for i := uint(0); i < lp.Len(); i++ {
		c := lp.Data[i]
		if c != ' ' && c != '\t' {
			return i, c
		}
	}
	return lp.Len(), 0
}

func lineStartsWith(lp *Line, text string) bool {
	if lp == nil {
		return false
	}
	off, _ := lineFirstNonblankOffset(lp)
	pat := []byte(text)
	if off >= lp.Len() {
		return false
	}
	if off+uint(len(pat)) > lp.Len() {
		return false
	}
	return bytes.Equal(lp.Data[off:off+uint(len(pat))], pat)
}

func lineIsCommentOrPreproc(bp *Buffer, lineNumber uint) bool {
	lp := bp.Line(lineNumber)
	if lp == nil {
		return false
	}
	if lp.IsBlank() {
		return false
	}
	off, ch := lineFirstNonblankOffset(lp)
	if off >= lp.Len() {
		return false
	}
	if ch == '*' || ch == '#' {
		return true
	}
	if ch == '/' && off+1 < lp.Len() {
		next := lp.Data[off+1]
		if next == '/' || next == '*' {
			return true
		}
	}
	return false
}

func lineIsPreproc(bp *Buffer, lineNumber uint) bool {
	lp := bp.Line(lineNumber)
	if lp == nil || lp.IsBlank() {
		return false
	}
	_, ch := lineFirstNonblankOffset(lp)
	return ch == '#'
}

func prevCodeLineNumber(bp *Buffer, lineNumber uint) uint {
	for ln := lineNumber; ln > 1; {
		ln--
		p := bp.Line(ln)
		if p == nil {
			continue
		}
		if !p.IsBlank() && !lineIsCommentOrPreproc(bp, ln) {
			return ln
		}
	}
	return 0
}

func lineIsCaseLabel(lp *Line) bool {
	if lp == nil {
		return false
	}
	off := lp.FirstNonblank()
	if off >= lp.Len() {
		return false
	}
	if off+4 <= lp.Len() {
		if bytes.Equal(lp.Data[off:off+4], []byte("case")) {
			if off+4 == lp.Len() {
				return true
			}
			nc := lp.Data[off+4]
			return nc == ' ' || nc == '\t' || nc == '('
		}
	}
	if off+7 <= lp.Len() && bytes.Equal(lp.Data[off:off+7], []byte("default")) {
		return true
	}
	return false
}

func lineEndsWithContinuation(lp *Line) bool {
	if lp == nil || lp.Len() == 0 {
		return false
	}
	last := lp.Data[lp.Len()-1]
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
		prev := bp.Line(prevLine)
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

func findCaseIndent(bp *Buffer, lineNumber uint, offset uint) int {
	cIndent := bp.CIndent
	cColonOffset := bp.CColonOffset
	for ln := int(lineNumber); ln >= 1; ln-- {
		line := bp.Line(uint(ln))
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

func findClosingDelimiterIndent(bp *Buffer, lineNumber uint, offset uint) int {
	line := bp.Line(lineNumber)
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
	depth := 0
	for ln := int(lineNumber); ln >= 1; ln-- {
		lp := bp.Line(uint(ln))
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
		lp := bp.Line(uint(ln))
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
	lp := bp.Line(open.Line)
	if lp == nil {
		return -1
	}
	tail := open.Offset + 1
	for tail < lp.Len() {
		ch := lp.Data[tail]
		if ch != ' ' && ch != '\t' {
			return lineColOfOffset(lp, tail)
		}
		tail++
	}
	return int(lp.IndentColumn()) + int(bp.CIndent)
}

func findEnclosingBlockIndent(bp *Buffer, lineNumber uint, offset uint) int {
	open, ok := findUnmatchedOpenBrace(bp, lineNumber, offset)
	if !ok {
		return -1
	}
	lp := bp.Line(open.Line)
	if lp == nil {
		return -1
	}
	return int(lp.IndentColumn()) + int(bp.CIndent)
}

func findUnmatchedOpenBrace(bp *Buffer, lineNumber, offset uint) (Location, bool) {
	depth := 0
	for ln := int(lineNumber); ln >= 1; ln-- {
		lp := bp.Line(uint(ln))
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
	lp := bp.Line(lineNumber)
	if lp == nil {
		return 0
	}
	cIndent := bp.CIndent
	cBrace := bp.CBrace
	first, fc := lineFirstNonblankOffset(lp)
	if lineIsPreproc(bp, lineNumber) {
		return 0
	}
	if !lp.IsBlank() {
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
	ref := bp.Line(refLine)
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

func setLineIndent(wp *Window, col int) bool {
	if wp == nil || wp.Buffer == nil || col < 0 || PackageHooks.BufferSetText == nil {
		return false
	}
	bp := wp.Buffer
	ln := wp.Cursor.Line
	lp := bp.Line(ln)
	if lp == nil {
		return false
	}
	oldFirst := lp.FirstNonblank()
	spaces := make([]byte, col)
	for i := range spaces {
		spaces[i] = ' '
	}
	begin := MakeLocation(ln, 0)
	end := MakeLocation(ln, oldFirst)
	if PackageHooks.UndoBeginCommand != nil {
		PackageHooks.UndoBeginCommand()
	}
	ok := PackageHooks.BufferSetText(bp, begin, end, spaces, nil, false)
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
			endOffset = bp.Line(endLine).Len()
		}
		orig := wp.Cursor
		if PackageHooks.UndoBeginCommand != nil {
			PackageHooks.UndoBeginCommand()
		}
		for ln := startLine; ln <= endLine; ln++ {
			lineLp := bp.Line(ln)
			if lineLp == nil {
				continue
			}
			start := lineLp.FirstNonblank()
			insLoc := MakeLocation(ln, start)
			if !PackageHooks.BufferSetText(bp, insLoc, insLoc, []byte("/*"), nil, false) {
				wp.Cursor = orig
				if PackageHooks.UndoEndCommand != nil {
					PackageHooks.UndoEndCommand()
				}
				return false
			}
			lineLp = bp.Line(ln)
			closeLoc := MakeLocation(ln, lineLp.Len())
			if !PackageHooks.BufferSetText(bp, closeLoc, closeLoc, []byte("*/"), nil, false) {
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
	if !PackageHooks.BufferSetText(bp, insLoc, insLoc, cmt, nil, false) {
		return false
	}
	lp := bp.Line(wp.Cursor.Line)
	if lp == nil {
		return false
	}
	if lp.Len() < 3 {
		wp.Cursor.Offset = lp.Len()
	} else {
		wp.Cursor.Offset = lp.Len() - 3
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
	if lineNumber == bp.EOF() {
		lineNumber = bp.LineCount
	}
	depth := 0
	for ln := int(lineNumber); ln >= 1; ln-- {
		lp := bp.Line(uint(ln))
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
				if lp.IndentColumn() == 0 {
					found = true
					break
				}
			}
		}
		if found {
			sigLine := uint(ln)
			for sigLine > 1 && bp.Line(sigLine-1).IsBlank() {
				sigLine--
			}
			if sigLine > 1 {
				sigLine--
			}
				wp.SetCursor(MakeLocation(sigLine, 0))
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
	if lineNumber == bp.EOF() {
		lineNumber = bp.LineCount
	}
	depth := 0
	for ln := int(lineNumber); ln <= int(bp.LineCount); ln++ {
		lp := bp.Line(uint(ln))
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
					wp.SetCursor(MakeLocation(uint(ln), uint(i)))
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
		wp.SetCursor(MakeLocation(origLine, origDoto))
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
	lp := bp.Line(wp.Cursor.Line)
	if lp == nil {
		return false
	}
	wp.Cursor.Offset = uint(col + n)
	if wp.Cursor.Offset > lp.Len() {
		wp.Cursor.Offset = lp.Len()
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

package modes

import (
	"bytes"

	"github.com/jdpalmer/jem/app"
)

func lpKeyword(lp *Line, i uint, kw string) bool {
	if lp == nil {
		return false
	}
	klen := uint(len(kw))
	if i+klen > LineLength(lp) {
		return false
	}
	if !bytes.Equal(lp.Data[i:i+klen], []byte(kw)) {
		return false
	}
	after := i + klen
	if after >= LineLength(lp) {
		return true
	}
	nc := lp.Data[after]
	return nc == ' ' || nc == '\t' || nc == ':' || nc == '(' || nc == '\r'
}

func prevCodeLineNumberPy(bp *Buffer, lineNumber uint) uint {
	for lineNumber > 1 {
		lineNumber--
		p := BufferGetLine(bp, lineNumber)
		if p == nil {
			continue
		}
		if !line_is_blank(p) && line_first_byte(p) != '#' {
			return lineNumber
		}
	}
	return 0
}

func lineIsDeindentKw(lp *Line) bool {
	if lp == nil {
		return false
	}
	i := line_first_nonblank(lp)
	return lpKeyword(lp, i, "elif") || lpKeyword(lp, i, "else") || lpKeyword(lp, i, "except") || lpKeyword(lp, i, "finally")
}

func lineIsDedentStmt(lp *Line) bool {
	if lp == nil {
		return false
	}
	i := line_first_nonblank(lp)
	return lpKeyword(lp, i, "return") || lpKeyword(lp, i, "break") || lpKeyword(lp, i, "continue") || lpKeyword(lp, i, "raise") || lpKeyword(lp, i, "pass")
}

func calcIndentPy(bp *Buffer, lineNumber uint) int {
	lp := BufferGetLine(bp, lineNumber)
	pyIndentWidth := bp.PyIndent
	pyContinuedOffset := bp.PyContinuedOffset

	if lp != nil && lineIsDeindentKw(lp) {
		refLine := prevCodeLineNumberPy(bp, lineNumber)
		if refLine == 0 {
			return 0
		}
		ref := BufferGetLine(bp, refLine)
		if ref == nil {
			return 0
		}
		ind := int(line_indent_column(ref)) - int(pyIndentWidth)
		if ind < 0 {
			return 0
		}
		return ind
	}

	refLine := prevCodeLineNumberPy(bp, lineNumber)
	if refLine == 0 {
		return 0
	}

	baseLine := refLine
	for baseLine > 1 {
		l := BufferGetLine(bp, baseLine)
		if l == nil {
			break
		}
		if line_last_byte(l) != '\\' {
			break
		}
		nextRef := prevCodeLineNumberPy(bp, baseLine)
		if nextRef == 0 {
			break
		}
		baseLine = nextRef
	}

	ref := BufferGetLine(bp, baseLine)
	if ref == nil {
		return 0
	}
	refInd := int(line_indent_column(ref))
	last := line_last_byte(BufferGetLine(bp, refLine))

	if last == '\\' {
		return refInd + int(pyContinuedOffset)
	}
	if last == ':' {
		return refInd + int(pyIndentWidth)
	}
	if last == '(' || last == '[' || last == '{' {
		return refInd + int(pyIndentWidth)
	}

	prev := BufferGetLine(bp, refLine)
	if prev != nil && lineIsDedentStmt(prev) {
		curInd := int(line_indent_column(prev))
		ln := prevCodeLineNumberPy(bp, refLine)
		for ln != 0 {
			p := BufferGetLine(bp, ln)
			if p == nil {
				ln = prevCodeLineNumberPy(bp, ln)
				continue
			}
			ind := int(line_indent_column(p))
			if ind < curInd {
				if ind < 0 {
					return 0
				}
				return ind
			}
			ln = prevCodeLineNumberPy(bp, ln)
		}
		return 0
	}
	return refInd
}

func setLineIndentPy(wp *Window, col int) bool {
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

func findDefLineNumber(bp *Buffer, lineNumber uint) uint {
	if bp.LineCount == 0 {
		return 0
	}
	if lineNumber == BufferEOF(bp) {
		lineNumber = bp.LineCount
	}
	lp := BufferGetLine(bp, lineNumber)
	if lp == nil {
		return 0
	}
	i := line_first_nonblank(lp)
	if lpKeyword(lp, i, "def") || lpKeyword(lp, i, "class") {
		return lineNumber
	}
	curInd := int(line_indent_column(lp))
	for lineNumber > 1 {
		lineNumber--
		lp = BufferGetLine(bp, lineNumber)
		if lp == nil {
			continue
		}
		if !line_is_blank(lp) {
			ind := int(line_indent_column(lp))
			if ind < curInd {
				j := line_first_nonblank(lp)
				if lpKeyword(lp, j, "def") || lpKeyword(lp, j, "class") {
					return lineNumber
				}
				curInd = ind
			}
		}
	}
	return 0
}

func cmdPyNewlineAndIndent(f bool, n int) bool {
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
		indent := calcIndentPy(bp, wp.Cursor.Line)
		setLineIndentPy(wp, indent)
	}
	return true
}

func cmdPyIndentLine(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	col := calcIndentPy(bp, wp.Cursor.Line)
	setLineIndentPy(wp, col)
	wp.DidEdit = true
	return true
}

func cmdPyMakeComment(f bool, n int) bool {
	_ = f
	_ = n
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil || PackageHooks.WindowInsertText == nil {
		return false
	}
	lp := BufferGetLine(bp, wp.Cursor.Line)
	if lp != nil {
		for i := uint(0); i < LineLength(lp); i++ {
			if LineGetc(lp, i) == '#' {
				wp.Cursor.Offset = i + 1
				if wp.Cursor.Offset < LineLength(lp) && LineGetc(lp, wp.Cursor.Offset) == ' ' {
					wp.Cursor.Offset++
				}
				wp.DidMove = true
				return true
			}
		}
	}
	if lp != nil {
		wp.Cursor.Offset = LineLength(lp)
	} else {
		wp.Cursor.Offset = 0
	}
	cmt := []byte("  # ")
	if !PackageHooks.WindowInsertText(wp, cmt, len(cmt)) {
		return false
	}
	wp.DidMove = true
	return true
}

func cmdPyTopOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	defLine := findDefLineNumber(bp, wp.Cursor.Line)
	if defLine == 0 {
		if PackageHooks.Message != nil {
			PackageHooks.Message("[Not in a function]")
		}
		return false
	}
	if PackageHooks.WindowSetCursor != nil {
		PackageHooks.WindowSetCursor(wp, MakeLocation(defLine, 0))
	}
	wp.DidMove = true
	return true
}

func cmdPyEndOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	defLine := findDefLineNumber(bp, wp.Cursor.Line)
	if defLine == 0 {
		if PackageHooks.Message != nil {
			PackageHooks.Message("[Not in a function]")
		}
		return false
	}
	def := BufferGetLine(bp, defLine)
	if def == nil {
		return false
	}
	defInd := int(line_indent_column(def))
	lineNumber := defLine + 1
	lastLine := defLine
	for lineNumber <= bp.LineCount {
		lp := BufferGetLine(bp, lineNumber)
		if !line_is_blank(lp) {
			if int(line_indent_column(lp)) <= defInd {
				break
			}
			lastLine = lineNumber
		}
		lineNumber++
	}
	targetLine := lastLine
	if targetLine == 0 {
		targetLine = defLine
	}
	lp := BufferGetLine(bp, targetLine)
	off := uint(0)
	if lp != nil {
		off = LineLength(lp)
	}
	if PackageHooks.WindowSetCursor != nil {
		PackageHooks.WindowSetCursor(wp, MakeLocation(targetLine, off))
	}
	wp.DidMove = true
	return true
}

func cmdPyMarkFunction(f bool, n int) bool {
	_ = f
	_ = n
	wp := app.State.CurrentWindow
	if wp == nil {
		return false
	}
	origLine := wp.Cursor.Line
	origOff := wp.Cursor.Offset
	if !cmdPyEndOfFunction(false, 1) {
		return false
	}
	wp.Mark.Line = wp.Cursor.Line
	wp.Mark.Offset = wp.Cursor.Offset
	if PackageHooks.WindowSetCursor != nil {
		PackageHooks.WindowSetCursor(wp, MakeLocation(origLine, origOff))
	}
	if !cmdPyTopOfFunction(false, 1) {
		wp.Mark.Line = 0
		return false
	}
	wp.DidMove = true
	if PackageHooks.Message != nil {
		PackageHooks.Message("[function marked]")
	}
	return true
}

func init() {
	for i := range modeTable {
		if modeTable[i].Mode == app.LModePython {
			modeTable[i].NewlineAndIndent = cmdPyNewlineAndIndent
			modeTable[i].IndentLine = cmdPyIndentLine
			modeTable[i].MakeComment = cmdPyMakeComment
			modeTable[i].TopOfFunction = cmdPyTopOfFunction
			modeTable[i].EndOfFunction = cmdPyEndOfFunction
			modeTable[i].MarkFunction = cmdPyMarkFunction
		}
	}
}

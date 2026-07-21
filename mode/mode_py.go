package mode

import (
	"bytes"
	"github.com/jdpalmer/jem/window"

	"github.com/jdpalmer/jem/buffer"
)

func lpKeyword(lp *buffer.Line, i uint, kw string) bool {
	if lp == nil {
		return false
	}
	klen := uint(len(kw))
	if i+klen > lp.Len() {
		return false
	}
	if !bytes.Equal(lp.Data[i:i+klen], []byte(kw)) {
		return false
	}
	after := i + klen
	if after >= lp.Len() {
		return true
	}
	nc := lp.Data[after]
	return nc == ' ' || nc == '\t' || nc == ':' || nc == '(' || nc == '\r'
}

func prevCodeLineNumberPy(bp *buffer.Buffer, lineNumber uint) uint {
	for lineNumber > 1 {
		lineNumber--
		p := bp.Line(lineNumber)
		if p == nil {
			continue
		}
		if !p.IsBlank() && p.FirstByte() != '#' {
			return lineNumber
		}
	}
	return 0
}

func lineIsDeindentKw(lp *buffer.Line) bool {
	if lp == nil {
		return false
	}
	i := lp.FirstNonblank()
	return lpKeyword(lp, i, "elif") || lpKeyword(lp, i, "else") || lpKeyword(lp, i, "except") || lpKeyword(lp, i, "finally")
}

func lineIsDedentStmt(lp *buffer.Line) bool {
	if lp == nil {
		return false
	}
	i := lp.FirstNonblank()
	return lpKeyword(lp, i, "return") || lpKeyword(lp, i, "break") || lpKeyword(lp, i, "continue") || lpKeyword(lp, i, "raise") || lpKeyword(lp, i, "pass")
}

func calcIndentPy(bp *buffer.Buffer, lineNumber uint) int {
	lp := bp.Line(lineNumber)
	pyIndentWidth := bp.PyIndent
	pyContinuedOffset := bp.PyContinuedOffset

	if lp != nil && lineIsDeindentKw(lp) {
		refLine := prevCodeLineNumberPy(bp, lineNumber)
		if refLine == 0 {
			return 0
		}
		ref := bp.Line(refLine)
		if ref == nil {
			return 0
		}
		ind := int(ref.IndentColumn()) - int(pyIndentWidth)
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
		l := bp.Line(baseLine)
		if l == nil {
			break
		}
		if l.LastByte() != '\\' {
			break
		}
		nextRef := prevCodeLineNumberPy(bp, baseLine)
		if nextRef == 0 {
			break
		}
		baseLine = nextRef
	}

	ref := bp.Line(baseLine)
	if ref == nil {
		return 0
	}
	refInd := int(ref.IndentColumn())
	last := bp.Line(refLine).LastByte()

	if last == '\\' {
		return refInd + int(pyContinuedOffset)
	}
	if last == ':' {
		return refInd + int(pyIndentWidth)
	}
	if last == '(' || last == '[' || last == '{' {
		return refInd + int(pyIndentWidth)
	}

	prev := bp.Line(refLine)
	if prev != nil && lineIsDedentStmt(prev) {
		curInd := int(prev.IndentColumn())
		ln := prevCodeLineNumberPy(bp, refLine)
		for ln != 0 {
			p := bp.Line(ln)
			if p == nil {
				ln = prevCodeLineNumberPy(bp, ln)
				continue
			}
			ind := int(p.IndentColumn())
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

func setLineIndentPy(wp *window.Window, col int) bool {
	if wp == nil || wp.Buffer == nil || col < 0 {
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
	begin := buffer.MakeLocation(ln, 0)
	end := buffer.MakeLocation(ln, oldFirst)
	PackageHooks.BeginCommand()
	err := PackageHooks.SetText(bp, begin, end, spaces, nil)
	ok := err == nil
	PackageHooks.EndCommand()
	if ok {
		wp.DidEdit = true
	}
	return ok
}

func findDefLineNumber(bp *buffer.Buffer, lineNumber uint) uint {
	if bp.LineCount == 0 {
		return 0
	}
	if lineNumber == bp.EOF() {
		lineNumber = bp.LineCount
	}
	lp := bp.Line(lineNumber)
	if lp == nil {
		return 0
	}
	i := lp.FirstNonblank()
	if lpKeyword(lp, i, "def") || lpKeyword(lp, i, "class") {
		return lineNumber
	}
	curInd := int(lp.IndentColumn())
	for lineNumber > 1 {
		lineNumber--
		lp = bp.Line(lineNumber)
		if lp == nil {
			continue
		}
		if !lp.IsBlank() {
			ind := int(lp.IndentColumn())
			if ind < curInd {
				j := lp.FirstNonblank()
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
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	for i := 0; i < n; i++ {
		if err := window.InsertNewline(wp); err != nil {
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
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
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
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	lp := bp.Line(wp.Cursor.Line)
	if lp != nil {
		for i := uint(0); i < lp.Len(); i++ {
			if lp.Byte(i) == '#' {
				wp.Cursor.Offset = i + 1
				if wp.Cursor.Offset < lp.Len() && lp.Byte(wp.Cursor.Offset) == ' ' {
					wp.Cursor.Offset++
				}
				wp.DidMove = true
				return true
			}
		}
	}
	if lp != nil {
		wp.Cursor.Offset = lp.Len()
	} else {
		wp.Cursor.Offset = 0
	}
	cmt := []byte("  # ")
	if err := window.InsertText(wp, cmt); err != nil {
		return false
	}
	wp.DidMove = true
	return true
}

func cmdPyTopOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
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
	wp.SetCursor(buffer.MakeLocation(defLine, 0))
	wp.DidMove = true
	return true
}

func cmdPyEndOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
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
	def := bp.Line(defLine)
	if def == nil {
		return false
	}
	defInd := int(def.IndentColumn())
	lineNumber := defLine + 1
	lastLine := defLine
	for lineNumber <= bp.LineCount {
		lp := bp.Line(lineNumber)
		if !lp.IsBlank() {
			if int(lp.IndentColumn()) <= defInd {
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
	lp := bp.Line(targetLine)
	off := uint(0)
	if lp != nil {
		off = lp.Len()
	}
	wp.SetCursor(buffer.MakeLocation(targetLine, off))
	wp.DidMove = true
	return true
}

func cmdPyMarkFunction(f bool, n int) bool {
	_ = f
	_ = n
	wp := window.Active.CurrentWindow
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
	wp.SetCursor(buffer.MakeLocation(origLine, origOff))
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
		if modeTable[i].Mode == buffer.LModePython {
			modeTable[i].NewlineAndIndent = cmdPyNewlineAndIndent
			modeTable[i].IndentLine = cmdPyIndentLine
			modeTable[i].MakeComment = cmdPyMakeComment
			modeTable[i].TopOfFunction = cmdPyTopOfFunction
			modeTable[i].EndOfFunction = cmdPyEndOfFunction
			modeTable[i].MarkFunction = cmdPyMarkFunction
		}
	}
}

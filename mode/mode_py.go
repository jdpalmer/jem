package mode

import (
	"bytes"
	"github.com/jdpalmer/jem/window"

	"github.com/jdpalmer/jem/buffer"
)

func lpKeyword(line *buffer.Line, i int, kw string) bool {
	if line == nil {
		return false
	}
	klen := len(kw)
	if i+klen > line.Len() {
		return false
	}
	if !bytes.Equal(line.Data[i:i+klen], []byte(kw)) {
		return false
	}
	after := i + klen
	if after >= line.Len() {
		return true
	}
	nc := line.Data[after]
	return nc == ' ' || nc == '\t' || nc == ':' || nc == '(' || nc == '\r'
}

func prevCodeLineNumberPy(buf *buffer.Buffer, lineNumber int) int {
	for lineNumber > 1 {
		lineNumber--
		p := buf.Line(lineNumber)
		if p == nil {
			continue
		}
		if !p.IsBlank() && p.FirstByte() != '#' {
			return lineNumber
		}
	}
	return 0
}

func lineIsDeindentKw(line *buffer.Line) bool {
	if line == nil {
		return false
	}
	i := line.FirstNonblank()
	return lpKeyword(line, i, "elif") || lpKeyword(line, i, "else") || lpKeyword(line, i, "except") || lpKeyword(line, i, "finally")
}

func lineIsDedentStmt(line *buffer.Line) bool {
	if line == nil {
		return false
	}
	i := line.FirstNonblank()
	return lpKeyword(line, i, "return") || lpKeyword(line, i, "break") || lpKeyword(line, i, "continue") || lpKeyword(line, i, "raise") || lpKeyword(line, i, "pass")
}

func calcIndentPy(buf *buffer.Buffer, lineNumber int) int {
	line := buf.Line(lineNumber)
	pyIndentWidth := buf.PyIndent
	pyContinuedOffset := buf.PyContinuedOffset

	if line != nil && lineIsDeindentKw(line) {
		refLine := prevCodeLineNumberPy(buf, lineNumber)
		if refLine == 0 {
			return 0
		}
		ref := buf.Line(refLine)
		if ref == nil {
			return 0
		}
		ind := ref.IndentColumn() - int(pyIndentWidth)
		if ind < 0 {
			return 0
		}
		return ind
	}

	refLine := prevCodeLineNumberPy(buf, lineNumber)
	if refLine == 0 {
		return 0
	}

	baseLine := refLine
	for baseLine > 1 {
		l := buf.Line(baseLine)
		if l == nil {
			break
		}
		if l.LastByte() != '\\' {
			break
		}
		nextRef := prevCodeLineNumberPy(buf, baseLine)
		if nextRef == 0 {
			break
		}
		baseLine = nextRef
	}

	ref := buf.Line(baseLine)
	if ref == nil {
		return 0
	}
	refInd := ref.IndentColumn()
	last := buf.Line(refLine).LastByte()

	if last == '\\' {
		return refInd + int(pyContinuedOffset)
	}
	if last == ':' {
		return refInd + int(pyIndentWidth)
	}
	if last == '(' || last == '[' || last == '{' {
		return refInd + int(pyIndentWidth)
	}

	prev := buf.Line(refLine)
	if prev != nil && lineIsDedentStmt(prev) {
		curInd := prev.IndentColumn()
		ln := prevCodeLineNumberPy(buf, refLine)
		for ln != 0 {
			p := buf.Line(ln)
			if p == nil {
				ln = prevCodeLineNumberPy(buf, ln)
				continue
			}
			ind := p.IndentColumn()
			if ind < curInd {
				if ind < 0 {
					return 0
				}
				return ind
			}
			ln = prevCodeLineNumberPy(buf, ln)
		}
		return 0
	}
	return refInd
}

func setLineIndentPy(win *window.Window, col int) bool {
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

func findDefLineNumber(buf *buffer.Buffer, lineNumber int) int {
	if len(buf.Lines) == 0 {
		return 0
	}
	if lineNumber == buf.EOF() {
		lineNumber = len(buf.Lines)
	}
	line := buf.Line(lineNumber)
	if line == nil {
		return 0
	}
	i := line.FirstNonblank()
	if lpKeyword(line, i, "def") || lpKeyword(line, i, "class") {
		return lineNumber
	}
	curInd := line.IndentColumn()
	for lineNumber > 1 {
		lineNumber--
		line = buf.Line(lineNumber)
		if line == nil {
			continue
		}
		if !line.IsBlank() {
			ind := line.IndentColumn()
			if ind < curInd {
				j := line.FirstNonblank()
				if lpKeyword(line, j, "def") || lpKeyword(line, j, "class") {
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
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	for i := 0; i < n; i++ {
		if err := window.InsertNewline(win); err != nil {
			return false
		}
		indent := calcIndentPy(buf, win.Cursor.Line)
		setLineIndentPy(win, indent)
	}
	return true
}

func cmdPyIndentLine(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	col := calcIndentPy(buf, win.Cursor.Line)
	setLineIndentPy(win, col)
	win.DidEdit = true
	return true
}

func cmdPyMakeComment(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	line := buf.Line(win.Cursor.Line)
	if line != nil {
		for i := 0; i < line.Len(); i++ {
			if line.Byte(i) == '#' {
				win.Cursor.Offset = i + 1
				if win.Cursor.Offset < line.Len() && line.Byte(win.Cursor.Offset) == ' ' {
					win.Cursor.Offset++
				}
				win.DidMove = true
				return true
			}
		}
	}
	if line != nil {
		win.Cursor.Offset = line.Len()
	} else {
		win.Cursor.Offset = 0
	}
	cmt := []byte("  # ")
	if err := window.InsertText(win, cmt); err != nil {
		return false
	}
	win.DidMove = true
	return true
}

func cmdPyTopOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	defLine := findDefLineNumber(buf, win.Cursor.Line)
	if defLine == 0 {
		if PackageHooks.Message != nil {
			PackageHooks.Message("[Not in a function]")
		}
		return false
	}
	win.SetCursor(buffer.MakeLocation(defLine, 0))
	win.DidMove = true
	return true
}

func cmdPyEndOfFunction(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	defLine := findDefLineNumber(buf, win.Cursor.Line)
	if defLine == 0 {
		if PackageHooks.Message != nil {
			PackageHooks.Message("[Not in a function]")
		}
		return false
	}
	def := buf.Line(defLine)
	if def == nil {
		return false
	}
	defInd := def.IndentColumn()
	lineNumber := defLine + 1
	lastLine := defLine
	for lineNumber <= len(buf.Lines) {
		line := buf.Line(lineNumber)
		if !line.IsBlank() {
			if line.IndentColumn() <= defInd {
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
	line := buf.Line(targetLine)
	off := 0
	if line != nil {
		off = line.Len()
	}
	win.SetCursor(buffer.MakeLocation(targetLine, off))
	win.DidMove = true
	return true
}

func cmdPyMarkFunction(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	if win == nil {
		return false
	}
	origLine := win.Cursor.Line
	origOff := win.Cursor.Offset
	if !cmdPyEndOfFunction(false, 1) {
		return false
	}
	win.Mark.Line = win.Cursor.Line
	win.Mark.Offset = win.Cursor.Offset
	win.SetCursor(buffer.MakeLocation(origLine, origOff))
	if !cmdPyTopOfFunction(false, 1) {
		win.Mark.Line = 0
		return false
	}
	win.DidMove = true
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

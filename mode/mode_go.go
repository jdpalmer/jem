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

func calcIndentGo(bp *buffer.Buffer, lineNumber uint) int {
	if bp == nil {
		return 0
	}
	saved := bp.CIndent
	bp.CIndent = goIndentCols
	ind := calcIndent(bp, lineNumber)
	bp.CIndent = saved
	return ind
}

func setLineIndentGo(wp *window.Window, col int) bool {
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
	prefix := indentBytesForCol(col)
	begin := buffer.MakeLocation(ln, 0)
	end := buffer.MakeLocation(ln, oldFirst)
	PackageHooks.BeginCommand()
	err := PackageHooks.SetText(bp, begin, end, prefix, nil)
	ok := err == nil
	PackageHooks.EndCommand()
	if ok {
		wp.DidEdit = true
		// Park cursor after the new indent when it was in the old indent region.
		if wp.Cursor.Offset <= oldFirst {
			wp.Cursor.Offset = uint(len(prefix))
		} else {
			delta := int(len(prefix)) - int(oldFirst)
			off := int(wp.Cursor.Offset) + delta
			if off < 0 {
				off = 0
			}
			wp.Cursor.Offset = uint(off)
		}
	}
	return ok
}

func cmdGoNewlineAndIndent(f bool, n int) bool {
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
		indent := calcIndentGo(bp, wp.Cursor.Line)
		setLineIndentGo(wp, indent)
	}
	return true
}

func cmdGoIndentLine(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	col := calcIndentGo(bp, wp.Cursor.Line)
	setLineIndentGo(wp, col)
	wp.DidEdit = true
	return true
}

func cmdGoCloseBrace(f bool, n int) bool {
	_ = f
	if n <= 0 {
		n = 1
	}
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	// Insert closers first so calcIndent sees '}' and aligns to the open brace.
	if !setLineIndentGo(wp, 0) {
		return false
	}
	wp.Cursor.Offset = 0
	for i := 0; i < n; i++ {
		if err := window.InsertCodepoint(wp, '}'); err != nil {
			return false
		}
	}
	col := calcIndentGo(bp, wp.Cursor.Line)
	if !setLineIndentGo(wp, col) {
		return false
	}
	lp := bp.Line(wp.Cursor.Line)
	if lp != nil {
		wp.Cursor.Offset = lp.Len()
	}
	wp.DidEdit = true
	return true
}

// cmdGoMakeComment inserts or jumps into a // line comment (gofmt style).
func cmdGoMakeComment(f bool, n int) bool {
	_ = f
	_ = n
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	lp := bp.Line(wp.Cursor.Line)
	if lp != nil {
		prefix := []byte("//")
		if lineHasCommentPrefix(lp, prefix) {
			pos := lp.FirstNonblank()
			wp.Cursor.Offset = pos + uint(len(prefix))
			if wp.Cursor.Offset < lp.Len() && lp.Byte(wp.Cursor.Offset) == ' ' {
				wp.Cursor.Offset++
			}
			wp.DidMove = true
			return true
		}
	}
	if wp.Mark.Line != 0 && wp.Mark.Line != wp.Cursor.Line {
		info := CurrentModeInfo()
		startLine := wp.Mark.Line
		endLine := wp.Cursor.Line
		if startLine > endLine {
			startLine, endLine = endLine, startLine
		}
		return modeToggleCommentRegion(wp, bp, info, []byte("//"), startLine, endLine)
	}
	if lp != nil {
		wp.Cursor.Offset = lp.Len()
	} else {
		wp.Cursor.Offset = 0
	}
	if err := window.InsertText(wp, []byte("  // ")); err != nil {
		return false
	}
	wp.DidMove = true
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

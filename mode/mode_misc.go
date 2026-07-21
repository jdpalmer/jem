package mode

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/window"
)

const (
	simpleIndent           = 2
	makeContinuationIndent = 2
)

type IndentSpec struct {
	Spaces     uint
	LeadingTab bool
}

func u8lower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b - 'A' + 'a'
	}
	return b
}

func u8isalnum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func prevNonblankLineNumberBuf(bp *buffer.Buffer, lineNumber uint) uint {
	for lineNumber > 1 {
		lineNumber--
		p := bp.Line(lineNumber)
		if p != nil && !p.IsBlank() {
			return lineNumber
		}
	}
	return 0
}

func wordMatchCI(lp *buffer.Line, start uint, word string) bool {
	if lp == nil {
		return false
	}
	i := start
	j := 0
	for j < len(word) {
		if i >= lp.Len() {
			return false
		}
		if u8lower(lp.Byte(i)) != u8lower(word[j]) {
			return false
		}
		i++
		j++
	}
	if i >= lp.Len() {
		return true
	}
	c := lp.Byte(i)
	return !(u8isalnum(c) || c == '_')
}

func lineStartsWordCI(lp *buffer.Line, word string) bool {
	return wordMatchCI(lp, lp.FirstNonblank(), word)
}

func lineEndsWithWordCI(lp *buffer.Line, word string) bool {
	if lp == nil {
		return false
	}
	end := int(lp.Len())
	wlen := len(word)
	for end > 0 {
		c := lp.Byte(uint(end - 1))
		if c != ' ' && c != '\t' {
			break
		}
		end--
	}
	if end < wlen {
		return false
	}
	for i := 0; i < wlen; i++ {
		if u8lower(lp.Byte(uint(end-wlen+i))) != u8lower(word[i]) {
			return false
		}
	}
	if end == wlen {
		return true
	}
	c := lp.Byte(uint(end - wlen - 1))
	return !(u8isalnum(c) || c == '_')
}

func isMakeTargetLine(lp *buffer.Line) bool {
	if lp == nil {
		return false
	}
	i := lp.FirstNonblank()
	if i >= lp.Len() || lp.Byte(i) == '\t' {
		return false
	}
	seenColon := false
	for k := i; k < lp.Len(); k++ {
		c := lp.Byte(k)
		if c == '#' {
			break
		}
		if c == ':' && k+1 < lp.Len() && lp.Byte(k+1) == '=' {
			return false
		}
		if c == ':' && !seenColon {
			seenColon = true
		}
		if c == '=' && !seenColon {
			return false
		}
	}
	return seenColon
}

func calcMakeIndent(bp *buffer.Buffer, lineNumber uint) IndentSpec {
	lp := bp.Line(lineNumber)
	refLine := prevNonblankLineNumberBuf(bp, lineNumber)
	var ref *buffer.Line
	if refLine != 0 {
		ref = bp.Line(refLine)
	}
	if lp != nil && lp.FirstByte() == '\t' {
		return IndentSpec{0, true}
	}
	if ref == nil {
		return IndentSpec{0, false}
	}
	if isMakeTargetLine(ref) || ref.FirstByte() == '\t' {
		return IndentSpec{0, true}
	}
	if ref.LastByte() == '\\' {
		return IndentSpec{uint(ref.IndentColumn() + makeContinuationIndent), false}
	}
	return IndentSpec{0, false}
}

func luaIsCloser(lp *buffer.Line) bool {
	return lineStartsWordCI(lp, "end") || lineStartsWordCI(lp, "until") || lineStartsWordCI(lp, "elseif") || lineStartsWordCI(lp, "else")
}
func luaIsOpener(lp *buffer.Line) bool {
	return lineEndsWithWordCI(lp, "then") || lineEndsWithWordCI(lp, "do") || lineStartsWordCI(lp, "repeat") || lineStartsWordCI(lp, "function")
}
func pascalIsCloser(lp *buffer.Line) bool {
	return lineStartsWordCI(lp, "end") || lineStartsWordCI(lp, "until") || lineStartsWordCI(lp, "else")
}
func pascalIsOpener(lp *buffer.Line) bool {
	return lineStartsWordCI(lp, "begin") || lineStartsWordCI(lp, "repeat") || lineEndsWithWordCI(lp, "then") || lineEndsWithWordCI(lp, "do") || lineStartsWordCI(lp, "case") || lineStartsWordCI(lp, "record")
}
func verilogIsCloser(lp *buffer.Line) bool {
	return lineStartsWordCI(lp, "endcase") || lineStartsWordCI(lp, "endmodule") || lineStartsWordCI(lp, "endfunction") || lineStartsWordCI(lp, "endtask") || lineStartsWordCI(lp, "endclass") || lineStartsWordCI(lp, "join") || lineStartsWordCI(lp, "end")
}
func verilogIsOpener(lp *buffer.Line) bool {
	return lineStartsWordCI(lp, "module") || lineStartsWordCI(lp, "class") || lineStartsWordCI(lp, "function") || lineStartsWordCI(lp, "task") || lineStartsWordCI(lp, "case") || lineStartsWordCI(lp, "casex") || lineStartsWordCI(lp, "casez") || lineStartsWordCI(lp, "fork") || lineEndsWithWordCI(lp, "begin")
}

func htmlIsCloser(lp *buffer.Line) bool {
	i := lp.FirstNonblank()
	if i >= lp.Len() || lp.Byte(i) != '<' {
		return false
	}
	i++
	return i < lp.Len() && lp.Byte(i) == '/'
}

func htmlIsOpener(lp *buffer.Line) bool {
	i := lp.FirstNonblank()
	end := lp.Len()
	for end > i {
		c := lp.Byte(uint(end - 1))
		if c != ' ' && c != '\t' {
			break
		}
		end--
	}
	if i >= end || lp.Byte(i) != '<' || i+1 >= end {
		return false
	}
	c := lp.Byte(uint(i + 1))
	if c == '/' || c == '!' || c == '?' {
		return false
	}
	if end-i >= 2 && lp.Byte(uint(end-2)) == '/' && lp.Byte(uint(end-1)) == '>' {
		return false
	}
	return lp.Byte(uint(end-1)) == '>'
}

func calcBlockIndent(bp *buffer.Buffer, lineNumber uint, isCloser func(*buffer.Line) bool, isOpener func(*buffer.Line) bool) IndentSpec {
	lp := bp.Line(lineNumber)
	refLine := prevNonblankLineNumberBuf(bp, lineNumber)
	var ref *buffer.Line
	if refLine != 0 {
		ref = bp.Line(refLine)
	}
	base := 0
	if ref != nil {
		base = int(ref.IndentColumn())
	}
	if lp != nil && isCloser != nil && isCloser(lp) {
		ind := base - simpleIndent
		if ind < 0 {
			ind = 0
		}
		return IndentSpec{uint(ind), false}
	}
	if ref != nil && isOpener != nil && isOpener(ref) {
		return IndentSpec{uint(base + simpleIndent), false}
	}
	return IndentSpec{uint(base), false}
}

func calcLispIndent(bp *buffer.Buffer, lineNumber uint) IndentSpec {
	lp := bp.Line(lineNumber)
	depth := 0
	closeAlign := lp != nil && lp.FirstByte() == ')'
	for lineNumber > 1 {
		lineNumber--
		p := bp.Line(lineNumber)
		if p == nil {
			continue
		}
		for i := int(p.Len()); i > 0; i-- {
			c := p.Byte(uint(i - 1))
			if c == ')' {
				depth++
			} else if c == '(' {
				if depth == 0 {
					openCol := lineColOfOffset(p, uint(i-1))
					if closeAlign {
						return IndentSpec{uint(openCol), false}
					}
					for j := i; j < int(p.Len()); j++ {
						nc := p.Byte(uint(j))
						if nc == ' ' || nc == '\t' {
							continue
						}
						if nc == ')' {
							break
						}
						return IndentSpec{uint(lineColOfOffset(p, uint(j))), false}
					}
					return IndentSpec{uint(openCol + 1), false}
				}
				depth--
			}
		}
	}
	return IndentSpec{0, false}
}

func calcMiscIndent(bp *buffer.Buffer, lineNumber uint) IndentSpec {
	kind := LangModeInfo(bp.LangMode).MiscIndentKind
	switch kind {
	case ModeMiscIndentNone:
		return IndentSpec{0, false}
	case ModeMiscIndentMake:
		return calcMakeIndent(bp, lineNumber)
	case ModeMiscIndentLua:
		return calcBlockIndent(bp, lineNumber, luaIsCloser, luaIsOpener)
	case ModeMiscIndentPascal:
		return calcBlockIndent(bp, lineNumber, pascalIsCloser, pascalIsOpener)
	case ModeMiscIndentVerilog:
		return calcBlockIndent(bp, lineNumber, verilogIsCloser, verilogIsOpener)
	case ModeMiscIndentR:
		return calcBlockIndent(bp, lineNumber, nil, nil)
	case ModeMiscIndentHTML:
		return calcBlockIndent(bp, lineNumber, htmlIsCloser, htmlIsOpener)
	case ModeMiscIndentLisp:
		return calcLispIndent(bp, lineNumber)
	default:
		return IndentSpec{0, false}
	}
}

func setLineIndentMisc(wp *window.Window, spec IndentSpec) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	bp := wp.Buffer
	ln := wp.Cursor.Line
	lp := bp.Line(ln)
	if lp == nil {
		return false
	}
	first := lp.FirstNonblank()
	var prefix []byte
	if spec.LeadingTab {
		prefix = []byte{'\t'}
	} else {
		prefix = make([]byte, spec.Spaces)
		for i := range prefix {
			prefix[i] = ' '
		}
	}
	begin := buffer.MakeLocation(ln, 0)
	end := buffer.MakeLocation(ln, first)
	PackageHooks.BeginCommand()
	err := PackageHooks.SetText(bp, begin, end, prefix, nil)
	ok := err == nil
	PackageHooks.EndCommand()
	if ok {
		wp.DidEdit = true
	}
	return ok
}

func cmdMiscNewlineAndIndent(f bool, n int) bool {
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
		if !window.InsertNewline(wp) {
			return false
		}
		spec := calcMiscIndent(bp, wp.Cursor.Line)
		if !setLineIndentMisc(wp, spec) {
			return false
		}
	}
	return true
}

func cmdMiscIndentLine(f bool, n int) bool {
	_ = f
	_ = n
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	spec := calcMiscIndent(bp, wp.Cursor.Line)
	_ = setLineIndentMisc(wp, spec)
	wp.DidEdit = true
	return true
}

func init() {
	for i := range modeTable {
		if modeTable[i].MiscIndentKind != ModeMiscIndentNone {
			modeTable[i].NewlineAndIndent = cmdMiscNewlineAndIndent
			modeTable[i].IndentLine = cmdMiscIndentLine
		}
	}
}

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

func u8isalnum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func prevNonblankLineNumberBuf(buf *buffer.Buffer, lineNumber uint) uint {
	for lineNumber > 1 {
		lineNumber--
		p := buf.Line(lineNumber)
		if p != nil && !p.IsBlank() {
			return lineNumber
		}
	}
	return 0
}

func wordMatchCI(line *buffer.Line, start uint, word string) bool {
	if line == nil {
		return false
	}
	i := start
	j := 0
	for j < len(word) {
		if i >= line.Len() {
			return false
		}
		if buffer.ToLowerASCII(line.Byte(i)) != buffer.ToLowerASCII(word[j]) {
			return false
		}
		i++
		j++
	}
	if i >= line.Len() {
		return true
	}
	c := line.Byte(i)
	return !(u8isalnum(c) || c == '_')
}

func lineStartsWordCI(line *buffer.Line, word string) bool {
	return wordMatchCI(line, line.FirstNonblank(), word)
}

func lineEndsWithWordCI(line *buffer.Line, word string) bool {
	if line == nil {
		return false
	}
	end := int(line.Len())
	wlen := len(word)
	for end > 0 {
		c := line.Byte(uint(end - 1))
		if c != ' ' && c != '\t' {
			break
		}
		end--
	}
	if end < wlen {
		return false
	}
	for i := 0; i < wlen; i++ {
		if buffer.ToLowerASCII(line.Byte(uint(end-wlen+i))) != buffer.ToLowerASCII(word[i]) {
			return false
		}
	}
	if end == wlen {
		return true
	}
	c := line.Byte(uint(end - wlen - 1))
	return !(u8isalnum(c) || c == '_')
}

func isMakeTargetLine(line *buffer.Line) bool {
	if line == nil {
		return false
	}
	i := line.FirstNonblank()
	if i >= line.Len() || line.Byte(i) == '\t' {
		return false
	}
	seenColon := false
	for k := i; k < line.Len(); k++ {
		c := line.Byte(k)
		if c == '#' {
			break
		}
		if c == ':' && k+1 < line.Len() && line.Byte(k+1) == '=' {
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

func calcMakeIndent(buf *buffer.Buffer, lineNumber uint) IndentSpec {
	line := buf.Line(lineNumber)
	refLine := prevNonblankLineNumberBuf(buf, lineNumber)
	var ref *buffer.Line
	if refLine != 0 {
		ref = buf.Line(refLine)
	}
	if line != nil && line.FirstByte() == '\t' {
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

func luaIsCloser(line *buffer.Line) bool {
	return lineStartsWordCI(line, "end") || lineStartsWordCI(line, "until") || lineStartsWordCI(line, "elseif") || lineStartsWordCI(line, "else")
}
func luaIsOpener(line *buffer.Line) bool {
	return lineEndsWithWordCI(line, "then") || lineEndsWithWordCI(line, "do") || lineStartsWordCI(line, "repeat") || lineStartsWordCI(line, "function")
}
func pascalIsCloser(line *buffer.Line) bool {
	return lineStartsWordCI(line, "end") || lineStartsWordCI(line, "until") || lineStartsWordCI(line, "else")
}
func pascalIsOpener(line *buffer.Line) bool {
	return lineStartsWordCI(line, "begin") || lineStartsWordCI(line, "repeat") || lineEndsWithWordCI(line, "then") || lineEndsWithWordCI(line, "do") || lineStartsWordCI(line, "case") || lineStartsWordCI(line, "record")
}
func verilogIsCloser(line *buffer.Line) bool {
	return lineStartsWordCI(line, "endcase") || lineStartsWordCI(line, "endmodule") || lineStartsWordCI(line, "endfunction") || lineStartsWordCI(line, "endtask") || lineStartsWordCI(line, "endclass") || lineStartsWordCI(line, "join") || lineStartsWordCI(line, "end")
}
func verilogIsOpener(line *buffer.Line) bool {
	return lineStartsWordCI(line, "module") || lineStartsWordCI(line, "class") || lineStartsWordCI(line, "function") || lineStartsWordCI(line, "task") || lineStartsWordCI(line, "case") || lineStartsWordCI(line, "casex") || lineStartsWordCI(line, "casez") || lineStartsWordCI(line, "fork") || lineEndsWithWordCI(line, "begin")
}

func htmlIsCloser(line *buffer.Line) bool {
	i := line.FirstNonblank()
	if i >= line.Len() || line.Byte(i) != '<' {
		return false
	}
	i++
	return i < line.Len() && line.Byte(i) == '/'
}

func htmlIsOpener(line *buffer.Line) bool {
	i := line.FirstNonblank()
	end := line.Len()
	for end > i {
		c := line.Byte(uint(end - 1))
		if c != ' ' && c != '\t' {
			break
		}
		end--
	}
	if i >= end || line.Byte(i) != '<' || i+1 >= end {
		return false
	}
	c := line.Byte(uint(i + 1))
	if c == '/' || c == '!' || c == '?' {
		return false
	}
	if end-i >= 2 && line.Byte(uint(end-2)) == '/' && line.Byte(uint(end-1)) == '>' {
		return false
	}
	return line.Byte(uint(end-1)) == '>'
}

func calcBlockIndent(buf *buffer.Buffer, lineNumber uint, isCloser func(*buffer.Line) bool, isOpener func(*buffer.Line) bool) IndentSpec {
	line := buf.Line(lineNumber)
	refLine := prevNonblankLineNumberBuf(buf, lineNumber)
	var ref *buffer.Line
	if refLine != 0 {
		ref = buf.Line(refLine)
	}
	base := 0
	if ref != nil {
		base = int(ref.IndentColumn())
	}
	if line != nil && isCloser != nil && isCloser(line) {
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

func calcLispIndent(buf *buffer.Buffer, lineNumber uint) IndentSpec {
	line := buf.Line(lineNumber)
	depth := 0
	closeAlign := line != nil && line.FirstByte() == ')'
	for lineNumber > 1 {
		lineNumber--
		p := buf.Line(lineNumber)
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

func calcMiscIndent(buf *buffer.Buffer, lineNumber uint) IndentSpec {
	kind := LangModeInfo(buf.LangMode).MiscIndentKind
	switch kind {
	case ModeMiscIndentNone:
		return IndentSpec{0, false}
	case ModeMiscIndentMake:
		return calcMakeIndent(buf, lineNumber)
	case ModeMiscIndentLua:
		return calcBlockIndent(buf, lineNumber, luaIsCloser, luaIsOpener)
	case ModeMiscIndentPascal:
		return calcBlockIndent(buf, lineNumber, pascalIsCloser, pascalIsOpener)
	case ModeMiscIndentVerilog:
		return calcBlockIndent(buf, lineNumber, verilogIsCloser, verilogIsOpener)
	case ModeMiscIndentR:
		return calcBlockIndent(buf, lineNumber, nil, nil)
	case ModeMiscIndentHTML:
		return calcBlockIndent(buf, lineNumber, htmlIsCloser, htmlIsOpener)
	case ModeMiscIndentLisp:
		return calcLispIndent(buf, lineNumber)
	default:
		return IndentSpec{0, false}
	}
}

func setLineIndentMisc(win *window.Window, spec IndentSpec) bool {
	if win == nil || win.Buffer == nil {
		return false
	}
	buf := win.Buffer
	ln := win.Cursor.Line
	line := buf.Line(ln)
	if line == nil {
		return false
	}
	first := line.FirstNonblank()
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
	err := PackageHooks.SetText(buf, begin, end, prefix, nil)
	ok := err == nil
	PackageHooks.EndCommand()
	if ok {
		win.DidEdit = true
	}
	return ok
}

func cmdMiscNewlineAndIndent(f bool, n int) bool {
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
		spec := calcMiscIndent(buf, win.Cursor.Line)
		if !setLineIndentMisc(win, spec) {
			return false
		}
	}
	return true
}

func cmdMiscIndentLine(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	spec := calcMiscIndent(buf, win.Cursor.Line)
	_ = setLineIndentMisc(win, spec)
	win.DidEdit = true
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

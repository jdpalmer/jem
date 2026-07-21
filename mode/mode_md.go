package mode

import (
	"strconv"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/window"
)

func prevNonblankLineNumber(bp *buffer.Buffer, lineNumber uint) uint {
	for lineNumber > 1 {
		lineNumber--
		p := bp.Line(lineNumber)
		if p != nil && !p.IsBlank() {
			return lineNumber
		}
	}
	return 0
}

func setLinePrefix(wp *window.Window, prefix []byte) bool {
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

func mdBuildPrefix(lp *buffer.Line) []byte {
	if lp == nil || lp.Len() == 0 {
		return nil
	}
	p := lp.Data
	out := make([]byte, 0, 32)
	i := 0
	for i < len(p) && (p[i] == ' ' || p[i] == '\t') {
		out = append(out, p[i])
		i++
	}
	for i < len(p) && p[i] == '>' {
		out = append(out, '>')
		out = append(out, ' ')
		i++
		if i < len(p) && p[i] == ' ' {
			i++
		}
		for i < len(p) && p[i] == ' ' {
			i++
		}
	}
	if i < len(p) && (p[i] == '-' || p[i] == '*' || p[i] == '+') {
		if i+1 < len(p) && p[i+1] == ' ' {
			out = append(out, p[i], ' ')
			return out
		}
	}
	save := i
	num := 0
	for i < len(p) && p[i] >= '0' && p[i] <= '9' {
		num = num*10 + int(p[i]-'0')
		i++
	}
	if i > save {
		if i < len(p) {
			del := p[i]
			if (del == '.' || del == ')') && i+1 < len(p) && p[i+1] == ' ' {
				next := num + 1
				out = append(out, strconv.AppendInt(nil, int64(next), 10)...)
				out = append(out, del, ' ')
				return out
			}
		}
		i = save
	}
	_ = i
	return out
}

func cmdMdNewlineAndIndent(f bool, n int) bool {
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
		lp := bp.Line(wp.Cursor.Line)
		prefix := mdBuildPrefix(lp)
		if !window.InsertNewline(wp) {
			return false
		}
		if len(prefix) > 0 && !window.InsertText(wp, prefix) {
			return false
		}
	}
	return true
}

func cmdMdIndentLine(f bool, n int) bool {
	_ = f
	_ = n
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	refLine := prevNonblankLineNumber(bp, wp.Cursor.Line)
	if refLine == 0 {
		return true
	}
	prefix := mdBuildPrefix(bp.Line(refLine))
	setLinePrefix(wp, prefix)
	wp.DidEdit = true
	return true
}

func init() {
	for i := range modeTable {
		if modeTable[i].Mode == buffer.LModeMarkdown {
			modeTable[i].NewlineAndIndent = cmdMdNewlineAndIndent
			modeTable[i].IndentLine = cmdMdIndentLine
		}
	}
}

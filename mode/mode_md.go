package mode

import (
	"strconv"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/window"
)

func prevNonblankLineNumber(buf *buffer.Buffer, lineNumber int) int {
	for lineNumber > 1 {
		lineNumber--
		p := buf.Line(lineNumber)
		if p != nil && !p.IsBlank() {
			return lineNumber
		}
	}
	return 0
}

func setLinePrefix(win *window.Window, prefix []byte) bool {
	buf := win.Buffer
	ln := win.Cursor.Line
	line := buf.Line(ln)
	if line == nil {
		return false
	}
	first := line.FirstNonblank()
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

func mdBuildPrefix(line *buffer.Line) []byte {
	p := line.Data
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
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	for i := 0; i < n; i++ {
		line := buf.Line(win.Cursor.Line)
		prefix := mdBuildPrefix(line)
		if err := window.InsertNewline(win); err != nil {
			return false
		}
		if len(prefix) > 0 {
			if err := window.InsertText(win, prefix); err != nil {
				return false
			}
		}
	}
	return true
}

func cmdMdIndentLine(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return false
	}
	refLine := prevNonblankLineNumber(buf, win.Cursor.Line)
	if refLine == 0 {
		return true
	}
	prefix := mdBuildPrefix(buf.Line(refLine))
	setLinePrefix(win, prefix)
	win.DidEdit = true
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

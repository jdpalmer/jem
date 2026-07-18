package modeactions

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/edit"
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

func setLinePrefix(wp *app.Window, prefix []byte) bool {
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
	edit.BeginCommand()
	err := edit.SetText(bp, begin, end, prefix, nil)
	ok := err == nil
	edit.EndCommand()
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
				buf := []byte("")
				temp := make([]byte, 0, 12)
				if next == 0 {
					temp = append(temp, '0')
				} else {
					nn := next
					for nn > 0 {
						temp = append(temp, byte('0'+(nn%10)))
						nn /= 10
					}
					for k := len(temp) - 1; k >= 0; k-- {
						buf = append(buf, temp[k])
					}
				}
				out = append(out, buf...)
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
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	for i := 0; i < n; i++ {
		lp := bp.Line(wp.Cursor.Line)
		prefix := mdBuildPrefix(lp)
		if !edit.InsertNewline(wp) {
			return false
		}
		if len(prefix) > 0 && !edit.InsertText(wp, prefix) {
			return false
		}
	}
	return true
}

func cmdMdIndentLine(f bool, n int) bool {
	_ = f
	_ = n
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
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

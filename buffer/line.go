package buffer

import (
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

type Line struct {
	Data           []byte
	SyntaxEndState SynState
	SyntaxSummary  SyntaxLineSummary
	SyntaxValid    bool
	RuneCache      []rune
	WidthCache     []int8
	SyntaxStyles   []TextStyle
	CacheValid     bool
	Metadata       any
	LangMode       LangMode
	Buffer         *Buffer
}

func (line *Line) Byte(n int) byte {
	if line == nil || n < 0 || n >= len(line.Data) {
		return 0
	}
	return line.Data[n]
}

func (line *Line) Len() int {
	if line == nil {
		return 0
	}
	return len(line.Data)
}

// EnsureCache decodes UTF-8 runes for syntax and display helpers.
func (line *Line) EnsureCache() {
	if line == nil || line.CacheValid {
		return
	}
	bs := line.Data
	capNeeded := len(bs)/2 + 1
	runes := make([]rune, 0, capNeeded)
	widths := make([]int8, 0, capNeeded)
	i := 0
	for i < len(bs) {
		r, size := utf8.DecodeRune(bs[i:])
		if r == utf8.RuneError && size == 1 {
			r = rune(bs[i])
			size = 1
		}
		w := runewidth.RuneWidth(r)
		if w <= 0 {
			w = 1
		}
		runes = append(runes, r)
		widths = append(widths, int8(w))
		i += size
	}
	line.RuneCache = runes
	line.WidthCache = widths
	line.CacheValid = true
}

func (line *Line) FirstNonblank() int {
	if line == nil {
		return 0
	}
	for i := 0; i < len(line.Data); i++ {
		c := line.Data[i]
		if c != ' ' && c != '\t' {
			return i
		}
	}
	return len(line.Data)
}

func (line *Line) IndentColumn() int {
	if line == nil {
		return 0
	}
	col := 0
	for i := 0; i < len(line.Data); i++ {
		c := line.Data[i]
		if c == ' ' {
			col++
		} else if c == '\t' {
			col += 8 - (col % 8)
		} else {
			break
		}
	}
	return col
}

func (line *Line) FirstByte() byte {
	if line == nil || len(line.Data) == 0 {
		return 0
	}
	return line.Data[0]
}

func (line *Line) LastByte() byte {
	if line == nil || len(line.Data) == 0 {
		return 0
	}
	return line.Data[len(line.Data)-1]
}

func (line *Line) IsBlank() bool {
	if line == nil || len(line.Data) == 0 {
		return true
	}
	for i := 0; i < len(line.Data); i++ {
		c := line.Data[i]
		if c != ' ' && c != '\t' {
			return false
		}
	}
	return true
}

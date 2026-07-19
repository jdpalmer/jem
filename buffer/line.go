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

func (lp *Line) Byte(n uint) byte {
	if lp == nil || n >= uint(len(lp.Data)) {
		return 0
	}
	return lp.Data[n]
}

func (lp *Line) Len() uint {
	if lp == nil {
		return 0
	}
	return uint(len(lp.Data))
}

// EnsureCache decodes UTF-8 runes for syntax and display helpers.
func (lp *Line) EnsureCache() {
	if lp == nil || lp.CacheValid {
		return
	}
	bs := lp.Data
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
	lp.RuneCache = runes
	lp.WidthCache = widths
	lp.CacheValid = true
}

func (lp *Line) FirstNonblank() uint {
	if lp == nil {
		return 0
	}
	for i := uint(0); i < uint(len(lp.Data)); i++ {
		c := lp.Data[i]
		if c != ' ' && c != '\t' {
			return i
		}
	}
	return uint(len(lp.Data))
}

func (lp *Line) IndentColumn() uint {
	if lp == nil {
		return 0
	}
	col := uint(0)
	for i := uint(0); i < uint(len(lp.Data)); i++ {
		c := lp.Data[i]
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

func (lp *Line) FirstByte() byte {
	if lp == nil || len(lp.Data) == 0 {
		return 0
	}
	return lp.Data[0]
}

func (lp *Line) LastByte() byte {
	if lp == nil || len(lp.Data) == 0 {
		return 0
	}
	return lp.Data[len(lp.Data)-1]
}

func (lp *Line) IsBlank() bool {
	if lp == nil || len(lp.Data) == 0 {
		return true
	}
	for i := uint(0); i < uint(len(lp.Data)); i++ {
		c := lp.Data[i]
		if c != ' ' && c != '\t' {
			return false
		}
	}
	return true
}

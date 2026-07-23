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

// Byte returns the byte at the given index, or 0 if the index is out of range.
func (line *Line) Byte(n int) byte {
	if n < 0 || n >= len(line.Data) {
		return 0
	}
	return line.Data[n]
}

// Len returns the number of bytes in the line, or 0 if the line is nil.
func (line *Line) Len() int {
	return len(line.Data)
}

// EnsureCache decodes UTF-8 runes for syntax and display helpers.
func (line *Line) EnsureCache() {
	if line.CacheValid {
		return
	}
	bs := line.Data
	runes := make([]rune, 0, len(bs))
	widths := make([]int8, 0, len(bs))
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

// FirstNonblank returns the byte index of the first non-whitespace character.
func (line *Line) FirstNonblank() int {
	for i := 0; i < len(line.Data); i++ {
		c := line.Data[i]
		if c != ' ' && c != '\t' {
			return i
		}
	}
	return len(line.Data)
}

// IndentColumn returns the column position of the first non-whitespace character.
func (line *Line) IndentColumn() int {
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

// FirstByte returns the first byte of the line, or 0 if the line is nil or empty.
func (line *Line) FirstByte() byte {
	if len(line.Data) == 0 {
		return 0
	}
	return line.Data[0]
}

// LastByte returns the last byte of the line, or 0 if the line is nil or empty.
func (line *Line) LastByte() byte {
	if len(line.Data) == 0 {
		return 0
	}
	return line.Data[len(line.Data)-1]
}

// IsBlank returns true if the line contains only whitespace or is empty.
func (line *Line) IsBlank() bool {
	return line.FirstNonblank() == len(line.Data)
}

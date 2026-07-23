package display

import (
	"slices"
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
	"github.com/mattn/go-runewidth"
)

func screenRowsScrollUp(rows *[]ScreenRow, start, length, n int) {
	reverseScreenRows(rows, start, start+n-1)
	reverseScreenRows(rows, start+n, start+length-1)
	reverseScreenRows(rows, start, start+length-1)
}

func screenRowsScrollDown(rows *[]ScreenRow, start, length, n int) {
	screenRowsScrollUp(rows, start, length, length-n)
}

func reverseScreenRows(rows *[]ScreenRow, lo, hi int) {
	if lo < hi {
		slices.Reverse((*rows)[lo : hi+1])
	}
}

// lineMeasureAdvance returns the screen column after rendering one character.
func lineMeasureAdvance(col int, c rune) int {
	if c == '\t' {
		col |= 0x07
		return col + 1
	}
	if c < 0x20 || c == 0x7F {
		return col + 2
	}
	if c < 0x80 {
		return col + 1
	}
	w := runewidth.RuneWidth(c)
	if w > 0 {
		return col + w
	}
	return col + 1
}

// lineColAtOffset returns the screen column corresponding to byte offset in line.
func lineColAtOffset(line *buffer.Line, offset int) int {
	col := 0
	i := 0
	for i < offset && i < line.Len() {
		b := line.Data[i]
		if b < 0x80 {
			col = lineMeasureAdvance(col, rune(b))
			i++
			continue
		}
		r, size := utf8.DecodeRune(line.Data[i:])
		if r == utf8.RuneError && size == 1 {
			col = lineMeasureAdvance(col, rune(b))
			i++
			continue
		}
		col = lineMeasureAdvance(col, r)
		i += size
	}
	return col
}

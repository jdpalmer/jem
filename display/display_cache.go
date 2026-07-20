package display

import (
	"fmt"
	"github.com/jdpalmer/jem/window"
	"os"
	"time"
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
	for lo < hi {
		(*rows)[lo], (*rows)[hi] = (*rows)[hi], (*rows)[lo]
		lo++
		hi--
	}
}

// ---- Line cache -----------------------------------------------------------------

func ensureLineCache(lp *buffer.Line) {
	if lp == nil || lp.CacheValid {
		return
	}
	bs := lp.Data
	capNeeded := len(bs)/2 + 1
	var runes []rune
	if v := runeSlicePool.Get(); v != nil {
		r := v.([]rune)
		if cap(r) >= capNeeded {
			runes = r[:0]
		} else {
			runes = make([]rune, 0, capNeeded)
		}
	} else {
		runes = make([]rune, 0, capNeeded)
	}
	var widths []int8
	if v := widthSlicePool.Get(); v != nil {
		w := v.([]int8)
		if cap(w) >= capNeeded {
			widths = w[:0]
		} else {
			widths = make([]int8, 0, capNeeded)
		}
	} else {
		widths = make([]int8, 0, capNeeded)
	}

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

// lineMeasureAdvance returns the screen column after rendering one character.
// Mirrors src/display.c line_measure_advance.
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

// lineColAtOffset returns the screen column corresponding to byte offset in lp.
func lineColAtOffset(lp *buffer.Line, offset uint) int {
	if lp == nil {
		return 0
	}
	col := 0
	i := uint(0)
	for i < offset && i < lp.Len() {
		b := lp.Data[i]
		if b < 0x80 {
			col = lineMeasureAdvance(col, rune(b))
			i++
			continue
		}
		r, size := utf8.DecodeRune(lp.Data[i:])
		if r == utf8.RuneError && size == 1 {
			col = lineMeasureAdvance(col, rune(b))
			i++
			continue
		}
		col = lineMeasureAdvance(col, r)
		i += uint(size)
	}
	return col
}

// ---- Debug/compatibility stubs --------------------------------------------------

// RenderModeline is a public wrapper for renderModeline (called from minibuf.go etc.)
func RenderModeline(wp *window.Window) {
	renderModeline(wp)
}

// debugDisplayLog writes a diagnostic message when debug logging is enabled.
var debugLogs bool

func debugDisplayLog(format string, args ...any) {
	if !debugLogs {
		return
	}
	f, err := os.OpenFile("/tmp/jem-display.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, time.Now().Format(time.RFC3339Nano)+" "+format+"\n", args...)
}

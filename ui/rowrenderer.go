package ui

import (
	"unicode/utf8"
)

// RowRenderer encapsulates logic to build RenderSpan sequences for a line
// and fill per-cell text/style outputs. It uses global pools for temporary
// buffers to minimize allocations.

type RowRenderer struct{}

var rowRenderer RowRenderer

// RenderLine fills textOut and styleOut (both must have length >= maxCols)
// with the per-cell rune and style values for the given line, and returns
// the RenderSpan slice describing the bytes to emit for the row.
func (r *RowRenderer) RenderLine(lp *Line, textOut []rune, styleOut []TextStyle, maxCols int) []RenderSpan {
	spans := make([]RenderSpan, 0, 4)
	if lp == nil {
		// fill with spaces
		for i := 0; i < maxCols; i++ {
			textOut[i] = ' '
			styleOut[i] = TextStyleDefault
		}
		space := make([]byte, maxCols)
		for i := range space {
			space[i] = ' '
		}
		spans = append(spans, RenderSpan{Style: TextStyleDefault, Bytes: space})
		return spans
	}

	// use a per-row scratch buffer from the pool
	var curBytes []byte
	if v := renderBufPool.Get(); v != nil {
		curBytes = v.([]byte)
		curBytes = curBytes[:0]
	} else {
		curBytes = make([]byte, 0, maxCols*3)
	}
	defer func() { renderBufPool.Put(curBytes[:0]) }()

	curStyle := TextStyleDefault
	var style TextStyle
	col := 0
	rIdx := 0
	// Ensure output slices have expected length
	for i := 0; i < maxCols; i++ {
		textOut[i] = ' '
		styleOut[i] = TextStyleDefault
	}

	// Walk runes and build spans
	for ci := 0; ci < len(lp.RuneCache) && rIdx < maxCols; ci++ {
		r := lp.RuneCache[ci]
		w := int(lp.WidthCache[ci])
		if w <= 0 {
			w = 1
		}

		// handle tab (width depends on current column, not per-rune cache)
		if r == '\t' {
			target := lineMeasureAdvance(col, '\t')
			tab := target - col
			if tab <= 0 {
				tab = 8
			}
			if curStyle != TextStyleDefault {
				if len(curBytes) > 0 {
					spans = append(spans, RenderSpan{Style: curStyle, Bytes: append([]byte(nil), curBytes...)})
					curBytes = curBytes[:0]
				}
				curStyle = TextStyleDefault
			}
			for s := 0; s < tab && rIdx < maxCols; s++ {
				curBytes = append(curBytes, ' ')
				textOut[rIdx] = ' '
				styleOut[rIdx] = TextStyleDefault
				rIdx++
				col++
			}
			continue
		}

		// control characters
		if (r < 0x20 && r != '\n') || r == 0x7f {
			ctl1 := byte('^')
			ctl2 := byte(r) ^ 0x40
			if curStyle != TextStyleDefault {
				if len(curBytes) > 0 {
					spans = append(spans, RenderSpan{Style: curStyle, Bytes: append([]byte(nil), curBytes...)})
					curBytes = curBytes[:0]
				}
				curStyle = TextStyleDefault
			}
			if rIdx < maxCols {
				curBytes = append(curBytes, ctl1)
				textOut[rIdx] = rune(ctl1)
				styleOut[rIdx] = TextStyleDefault
				rIdx++
			}
			if rIdx < maxCols {
				curBytes = append(curBytes, ctl2)
				textOut[rIdx] = rune(ctl2)
				styleOut[rIdx] = TextStyleDefault
				rIdx++
			}
			col += 2
			continue
		}

		// regular rune: encode into tmp bytes
		var tmp [4]byte
		n := utf8.EncodeRune(tmp[:], r)
		b := tmp[:n]
		// use style derived from syntax if present
		if ci < len(lp.SyntaxStyles) {
			style = lp.SyntaxStyles[ci]
		} else {
			style = TextStyleDefault
		}
		if style != curStyle {
			if len(curBytes) > 0 {
				spans = append(spans, RenderSpan{Style: curStyle, Bytes: append([]byte(nil), curBytes...)})
				curBytes = curBytes[:0]
			}
			curStyle = style
		}
		curBytes = append(curBytes, b...)
		// update cell state
		if rIdx < maxCols {
			textOut[rIdx] = r
			styleOut[rIdx] = style
		}
		if w == 2 {
			if rIdx+1 < maxCols {
				textOut[rIdx+1] = 0
				styleOut[rIdx+1] = style
			}
			rIdx += 2
			col += 2
		} else {
			rIdx++
			col++
		}
	}

	if len(curBytes) > 0 {
		spans = append(spans, RenderSpan{Style: curStyle, Bytes: append([]byte(nil), curBytes...)})
	}

	if rIdx < maxCols {
		rem := maxCols - rIdx
		spaceBytes := make([]byte, rem)
		for i := 0; i < rem; i++ {
			spaceBytes[i] = ' '
			textOut[rIdx+i] = ' '
			styleOut[rIdx+i] = TextStyleDefault
		}
		spans = append(spans, RenderSpan{Style: TextStyleDefault, Bytes: spaceBytes})
		rIdx = maxCols
	}

	return spans
}

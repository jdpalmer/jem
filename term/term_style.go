package term

import (
	"sync"

	"github.com/jdpalmer/jem/buffer"
)

var termBaseColorRgb = [8]string{
	"7;54;66",
	"220;50;47",
	"133;153;0",
	"181;137;0",
	"38;139;210",
	"211;54;130",
	"42;161;152",
	"238;232;213",
}

var termSolColorRgb = [9]string{
	"0;43;54",
	"7;54;66",
	"88;110;117",
	"101;123;131",
	"131;148;150",
	"147;161;161",
	"238;232;213",
	"253;246;227",
	"38;139;210",
}

var (
	termStyleCacheMu sync.Mutex
	termStyleCache   = make(map[buffer.TextStyle][]byte)
)

func termColorRgbAt(colorIndex buffer.TermColor) (rgb string, ok bool) {
	if colorIndex <= buffer.TermColorWhite {
		return termBaseColorRgb[colorIndex], true
	}
	if colorIndex >= 17 && colorIndex <= 25 {
		return termSolColorRgb[colorIndex-17], true
	}
	return "", false
}

func termAppendColorSgr(buf []byte, colorIndex buffer.TermColor, baseSgr, brightSgr int) int {
	if colorIndex == buffer.TermColorDefault {
		return 0
	}
	if colorIndex <= buffer.TermColorWhite || colorIndex >= 17 {
		rgb, hasRGB := termColorRgbAt(colorIndex)
		if hasRGB {
			n := 0
			buf[n] = ';'
			n++
			n += termAppendU32(buf[n:], uint32(baseSgr+8))
			buf[n] = ';'
			n++
			buf[n] = '2'
			n++
			buf[n] = ';'
			n++
			n += copy(buf[n:], rgb)
			return n
		}
		if colorIndex <= buffer.TermColorWhite {
			n := 0
			buf[n] = ';'
			n++
			n += termAppendU32(buf[n:], uint32(baseSgr+int(colorIndex)))
			return n
		}
		return 0
	}
	if colorIndex >= 8 && colorIndex < 17 {
		n := 0
		buf[n] = ';'
		n++
		n += termAppendU32(buf[n:], uint32(brightSgr+int(colorIndex)-8))
		return n
	}
	return 0
}

func termWriteStyle(buf []byte, style buffer.TextStyle) int {
	fg := style.Fg()
	bg := style.Bg()
	if style&buffer.TextStyleReverse != 0 {
		fg, bg = bg, fg
	}

	n := 0
	buf[n] = 0x1b
	n++
	buf[n] = '['
	n++
	buf[n] = '0'

	if style&buffer.TextStyleBold != 0 {
		buf[n] = ';'
		n++
		buf[n] = '1'
		n++
	}
	if style&buffer.TextStyleReverse != 0 {
		buf[n] = ';'
		n++
		buf[n] = '7'
		n++
	}
	if style&buffer.TextStyleUnderline != 0 {
		buf[n] = ';'
		n++
		buf[n] = '4'
		n++
	}

	n += termAppendColorSgr(buf[n:], fg, 30, 90)
	n += termAppendColorSgr(buf[n:], bg, 40, 100)

	buf[n] = 'm'
	n++
	return n
}

func termFormatStyle(style buffer.TextStyle) []byte {
	termStyleCacheMu.Lock()
	if b, ok := termStyleCache[style]; ok {
		termStyleCacheMu.Unlock()
		return b
	}
	termStyleCacheMu.Unlock()

	var seq [96]byte
	n := termWriteStyle(seq[:], style)
	out := make([]byte, n)
	copy(out, seq[:n])

	termStyleCacheMu.Lock()
	termStyleCache[style] = out
	termStyleCacheMu.Unlock()
	return out
}

func ClearStyleCache() {
	termStyleCacheMu.Lock()
	clear(termStyleCache)
	termStyleCacheMu.Unlock()
	termAnsiStyle = 0
}

func SetStyle(style buffer.TextStyle) {
	if termAnsiStyle == style {
		return
	}
	termAnsiStyle = style
	termOut.Write(termFormatStyle(style))
}

func StyleBytes(style buffer.TextStyle) []byte {
	return termFormatStyle(style)
}

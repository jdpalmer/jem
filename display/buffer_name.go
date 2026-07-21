package display

import (
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// BufferNameMaxCols is the max display width for buffer names in the modeline
// and buffer picker. Stored names are unbounded; only presentation is clipped.
const BufferNameMaxCols = 16

// FitBufferName truncates name to at most maxCols display columns, appending
// an ellipsis when clipped. If maxCols <= 0, BufferNameMaxCols is used.
func FitBufferName(name string, maxCols int) string {
	if maxCols <= 0 {
		maxCols = BufferNameMaxCols
	}
	if name == "" || runewidth.StringWidth(name) <= maxCols {
		return name
	}
	if maxCols <= 1 {
		return "…"
	}
	budget := maxCols - 1
	width := 0
	end := 0
	for i, r := range name {
		rw := runewidth.RuneWidth(r)
		if width+rw > budget {
			break
		}
		width += rw
		end = i + utf8.RuneLen(r)
	}
	return name[:end] + "…"
}

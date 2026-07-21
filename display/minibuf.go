package display

// minibuf.go - Minibuffer presentation (message line); editing lives in minibuffer.

import (
	"fmt"
	"github.com/jdpalmer/jem/minibuffer"
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/term"
)

// MBHistoryAdd appends text to the global minibuffer history.
func MBHistoryAdd(text string) {
	minibuffer.MinibufferHistoryAdd(text)
}

// ---- Message line rendering ---------------------------------------------------

// mlBegin starts rendering on the message line (resets gutter clip like C ml_begin).
func mlBegin(style buffer.TextStyle) {
	clipLeftCol = 0
	screenMove(term.Rows(), 0)
	screenSetStyle(style)
}

// mlFinish ends message-line rendering: erase trailing cells, flush, set cursor.
func mlFinish(cursorCol int, messagePresent bool) {
	screenEraseEol()
	screenFlushRow(term.Rows(), cursorCol)
	Active.MessagePresent = messagePresent
}

func MBWrite(format string, args ...interface{}) {
	var msg string
	if len(args) == 0 {
		msg = format
	} else {
		msg = fmt.Sprintf(format, args...)
	}
	mlBegin(Active.Theme.NormalStyle)
	screenPutBytes([]byte(msg))
	mlFinish(0, len(msg) > 0)
}

func MBClear() {
	mlBegin(Active.Theme.NormalStyle)
	mlFinish(0, false)
}

// displayWidthBytes returns the display column width of the first endOff bytes
// of text (treats each rune as width 1 — sufficient for minibuffer prompts).
func displayWidthBytes(text []byte, endOff int) int {
	if endOff > len(text) {
		endOff = len(text)
	}
	count := 0
	for o := 0; o < endOff; {
		r, size := utf8.DecodeRune(text[o:endOff])
		if r == utf8.RuneError && size == 1 {
			o++
			count++
			continue
		}
		o += size
		count++
	}
	return count
}

// MBWritePromptStyle renders prompt+text on the message line with the cursor
// placed at the column corresponding to cpos (byte offset into text).
func MBWritePromptStyle(prompt string, text []byte, cpos int, style buffer.TextStyle) {
	mlBegin(style)
	screenPutBytes([]byte(prompt))
	cursorCol := displayWidthBytes([]byte(prompt), len(prompt)) + displayWidthBytes(text, cpos)
	screenPutBytes(text)
	if cursorCol < 0 {
		cursorCol = 0
	}
	mlFinish(cursorCol, true)
}

func MBWritePrompt(prompt string, text []byte, cpos int) {
	MBWritePromptStyle(prompt, text, cpos, Active.Theme.NormalStyle)
}

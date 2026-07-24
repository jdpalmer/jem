package search

import (
	"bytes"
	"github.com/jdpalmer/jem/display"

	"github.com/jdpalmer/jem/buffer"
)

func writeISearchPrompt(label string, pattern []byte, cpos int, failing bool, buf *buffer.Buffer) {
	prompt := label
	if searchScopeIsAllBuffers() && buf != nil {
		prompt += "[all " + buf.Name + "]: "
	} else {
		prompt += ": "
	}
	style := display.Active.Theme.NormalStyle
	if failing {
		style = buffer.MakeTextStyle(buffer.TermColorRed, display.Active.Theme.NormalStyle.Bg(), 0)
	}
	end := bytes.IndexByte(pattern, 0)
	if end < 0 {
		end = len(pattern)
	}
	display.MBWritePromptStyle(prompt, pattern[:end], cpos, style)
}

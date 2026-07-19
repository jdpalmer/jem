package search

import (
	"bytes"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/model"
)

func writeISearchPrompt(label string, pattern []byte, cpos int, failing bool, bp *buffer.Buffer) {
	prompt := label
	if searchScopeIsAllBuffers() && bp != nil {
		prompt += "[all " + bp.Name + "]: "
	} else {
		prompt += ": "
	}
	style := model.State.Theme.NormalStyle
	if failing {
		style = buffer.MakeTextStyle(buffer.TermColorRed, model.State.Theme.NormalStyle.Bg(), 0)
	}
	end := bytes.IndexByte(pattern, 0)
	if end < 0 {
		end = len(pattern)
	}
	mbWritePromptStyle(prompt, pattern[:end], cpos, style)
}

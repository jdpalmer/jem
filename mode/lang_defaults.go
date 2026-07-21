package mode

import "github.com/jdpalmer/jem/buffer"

// goIndentCols is one gofmt indent step in display columns (one tab).
const goIndentCols = 8

func indentDefaultFor(mode buffer.LangMode) buffer.IndentConfig {
	switch mode {
	case buffer.LModeGo:
		return buffer.IndentConfig{Width: goIndentCols}
	case buffer.LModePython:
		return buffer.IndentConfig{Width: 4, Continued: 4}
	default:
		// C-family and text: 2-space step.
		return buffer.IndentConfig{Width: 2}
	}
}

// ApplyLangIndentDefaults sets buf.Indent from the mode's IndentDefault.
func ApplyLangIndentDefaults(buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	buf.Indent = LangModeInfo(buf.LangMode).IndentDefault
}

package mode

import "github.com/jdpalmer/jem/buffer"

// goIndentCols is one gofmt indent step in display columns (one tab).
const goIndentCols = 8

// ApplyLangIndentDefaults adjusts per-language indent settings after LangMode is set.
// Global buffer defaults (from OnBufferCreate) remain; Go overrides the C step to one tab.
func ApplyLangIndentDefaults(buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	switch buf.LangMode {
	case buffer.LModeGo:
		buf.CIndent = goIndentCols
		buf.CBrace = 0
		buf.CColonOffset = 0
	}
}

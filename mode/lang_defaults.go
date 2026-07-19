package mode

import "github.com/jdpalmer/jem/buffer"

// goIndentCols is one gofmt indent step in display columns (one tab).
const goIndentCols uint32 = 8

// ApplyLangIndentDefaults adjusts per-language indent settings after LangMode is set.
// Global buffer defaults (from OnBufferCreate) remain; Go overrides the C step to one tab.
func ApplyLangIndentDefaults(bp *buffer.Buffer) {
	if bp == nil {
		return
	}
	switch bp.LangMode {
	case buffer.LModeGo:
		bp.CIndent = goIndentCols
		bp.CBrace = 0
		bp.CColonOffset = 0
	}
}

package editor

import (
	"github.com/jdpalmer/jem/modes"
	sess "github.com/jdpalmer/jem/session"
)

func initModeHooks() {
	modes.PackageHooks = modes.Hooks{
		UndoBeginCommand:      UndoBeginCommand,
		UndoEndCommand:        UndoEndCommand,
		BufferSetText:         bufferSetText,
		WindowInsertNewline:   windowInsertNewline,
		WindowInsertText:      windowInsertText,
		WindowInsertCodepoint: windowInsertCodepoint,
		WindowSetCursor:       sess.WindowSetCursor,
		Message: func(msg string) {
			mbWrite("%s", msg)
		},
		DefaultGotoMatch: CmdSyntaxGotoMatch,
	}
}

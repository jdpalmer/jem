package editor

import (
	"github.com/jdpalmer/jem/modes"
)

func initModeHooks() {
	modes.PackageHooks = modes.Hooks{
		UndoBeginCommand:      UndoBeginCommand,
		UndoEndCommand:        UndoEndCommand,
		BufferSetText:         bufferSetText,
		WindowInsertNewline:   windowInsertNewline,
		WindowInsertText:      windowInsertText,
		WindowInsertCodepoint: windowInsertCodepoint,
		Message: func(msg string) {
			mbWrite("%s", msg)
		},
		DefaultGotoMatch: CmdSyntaxGotoMatch,
	}
}

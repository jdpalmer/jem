package editor

import (
	"github.com/jdpalmer/jem/app"
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
		WindowSetCursor:       app.WindowSetCursor,
		Message: func(msg string) {
			mbWrite("%s", msg)
		},
		DefaultGotoMatch: CmdSyntaxGotoMatch,
	}
}

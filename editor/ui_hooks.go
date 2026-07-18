package editor

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/ui"
)

func initUIHooks() {
	ui.PackageHooks = ui.Hooks{
		DecodeKeyChar:        decodeKeyChar,
		ApplyMetaPrefixToKey: applyMetaPrefixToKey,
		ApplyCtlxPrefix:      applyCtlxPrefix,
		RunCommandByName: func(name string) bool {
			cmd := commandByName(name)
			if cmd == nil || cmd.Fn == nil {
				return false
			}
			return cmd.Fn(false, 1)
		},
		Abort:                       func() { CmdAbort(false, 1) },
		MBWrite:                     mbWrite,
		MarkPushCurrent:             app.MarkPushCurrent,
		TagsMaybeShowCallHint:       tagsMaybeShowCallHint,
		AnyUnsavedBuffers:           anyUnsavedBuffers,
		GitLineDiff:                 gitLineDiff,
		GitModelineText:             gitModelineText,
		KillBegin:                   killBegin,
		KillAppend:                  killAppend,
		KillWriteClipboard:          killWriteClipboard,
		KillReadClipboard:           killReadClipboard,
		KillBytes:                   killBytes,
		MacroPlayPrompt:             macroPlayPrompt,
		MacroRecordMinibufferResult: macroRecordMinibufferResult,
		EditorInsertPaste:           editorInsertPaste,
		CommandsProvider:            commandsProvider,
		BuildCommandList:            buildCommandList,
	}
}

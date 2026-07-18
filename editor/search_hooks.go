package editor

import (
	"github.com/jdpalmer/jem/search"
	sess "github.com/jdpalmer/jem/session"
	"github.com/jdpalmer/jem/term"
)

func initSearchHooks() {
	search.PackageHooks = search.Hooks{
		MBWrite:            mbWrite,
		MBClear:            mbClear,
		MBReadString:       mbReadString,
		MBWritePromptStyle: mbWritePromptStyle,
		MBHistoryAdd:       mbHistoryAdd,
		MBEditKeyHistory:   mbEditKeyHistory,
		DisplayUpdate:      DisplayUpdate,
		MarkPushCurrent:    sess.MarkPushCurrent,
		ReadKey: func() (uint32, bool) {
			k, ok := <-GlobalMinibufKeyCh
			return k, ok
		},
		IsPasteRedrawKey: isPasteRedrawKey,
		SetText:          bufferSetText,
		Beep:             term.Beep,
	}
}

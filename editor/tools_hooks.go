package editor

import "github.com/jdpalmer/jem/tools"

func initToolsHooks() {
	tools.PackageHooks = tools.Hooks{
		MBWrite:                 mbWrite,
		MBClear:                 mbClear,
		MBHistoryAdd:            mbHistoryAdd,
		MBReadString:            mbReadString,
		MBReadStringCap:         mbReadStringCap,
		MBReadFuzzyListExString: mbReadFuzzyListExString,
		MarkPushCurrent:         markPushCurrent,
		VisitLocation:           fileVisitLocation,
		SwitchBuffer:            editorSwitchBuffer,
		Abort:                   func() { CmdAbort(false, 1) },
		TermFreezeInput:         TermFreezeInput,
		TermThawInput:           TermThawInput,
		ReadKey: func() (uint32, bool) {
			var k uint32
			ok := editorReadKey(&k)
			return k, ok
		},
		WindowRetile: windowRetile,
	}
}

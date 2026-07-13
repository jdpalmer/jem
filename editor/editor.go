package editor

// editor.go - Editor state and initialization (translation of editor.c)

import sess "github.com/jdpalmer/jem/session"

func EditorInit(firstBufferName string) {
	sess.App.BufferCount = 0
	sess.App.WindowCount = 0

	bp := sess.BufferCreate(&sess.App.EditorRuntimeState)
	if bp != nil {
		bp.Name = sess.TruncateBufferName(firstBufferName)
		sess.SetCurrentBuffer(bp)
	}

	wp := sess.WindowCreate()
	if wp != nil {
		sess.WindowSelect(wp)
	}
	sess.WindowRetile()

	sess.App.MovementState = CmdStateNone
	sess.App.KillState = CmdStateNone
	macroInit()
	sess.PackageHooks = sess.Hooks{
		UndoForgetBuffer: UndoForgetBuffer,
		SwitchBuffer:     editorSwitchBuffer,
	}
	initModeHooks()
	initBufferSyntaxHooks()
	initSearchHooks()
	initUIHooks()
	initToolsHooks()
}

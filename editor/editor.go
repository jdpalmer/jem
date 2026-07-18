package editor

// editor.go - Editor state and initialization (translation of editor.c)

import "github.com/jdpalmer/jem/app"

func EditorInit(firstBufferName string) {
	app.State.BufferCount = 0
	app.State.WindowCount = 0

	bp := app.BufferCreate(&app.State.EditorRuntimeState)
	if bp != nil {
		bp.Name = app.TruncateBufferName(firstBufferName)
		app.SetCurrentBuffer(bp)
	}

	wp := app.WindowCreate()
	if wp != nil {
		app.WindowSelect(wp)
	}
	app.WindowRetile()

	app.State.MovementState = CmdStateNone
	app.State.KillState = CmdStateNone
	macroInit()
	app.PackageHooks = app.Hooks{
		UndoForgetBuffer: UndoForgetBuffer,
		SwitchBuffer:     editorSwitchBuffer,
	}
	initModeHooks()
	initBufferSyntaxHooks()
	initSearchHooks()
	initUIHooks()
	initToolsHooks()
}

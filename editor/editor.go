package editor

// editor.go - Editor state and initialization (translation of editor.c)

import "github.com/jdpalmer/jem/app"

func EditorInit(firstBufferName string) {
	e := ensureCurrent()

	app.State.Buffers = nil
	app.State.WINDOWS = nil

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

	app.State.MovementState = app.CmdStateNone
	app.State.KillState = app.CmdStateNone
	macroInit()
	e.Services = buildServices()
	Svc = e.Services
	installServices(Svc)
}

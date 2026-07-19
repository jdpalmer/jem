package editor

import (
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/search"
)

// Editor is one editor instance: model state, search, undo, registers,
// and the services façade. Leaf packages reach shared state through
// package-level pointers (model.State, model.History, search.DefaultState)
// that Activate binds to this value.
type Editor struct {
	App       model.AppState
	Search    search.State
	History   buffer.UndoHistory
	Registers map[string][]byte
	Services  *Services

	QuitRequested bool
}

// Current is the active Editor after Activate. Nil until New/Activate.
var Current *Editor

// New creates an Editor with empty state and an empty register map.
func New() *Editor {
	return &Editor{
		Registers: make(map[string][]byte),
	}
}

// Activate binds this Editor as the process-wide active instance.
func (e *Editor) Activate() {
	if e == nil {
		return
	}
	if e.Registers == nil {
		e.Registers = make(map[string][]byte)
	}
	Current = e
	model.Bind(&e.App)
	model.BindHistory(&e.History)
	search.DefaultState = &e.Search
	registerStore = e.Registers
	if e.Services != nil {
		Svc = e.Services
		installServices(Svc)
	}
}

func ensureCurrent() *Editor {
	if Current == nil {
		New().Activate()
	}
	return Current
}

func EditorInit(firstBufferName string) {
	e := ensureCurrent()

	// Install services before BufferCreate so OnBufferCreate applies defaults.
	e.Services = buildServices()
	Svc = e.Services
	installServices(Svc)

	model.State.Buffers = nil
	model.State.Windows = nil

	bp := model.BufferCreate(&model.State.EditorRuntimeState)
	if bp != nil {
		bp.Name = model.TruncateBufferName(firstBufferName)
		model.SetCurrentBuffer(bp)
	}

	wp := model.WindowCreate()
	if wp != nil {
		model.WindowSelect(wp)
	}
	model.WindowRetile()

	model.State.MovementState = model.CmdStateNone
	model.State.KillState = model.CmdStateNone
	clearListeners()
	macroInit()
}

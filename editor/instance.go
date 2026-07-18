package editor

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/edit"
	"github.com/jdpalmer/jem/search"
)

// Editor is one editor instance: application state, search, undo, registers,
// and the services façade. Leaf packages still reach shared state through
// package-level pointers (app.State, edit.History, search.DefaultState) that
// Activate binds to this value.
type Editor struct {
	App       app.AppState
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
	app.Bind(&e.App)
	edit.BindHistory(&e.History)
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

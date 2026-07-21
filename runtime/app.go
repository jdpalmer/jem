package runtime

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/killring"
	"github.com/jdpalmer/jem/search"
	"github.com/jdpalmer/jem/window"
)

// App is one editor process instance: process, display, search, undo, registers,
// and the services façade. Leaf packages reach shared state through
// package-level pointers (State, History, search.DefaultState)
// that Activate binds to this value.
type App struct {
	Proc      ProcState
	Display   display.State
	Buffers   buffer.List
	Windows   window.State
	Search    search.State
	History   buffer.UndoHistory
	Registers map[string][]byte
	Services  *Services

	QuitRequested bool
}

// Current is the active App after Activate. Nil until New/Activate.
var Current *App

// New creates an App with empty state and an empty register map.
func New() *App {
	return &App{
		Registers: make(map[string][]byte),
	}
}

// Activate binds this App as the process-wide active instance.
func (e *App) Activate() {
	if e == nil {
		return
	}
	if e.Registers == nil {
		e.Registers = make(map[string][]byte)
	}
	Current = e
	BindState(&e.Proc)
	display.Bind(&e.Display)
	buffer.BindList(&e.Buffers)
	window.Bind(&e.Windows)
	BindHistory(&e.History)
	search.DefaultState = &e.Search
	registerStore = e.Registers
	if e.Services != nil {
		Svc = e.Services
		installServices(Svc)
	}
}

func ensureCurrent() *App {
	if Current == nil {
		New().Activate()
	}
	return Current
}

func AppInit(firstBufferName string) {
	e := ensureCurrent()

	// Install services before BufferCreate so OnBufferCreate applies defaults.
	e.Services = buildServices()
	Svc = e.Services
	installServices(Svc)

	buffer.All.Buffers = nil
	buffer.All.Current = nil
	buffer.All.NextSerial = 0
	window.Active.Windows = nil
	window.Active.CurrentWindow = nil

	bp := buffer.Create()
	if bp != nil {
		bp.Name = buffer.TruncateName(firstBufferName)
		buffer.SetCurrent(bp)
	}

	wp := window.WindowCreate()
	if wp != nil {
		window.WindowSelect(wp)
	}
	window.WindowRetile()

	killring.ClearSequence()
	clearListeners()
	macroInit()
}

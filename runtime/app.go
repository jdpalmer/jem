package runtime

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/killring"
	"github.com/jdpalmer/jem/register"
	"github.com/jdpalmer/jem/search"
	"github.com/jdpalmer/jem/window"
)

// App is one editor process instance: process, display, search, undo, and named registers.
// Leaf packages reach shared state through package-level pointers (State, History,
// search.DefaultState) that Activate binds to this value.
type App struct {
	Proc      ProcState
	Display   display.State
	Buffers   buffer.List
	Windows   window.State
	Search    search.State
	History   buffer.UndoHistory
	Registers map[string][]byte

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
	register.Bind(e.Registers)
}

func ensureCurrent() *App {
	if Current == nil {
		New().Activate()
	}
	return Current
}

func AppInit(firstBufferName string) {
	ensureCurrent()

	buffer.All.Buffers = nil
	buffer.All.Current = nil
	buffer.All.NextSerial = 0
	window.Active.Windows = nil
	window.Active.CurrentWindow = nil

	buf := buffer.Create()
	if buf != nil {
		buf.Name = firstBufferName
		buffer.SetCurrent(buf)
	}

	win := window.WindowCreate()
	if win != nil {
		window.WindowSelect(win)
	}
	window.WindowRetile()

	killring.ClearSequence()
	clearListeners()
	macroInit()
}

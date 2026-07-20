package window

import "github.com/jdpalmer/jem/buffer"

// Hooks connects window operations to buffer/session helpers window cannot import.
type Hooks struct {
	CurrentBuffer    func() *buffer.Buffer
	SetCurrentBuffer func(bp *buffer.Buffer)
	BeginCommand     func()
	EndCommand       func()
	SetText          func(bp *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error
	BufferFind       func(name string) *buffer.Buffer
	BufferCreate     func() *buffer.Buffer
}

// PackageHooks is set by the runtime during init.
var PackageHooks Hooks

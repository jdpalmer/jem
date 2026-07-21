package window

import "github.com/jdpalmer/jem/buffer"

// Hooks supplies edit helpers window cannot import from runtime (cycle).
// Buffer list APIs are called directly via package buffer.
type Hooks struct {
	BeginCommand func()
	EndCommand   func()
	SetText      func(buf *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error
}

// PackageHooks is set by the runtime during init.
var PackageHooks Hooks

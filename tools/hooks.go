package tools

import (
	"github.com/jdpalmer/jem/buffer"
)

// Hooks are editor-owned callbacks that tools cannot import directly (cycle).
// Minibuffer, mark push, window retile, and term freeze/thaw call view/term
// directly; only visit/switch/abort/key-read remain hooked.
type Hooks struct {
	VisitLocation func(path string, line, column uint32) bool
	SwitchBuffer  func(bp *buffer.Buffer)
	Abort         func()
	ReadKey       func() (uint32, bool)
}

var PackageHooks Hooks

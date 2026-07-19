package model

import (
	"github.com/jdpalmer/jem/buffer"
)

type Hooks struct {
	UndoForgetBuffer func(bp *buffer.Buffer)
	SwitchBuffer     func(bp *buffer.Buffer)
	// OnBufferCreate applies editor defaults (indent widths, fill-column, …).
	OnBufferCreate func(bp *buffer.Buffer)
}


var PackageHooks Hooks

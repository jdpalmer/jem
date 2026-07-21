package display

import (
	"github.com/jdpalmer/jem/buffer"
)

// Hooks are runtime-owned callbacks display cannot import directly (cycle).
// Async Ask* APIs live on runtime (and on search/tools PackageHooks); display
// only paints prompts and uses these for input decode, git gutters, and macros.
type Hooks struct {
	ApplyCtlxPrefix             func(second uint32) uint32
	GitLineDiff                 func(buf *buffer.Buffer, lineNumber uint) int
	GitModelineText             func(buf *buffer.Buffer) string
	MacroRecordMinibufferResult func(text []byte)
}

var PackageHooks Hooks

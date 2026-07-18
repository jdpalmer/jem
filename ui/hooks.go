package ui

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
)

type CommandFunc func(f bool, n int) bool

// Hooks are editor-owned callbacks ui cannot import directly (cycle).
// Kill ring, key decode/meta, paste, and macro play live in edit/ui/app;
// git modeline stays hooked (tools↔ui cycle).
type Hooks struct {
	ApplyCtlxPrefix             func(second uint32) uint32
	RunCommandByName            func(name string) bool
	Abort                       func()
	GitLineDiff                 func(bp *buffer.Buffer, lineNumber uint) app.GitLineDiff
	GitModelineText             func(bp *buffer.Buffer) string
	MacroRecordMinibufferResult func(text []byte)
	CommandsProvider            func(ctx any, idx uint) []byte
	BuildCommandList            func() []string
}

var PackageHooks Hooks

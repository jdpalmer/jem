package ui

type CommandFunc func(f bool, n int) bool

type Hooks struct {
	DecodeKeyChar               func(key uint32, controlContext bool) uint32
	ApplyMetaPrefixToKey        func(k uint32) uint32
	ApplyCtlxPrefix             func(second uint32) uint32
	RunCommandByName            func(name string) bool
	Abort                       func()
	MBWrite                     func(format string, args ...any)
	MarkPushCurrent             func()
	TagsMaybeShowCallHint       func()
	AnyUnsavedBuffers           func() bool
	GitLineDiff                 func(bp *Buffer, lineNumber uint) GitLineDiff
	GitModelineText             func(bp *Buffer) string
	KillBegin                   func()
	KillAppend                  func(text []byte) bool
	KillWriteClipboard          func()
	KillReadClipboard           func() bool
	KillBytes                   func() []byte
	MacroPlayPrompt             func(buf []byte) (PromptResult, bool)
	MacroRecordMinibufferResult func(text []byte)
	EditorInsertPaste           func(text []byte) bool
	CommandsProvider            func(ctx any, idx uint) []byte
	BuildCommandList            func() []string
	RequestRefresh              func()
}

var PackageHooks Hooks

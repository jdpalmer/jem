package editor

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/modeactions"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
	"github.com/jdpalmer/jem/ui"
)

// Services is the editor shell's callback façade for leaf packages.
// Leaf packages must not import editor (import cycle); they keep package-local
// PackageHooks tables that installServices fills from this one source.
//
// Set once at init via EnsureServices / EditorInit. Not safe for concurrent use.
type Services struct {
	App    app.Hooks
	Buffer buffer.EditSession
	Term   term.Hooks
	Modes  modeactions.Hooks
	Tools  tools.Hooks
	UI     ui.Hooks
}

// Svc is the process-wide services table after EnsureServices or EditorInit.
var Svc *Services

// buildServices constructs the full callback table. Shared helpers (minibuffer,
// mark push, switch buffer, abort) are bound once and reused across packages.
func buildServices() *Services {
	mbWriteFn := ui.MBWrite
	switchBufferFn := editorSwitchBuffer
	abortFn := func() { CmdAbort(false, 1) }

	return &Services{
		App: app.Hooks{
			UndoForgetBuffer: UndoForgetBuffer,
			SwitchBuffer:     switchBufferFn,
		},
		Buffer: buffer.EditSession{
			NoteEdit:                    app.NoteBufferEdit,
			AdjustLocationsAfterReplace: app.AdjustLocationsAfterReplace,
			ReparseFrom:                 syntax.IncrementalReparse,
		},
		Term: term.Hooks{
			OnMouse: func(col, row int) {
				app.State.Mouse.Col = uint32(col)
				app.State.Mouse.Row = uint32(row)
			},
			OnPaste: func(paste []byte) {
				ui.QueuePaste(paste)
			},
			OnResume: func() {
				if term.RefreshSize() {
					ui.DisplayInitHeadless(term.Rows(), term.Cols())
				}
			},
		},
		Modes: modeactions.Hooks{
			Message: func(msg string) {
				mbWriteFn("%s", msg)
			},
			DefaultGotoMatch: CmdSyntaxGotoMatch,
		},
		Tools: tools.Hooks{
			VisitLocation: fileVisitLocation,
			SwitchBuffer:  switchBufferFn,
			Abort:         abortFn,
			ReadKey: func() (uint32, bool) {
				var k uint32
				ok := editorReadKey(&k)
				return k, ok
			},
		},
		UI: ui.Hooks{
			ApplyCtlxPrefix: applyCtlxPrefix,
			RunCommandByName: func(name string) bool {
				cmd := commandByName(name)
				if cmd == nil || cmd.Fn == nil {
					return false
				}
				return cmd.Fn(false, 1)
			},
			Abort:                       abortFn,
			GitLineDiff:                 gitLineDiff,
			GitModelineText:             gitModelineText,
			MacroRecordMinibufferResult: macroRecordMinibufferResult,
			CommandsProvider:            commandsProvider,
			BuildCommandList:            buildCommandList,
		},
	}
}

// installServices copies Svc (or s) into each leaf package's PackageHooks /
// EditSession tables.
func installServices(s *Services) {
	if s == nil {
		return
	}
	app.PackageHooks = s.App
	buffer.SetEditSession(s.Buffer)
	term.PackageHooks = s.Term
	modeactions.PackageHooks = s.Modes
	tools.PackageHooks = s.Tools
	ui.PackageHooks = s.UI
	syncSyntaxPalette()
}

// EnsureServices builds Svc if needed and installs all package callbacks.
// Safe to call before EditorInit (e.g. so term hooks exist before term.Open).
func EnsureServices() {
	if Svc == nil {
		Svc = buildServices()
	}
	installServices(Svc)
}

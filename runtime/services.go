package runtime

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/mode"
	"github.com/jdpalmer/jem/search"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
	"github.com/jdpalmer/jem/window"
)

// Services is the runtime callback façade for leaf packages.
// Leaf packages must not import runtime (import cycle); they keep package-local
// PackageHooks tables that installServices fills from this one source.
//
// Set once at init via EnsureServices / AppInit. Not safe for concurrent use.
type Services struct {
	Buffer  buffer.Hooks
	Term    term.Hooks
	Modes   mode.Hooks
	Tools   tools.Hooks
	Display display.Hooks
	Search  search.Hooks
}

// Svc is the process-wide services table after EnsureServices or AppInit.
var Svc *Services

// buildServices constructs the full callback table.
func buildServices() *Services {
	mbWriteFn := display.MBWrite
	switchBufferFn := window.SwitchBuffer
	abortFn := func() { CmdAbort(false, 1) }

	return &Services{
		Buffer: buffer.Hooks{
			NoteEdit:                    window.NoteBufferEdit,
			AdjustLocationsAfterReplace: window.AdjustLocationsAfterReplace,
			ReparseFrom:                 syntax.IncrementalReparse,
			OnBufferCreate:              bufferApplyVarDefaults,
			OnBufferKill:                window.RetargetAfterBufferKill,
			UndoForgetBuffer:            ForgetBuffer,
		},
		Term: term.Hooks{
			OnMouse: func(col, row int) {
				display.Active.Mouse.Col = uint32(col)
				display.Active.Mouse.Row = uint32(row)
			},
			OnPaste: func(paste []byte) {
				event.Enqueue(event.PasteEvent{Data: paste})
			},
			OnResume: func() {
				if term.RefreshSize() {
					display.DisplayInitHeadless(term.Rows(), term.Cols())
				}
			},
		},
		Modes: mode.Hooks{
			Message: func(msg string) {
				mbWriteFn("%s", msg)
			},
			DefaultGotoMatch: CmdSyntaxGotoMatch,
			BeginCommand:     BeginCommand,
			EndCommand:       EndCommand,
			SetText:          SetText,
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
			AskString: func(prompt, initial string, onDone func(string, minibuffer.PromptResult)) {
				AskString(prompt, initial, onDone)
			},
			AskStringCap: func(prompt, initial string, capacity int, onDone func(string, minibuffer.PromptResult)) {
				AskStringCap(prompt, initial, capacity, onDone)
			},
			AskFuzzyEx: func(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter minibuffer.MbMatchFormatter, displayCtx any, onDone func(string, minibuffer.PromptResult)) {
				AskFuzzyEx(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx, onDone)
			},
		},
		Display: display.Hooks{
			ApplyCtlxPrefix: applyCtlxPrefix,
			GitLineDiff: func(bp *buffer.Buffer, lineNumber uint) int {
				return int(tools.GitLineDiffAt(bp, lineNumber))
			},
			GitModelineText:             tools.GitModelineText,
			MacroRecordMinibufferResult: macroRecordMinibufferResult,
			TakeMacroPromptReply:        TakeMacroPromptReply,
			BeginMinibuf:                BeginMinibuf,
			EndMinibuf:                  EndMinibuf,
			WaitKey:                     WaitKey,
		},
		Search: search.Hooks{
			PushKeySession: PushKeySession,
			SetText:        SetText,
			AskString: func(prompt, initial string, onDone func(string, minibuffer.PromptResult)) {
				AskString(prompt, initial, onDone)
			},
			WaitKey:      WaitKey,
			BeginMinibuf: BeginMinibuf,
			EndMinibuf:   EndMinibuf,
		},
	}
}

// installServices copies Svc (or s) into each leaf package's PackageHooks.
func installServices(s *Services) {
	if s == nil {
		return
	}
	markring.PackageHooks = markring.Hooks{
		CurrentBuffer:    func() *buffer.Buffer { return buffer.All.Current },
		Buffers:          func() []*buffer.Buffer { return buffer.All.Buffers },
		SwitchBuffer:     window.SwitchBuffer,
		SetCurrentBuffer: buffer.SetCurrent,
	}
	window.PackageHooks = window.Hooks{
		BeginCommand: BeginCommand,
		EndCommand:   EndCommand,
		SetText:      SetText,
	}
	buffer.PackageHooks = s.Buffer
	term.PackageHooks = s.Term
	mode.PackageHooks = s.Modes
	tools.PackageHooks = s.Tools
	display.PackageHooks = s.Display
	search.PackageHooks = s.Search
	syncSyntaxPalette()
}

func syncSyntaxPalette() {
	syntax.PackagePalette = syntax.Palette{
		NormalStyle:  display.Active.Theme.NormalStyle,
		CommentStyle: display.Active.Theme.CommentStyle,
	}
}

// EnsureServices builds Svc if needed and installs all package callbacks.
// Safe to call before AppInit (e.g. so term hooks exist before term.Open).
func EnsureServices() {
	if Svc == nil {
		Svc = buildServices()
	}
	installServices(Svc)
}

package editor

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/mode"
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/search"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
	"github.com/jdpalmer/jem/view"
)

// Services is the editor shell's callback façade for leaf packages.
// Leaf packages must not import editor (import cycle); they keep package-local
// PackageHooks tables that installServices fills from this one source.
//
// Set once at init via EnsureServices / EditorInit. Not safe for concurrent use.
type Services struct {
	App    model.Hooks
	Buffer buffer.Hooks
	Term   term.Hooks
	Modes  mode.Hooks
	Tools  tools.Hooks
	View   view.Hooks
	Search search.Hooks
}

// Svc is the process-wide services table after EnsureServices or EditorInit.
var Svc *Services

// buildServices constructs the full callback table. Shared helpers (minibuffer,
// mark push, switch buffer, abort) are bound once and reused across packages.
func buildServices() *Services {
	mbWriteFn := view.MBWrite
	switchBufferFn := model.SwitchBuffer
	abortFn := func() { CmdAbort(false, 1) }

	return &Services{
		App: model.Hooks{
			UndoForgetBuffer: model.ForgetBuffer,
			SwitchBuffer:     switchBufferFn,
			OnBufferCreate:   bufferApplyVarDefaults,
		},
		Buffer: buffer.Hooks{
			NoteEdit:                    model.NoteBufferEdit,
			AdjustLocationsAfterReplace: model.AdjustLocationsAfterReplace,
			ReparseFrom:                 syntax.IncrementalReparse,
		},
		Term: term.Hooks{
			OnMouse: func(col, row int) {
				model.State.Mouse.Col = uint32(col)
				model.State.Mouse.Row = uint32(row)
			},
			OnPaste: func(paste []byte) {
				event.Enqueue(event.PasteEvent{Data: paste})
			},
			OnResume: func() {
				if term.RefreshSize() {
					view.DisplayInitHeadless(term.Rows(), term.Cols())
				}
			},
		},
		Modes: mode.Hooks{
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
		View: view.Hooks{
			ApplyCtlxPrefix:             applyCtlxPrefix,
			GitLineDiff:                 tools.GitLineDiffAt,
			GitModelineText:             tools.GitModelineText,
			MacroRecordMinibufferResult: macroRecordMinibufferResult,
			BeginMinibuf:                BeginMinibuf,
			EndMinibuf:                  EndMinibuf,
			WaitKey:                     WaitKey,
			AskString: func(prompt, initial string, onDone func(string, model.PromptResult)) {
				AskString(prompt, initial, onDone)
			},
			AskStringCap: func(prompt, initial string, capacity int, onDone func(string, model.PromptResult)) {
				AskStringCap(prompt, initial, capacity, onDone)
			},
			AskFuzzy: func(prompt string, provider model.MbNameProviderFn, providerCtx any, providerCount uint, onDone func(string, model.PromptResult)) {
				AskFuzzy(prompt, provider, providerCtx, providerCount, onDone)
			},
			AskFuzzyEx: func(prompt string, provider model.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter model.MbMatchFormatter, displayCtx any, onDone func(string, model.PromptResult)) {
				AskFuzzyEx(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx, onDone)
			},
			AskFilename: func(prompt, initial string, onDone func(string, model.PromptResult)) {
				AskFilename(prompt, initial, onDone)
			},
			AskChoose: func(prompt string, ctx any, labelFn model.MLChoiceLabelFn, count uint8, defaultIdx uint8, onDone func(int16)) {
				AskChoose(prompt, ctx, labelFn, count, defaultIdx, onDone)
			},
		},
		Search: search.Hooks{
			PushKeySession: PushKeySession,
		},
	}
}

// installServices copies Svc (or s) into each leaf package's PackageHooks.
func installServices(s *Services) {
	if s == nil {
		return
	}
	model.PackageHooks = s.App
	buffer.PackageHooks = s.Buffer
	term.PackageHooks = s.Term
	mode.PackageHooks = s.Modes
	tools.PackageHooks = s.Tools
	view.PackageHooks = s.View
	search.PackageHooks = s.Search
	syncSyntaxPalette()
}

func syncSyntaxPalette() {
	syntax.PackagePalette = syntax.Palette{
		NormalStyle:  model.State.Theme.NormalStyle,
		CommentStyle: model.State.Theme.CommentStyle,
	}
}

// EnsureServices builds Svc if needed and installs all package callbacks.
// Safe to call before EditorInit (e.g. so term hooks exist before term.Open).
func EnsureServices() {
	if Svc == nil {
		Svc = buildServices()
	}
	installServices(Svc)
}

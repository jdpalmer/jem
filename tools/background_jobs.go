package tools

import (
	"context"
	"errors"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/window"
)

const (
	backgroundJobNone    = ""
	backgroundJobGrep    = "grep"
	backgroundJobCompile = "compile"
)

// BackgroundJobDone is the payload carried by event.JobDoneEvent.Raw.
type BackgroundJobDone struct {
	Kind string

	// Grep
	GrepRoot    string
	GrepPattern string
	GrepResult  grepSearchResult

	// Compile
	CompileCommand  string
	CompileStdout   string
	CompileStderr   string
	CompileExit     int
	CompileRan      bool
	CompileOutTrunc bool
	CompileErrTrunc bool

	Cancelled bool
}

var (
	backgroundJobCancel context.CancelFunc
	backgroundJobActive string
)

// InitBackgroundJobs resets background-job bookkeeping (no separate done channel).
func InitBackgroundJobs() {
	backgroundJobActive = backgroundJobNone
	backgroundJobCancel = nil
}

// BackgroundJobRunning reports whether a background job is currently active.
func BackgroundJobRunning() bool {
	return backgroundJobActive != ""
}

// RequestBackgroundJobCancel cancels the running background job and returns true.
func RequestBackgroundJobCancel() bool {
	if !BackgroundJobRunning() || backgroundJobCancel == nil {
		return false
	}
	backgroundJobCancel()
	return true
}

func postJobDone(done BackgroundJobDone) {
	event.Enqueue(event.JobDoneEvent{Kind: done.Kind, Raw: done})
}

// StartBackgroundGrep launches a project-wide grep search in the background.
func StartBackgroundGrep(root, pattern string) bool {
	if BackgroundJobRunning() {
		display.MBWrite("[busy: %s running]", backgroundJobActive)
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	backgroundJobCancel = cancel
	backgroundJobActive = backgroundJobGrep
	display.MBWrite("[Searching...] (C-g to cancel)")

	go func() {
		result := grepProjectSearch(ctx, root, pattern)
		cancelled := errors.Is(result.err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)
		postJobDone(BackgroundJobDone{
			Kind:        backgroundJobGrep,
			GrepRoot:    root,
			GrepPattern: pattern,
			GrepResult:  result,
			Cancelled:   cancelled,
		})
	}()
	return true
}

// StartBackgroundCompile runs a shell command in the background and captures output.
func StartBackgroundCompile(command string) bool {
	if BackgroundJobRunning() {
		display.MBWrite("[busy: %s running]", backgroundJobActive)
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	backgroundJobCancel = cancel
	backgroundJobActive = backgroundJobCompile
	display.MBWrite("[Compiling...] (C-g to cancel)")

	go func() {
		stdout, stderr, exitCode, ran, outTrunc, errTrunc := procRunShellContext(ctx, command, compileOutCapacity, compileErrCapacity)
		cancelled := errors.Is(ctx.Err(), context.Canceled)
		postJobDone(BackgroundJobDone{
			Kind:            backgroundJobCompile,
			CompileCommand:  command,
			CompileStdout:   stdout,
			CompileStderr:   stderr,
			CompileExit:     exitCode,
			CompileRan:      ran,
			CompileOutTrunc: outTrunc,
			CompileErrTrunc: errTrunc,
			Cancelled:       cancelled,
		})
	}()
	return true
}

// HandleBackgroundJobDone processes a completed background job and updates the display.
func HandleBackgroundJobDone(done BackgroundJobDone) {
	defer func() {
		backgroundJobActive = backgroundJobNone
		backgroundJobCancel = nil
	}()

	switch done.Kind {
	case backgroundJobGrep:
		handleBackgroundJobGrep(done)
	case backgroundJobCompile:
		handleBackgroundJobCompile(done)
	}
	display.Active.ScreenDirty = true
}

func handleBackgroundJobGrep(done BackgroundJobDone) {
	if done.Cancelled {
		display.MBWrite("[grep cancelled]")
		return
	}
	if done.GrepResult.err != nil {
		display.MBWrite("[grep error: %s]", done.GrepResult.err.Error())
		return
	}

	grepBuf := grepEnsureBuffer()
	if grepBuf == nil {
		display.MBWrite("[cannot create grep buffer]")
		return
	}

	wasReadonly := grepBuf.IsReadonly
	grepBuf.IsReadonly = false
	matchCount, ok := grepFillBuffer(grepBuf, done.GrepRoot, done.GrepResult.matches, done.GrepPattern, done.GrepResult.truncated)
	if !ok {
		grepBuf.IsReadonly = wasReadonly
		display.MBWrite("[grep failed]")
		return
	}
	grepBuf.IsReadonly = true

	markring.PushCurrent()
	if PackageHooks.SwitchBuffer != nil {
		PackageHooks.SwitchBuffer(grepBuf)
	} else {
		window.SwitchBuffer(grepBuf)
	}
	if win := window.Active.CurrentWindow; win != nil {
		win.SetTopLine(1)
		win.SetCursor(buffer.Location{Line: 1, Offset: 0})
		win.Mark = buffer.Location{Line: 0, Offset: 0}
		win.HScroll = 0
		win.ShouldRedraw = true
		win.ShouldUpdateModeLine = true
	}

	if matchCount == 1 {
		display.MBWrite("[1 match]")
	} else {
		display.MBWrite("[%d matches]", matchCount)
	}
}

func handleBackgroundJobCompile(done BackgroundJobDone) {
	if done.Cancelled {
		display.MBWrite("[compile cancelled]")
		return
	}
	if !done.CompileRan {
		display.MBWrite("[compile failed to start]")
		return
	}

	compileBuf := compileEnsureBuffer()
	if compileBuf == nil {
		display.MBWrite("[cannot create compile buffer]")
		return
	}

	wasReadonly := compileBuf.IsReadonly
	compileBuf.IsReadonly = false
	counts, ok := compileFillBuffer(compileBuf, done.CompileCommand, done.CompileStdout, done.CompileStderr,
		done.CompileExit, done.CompileOutTrunc, done.CompileErrTrunc)
	if !ok {
		compileBuf.IsReadonly = wasReadonly
		display.MBWrite("[compile failed]")
		return
	}
	compileBuf.IsReadonly = true

	markring.PushCurrent()
	if PackageHooks.SwitchBuffer != nil {
		PackageHooks.SwitchBuffer(compileBuf)
	} else {
		window.SwitchBuffer(compileBuf)
	}
	if win := window.Active.CurrentWindow; win != nil {
		win.SetTopLine(1)
		win.SetCursor(buffer.Location{Line: 1, Offset: 0})
		win.Mark = buffer.Location{Line: 0, Offset: 0}
		win.HScroll = 0
		win.ShouldRedraw = true
		win.ShouldUpdateModeLine = true
	}

	if done.CompileExit == 0 {
		display.MBWrite("[compile ok: %d diagnostics]", counts.diag)
	} else {
		display.MBWrite("[compile exit %d: %d diagnostics, %d errors, %d warnings]",
			done.CompileExit, counts.diag, counts.errors, counts.warnings)
	}
}

// ResetBackgroundJobsForTests clears job state for tests.
func ResetBackgroundJobsForTests() {
	backgroundJobActive = backgroundJobNone
	backgroundJobCancel = nil
	event.DrainForTest()
}

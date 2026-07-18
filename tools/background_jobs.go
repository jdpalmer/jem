package tools

import (
	"context"
	"errors"
	"github.com/jdpalmer/jem/app"
)

const (
	backgroundJobNone    = ""
	backgroundJobGrep    = "grep"
	backgroundJobCompile = "compile"
)

// BackgroundJobDone is sent on the done channel when a job finishes.
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
	backgroundJobDone   chan BackgroundJobDone
	backgroundJobCancel context.CancelFunc
	backgroundJobActive string
)

func InitBackgroundJobs() {
	backgroundJobDone = make(chan BackgroundJobDone, 1)
}

func BackgroundJobDoneChan() <-chan BackgroundJobDone {
	return backgroundJobDone
}

func BackgroundJobRunning() bool {
	return backgroundJobActive != ""
}

func RequestBackgroundJobCancel() bool {
	if !BackgroundJobRunning() || backgroundJobCancel == nil {
		return false
	}
	backgroundJobCancel()
	return true
}

func StartBackgroundGrep(root, pattern string) bool {
	if BackgroundJobRunning() {
		mbWrite("[busy: %s running]", backgroundJobActive)
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	backgroundJobCancel = cancel
	backgroundJobActive = backgroundJobGrep
	mbWrite("[Searching...] (C-g to cancel)")

	go func() {
		result := grepProjectSearch(ctx, root, pattern)
		cancelled := errors.Is(result.err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled)
		backgroundJobDone <- BackgroundJobDone{
			Kind:        backgroundJobGrep,
			GrepRoot:    root,
			GrepPattern: pattern,
			GrepResult:  result,
			Cancelled:   cancelled,
		}
	}()
	return true
}

func StartBackgroundCompile(command string) bool {
	if BackgroundJobRunning() {
		mbWrite("[busy: %s running]", backgroundJobActive)
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	backgroundJobCancel = cancel
	backgroundJobActive = backgroundJobCompile
	mbWrite("[Compiling...] (C-g to cancel)")

	go func() {
		stdout, stderr, exitCode, ran, outTrunc, errTrunc := procRunShellContext(ctx, command, compileOutCapacity, compileErrCapacity)
		cancelled := errors.Is(ctx.Err(), context.Canceled)
		backgroundJobDone <- BackgroundJobDone{
			Kind:            backgroundJobCompile,
			CompileCommand:  command,
			CompileStdout:   stdout,
			CompileStderr:   stderr,
			CompileExit:     exitCode,
			CompileRan:      ran,
			CompileOutTrunc: outTrunc,
			CompileErrTrunc: errTrunc,
			Cancelled:       cancelled,
		}
	}()
	return true
}

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
	app.State.ScreenDirty = true
}

func handleBackgroundJobGrep(done BackgroundJobDone) {
	if done.Cancelled {
		mbWrite("[grep cancelled]")
		return
	}
	if done.GrepResult.err != nil {
		mbWrite("[grep error: %s]", done.GrepResult.err.Error())
		return
	}

	grepBuf := grepEnsureBuffer()
	if grepBuf == nil {
		mbWrite("[cannot create grep buffer]")
		return
	}

	wasReadonly := grepBuf.IsReadonly
	grepBuf.IsReadonly = false
	matchCount, ok := grepFillBuffer(grepBuf, done.GrepRoot, done.GrepResult.matches, done.GrepPattern, done.GrepResult.truncated)
	if !ok {
		grepBuf.IsReadonly = wasReadonly
		mbWrite("[grep failed]")
		return
	}
	grepBuf.IsReadonly = true

	markPushCurrent()
	editorSwitchBuffer(grepBuf)
	if wp := app.State.CurrentWindow; wp != nil {
		app.WindowSetTopLine(wp, 1)
		app.WindowSetCursor(wp, Location{Line: 1, Offset: 0})
		wp.Mark = Location{Line: 0, Offset: 0}
		wp.HScroll = 0
		wp.ShouldRedraw = true
		wp.ShouldUpdateModeLine = true
	}

	if matchCount == 1 {
		mbWrite("[1 match]")
	} else {
		mbWrite("[%d matches]", matchCount)
	}
}

func handleBackgroundJobCompile(done BackgroundJobDone) {
	if done.Cancelled {
		mbWrite("[compile cancelled]")
		return
	}
	if !done.CompileRan {
		mbWrite("[compile failed to start]")
		return
	}

	compileBuf := compileEnsureBuffer()
	if compileBuf == nil {
		mbWrite("[cannot create compile buffer]")
		return
	}

	wasReadonly := compileBuf.IsReadonly
	compileBuf.IsReadonly = false
	counts, ok := compileFillBuffer(compileBuf, done.CompileCommand, done.CompileStdout, done.CompileStderr,
		done.CompileExit, done.CompileOutTrunc, done.CompileErrTrunc)
	if !ok {
		compileBuf.IsReadonly = wasReadonly
		mbWrite("[compile failed]")
		return
	}
	compileBuf.IsReadonly = true

	markPushCurrent()
	editorSwitchBuffer(compileBuf)
	if wp := app.State.CurrentWindow; wp != nil {
		app.WindowSetTopLine(wp, 1)
		app.WindowSetCursor(wp, Location{Line: 1, Offset: 0})
		wp.Mark = Location{Line: 0, Offset: 0}
		wp.HScroll = 0
		wp.ShouldRedraw = true
		wp.ShouldUpdateModeLine = true
	}

	if done.CompileExit == 0 {
		mbWrite("[compile ok: %d diagnostics]", counts.diag)
	} else {
		mbWrite("[compile exit %d: %d diagnostics, %d errors, %d warnings]",
			done.CompileExit, counts.diag, counts.errors, counts.warnings)
	}
}

// ResetBackgroundJobsForTests clears job state for tests.
func ResetBackgroundJobsForTests() {
	backgroundJobActive = backgroundJobNone
	backgroundJobCancel = nil
	if backgroundJobDone != nil {
		select {
		case <-backgroundJobDone:
		default:
		}
	}
}

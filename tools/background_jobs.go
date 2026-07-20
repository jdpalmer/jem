package tools

import (
	"context"
	"errors"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/window"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/event"
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

func postJobDone(done BackgroundJobDone) {
	event.Enqueue(event.JobDoneEvent{Kind: done.Kind, Raw: done})
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
	if wp := window.Active.CurrentWindow; wp != nil {
		wp.SetTopLine(1)
		wp.SetCursor(buffer.Location{Line: 1, Offset: 0})
		wp.Mark = buffer.Location{Line: 0, Offset: 0}
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
	if wp := window.Active.CurrentWindow; wp != nil {
		wp.SetTopLine(1)
		wp.SetCursor(buffer.Location{Line: 1, Offset: 0})
		wp.Mark = buffer.Location{Line: 0, Offset: 0}
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
	event.DrainForTest()
}

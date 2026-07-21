package tools

import (
	"context"
	"errors"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/window"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/event"
)

func TestGrepProjectSearchCancellation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "slow.go")
	if err := os.WriteFile(path, []byte("package main\n// TODO grep cancel test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan grepSearchResult, 1)
	go func() {
		done <- grepProjectSearch(ctx, dir, "TODO")
	}()
	cancel()
	select {
	case result := <-done:
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("grep search did not finish after cancel")
	}
}

func waitJobDone(t *testing.T) BackgroundJobDone {
	t.Helper()
	select {
	case e := <-event.Chan():
		jd, ok := e.(event.JobDoneEvent)
		if !ok {
			t.Fatalf("expected JobDoneEvent, got %T", e)
		}
		done, ok := jd.Raw.(BackgroundJobDone)
		if !ok {
			t.Fatalf("JobDoneEvent.Raw = %T, want BackgroundJobDone", jd.Raw)
		}
		return done
	case <-time.After(5 * time.Second):
		t.Fatal("job timed out")
		return BackgroundJobDone{}
	}
}

func TestBackgroundJobGrepCompletion(t *testing.T) {
	InitBackgroundJobs()
	ResetBackgroundJobsForTests()
	display.Reset()
	buf := buffer.Create()
	win := window.WindowCreate()
	window.WindowSelect(win)
	buffer.SetCurrent(buf)
	win.Buffer = buf
	PackageHooks = Hooks{
		SwitchBuffer: func(next *buffer.Buffer) {
			buffer.SetCurrent(next)
			if cw := window.Active.CurrentWindow; cw != nil {
				cw.Buffer = next
			}
		},
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\nfunc foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !StartBackgroundGrep(dir, "foo") {
		t.Fatal("failed to start grep job")
	}
	if !BackgroundJobRunning() {
		t.Fatal("expected active grep job")
	}

	done := waitJobDone(t)
	HandleBackgroundJobDone(done)

	if BackgroundJobRunning() {
		t.Fatal("job still marked active after completion")
	}
	if got := buffer.Find(GrepBufferName); got == nil {
		t.Fatal("grep buffer not created")
	}
}

func TestBackgroundJobCancel(t *testing.T) {
	InitBackgroundJobs()
	ResetBackgroundJobsForTests()
	display.Reset()
	buf := buffer.Create()
	win := window.WindowCreate()
	window.WindowSelect(win)
	buffer.SetCurrent(buf)
	win.Buffer = buf
	PackageHooks = Hooks{}

	dir := t.TempDir()
	path := filepath.Join(dir, "big.go")
	var b []byte
	for i := 0; i < 5000; i++ {
		b = append(b, "// TODO line\n"...)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	if !StartBackgroundGrep(dir, "TODO") {
		t.Fatal("failed to start grep job")
	}
	time.Sleep(10 * time.Millisecond)
	if !RequestBackgroundJobCancel() {
		t.Fatal("cancel request failed")
	}

	done := waitJobDone(t)
	if !done.Cancelled {
		t.Fatalf("expected cancelled result, got %+v", done)
	}
	HandleBackgroundJobDone(done)
}

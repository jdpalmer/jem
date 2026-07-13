package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	sess "github.com/jdpalmer/jem/session"
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

func TestBackgroundJobGrepCompletion(t *testing.T) {
	InitBackgroundJobs()
	ResetBackgroundJobsForTests()
	sess.App = sess.AppState{}
	bp := sess.BufferCreate(&sess.App.EditorRuntimeState)
	wp := sess.WindowCreate()
	sess.WindowSelect(wp)
	sess.SetCurrentBuffer(bp)
	wp.Buffer = bp
	PackageHooks = Hooks{
		MBWrite: func(string, ...interface{}) {},
		SwitchBuffer: func(next *sess.Buffer) {
			sess.SetCurrentBuffer(next)
			if cw := sess.App.CurrentWindow; cw != nil {
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

	select {
	case done := <-BackgroundJobDoneChan():
		HandleBackgroundJobDone(done)
	case <-time.After(5 * time.Second):
		t.Fatal("grep job timed out")
	}

	if BackgroundJobRunning() {
		t.Fatal("job still marked active after completion")
	}
	if got := bufferFind(GrepBufferName); got == nil {
		t.Fatal("grep buffer not created")
	}
}

func TestBackgroundJobCancel(t *testing.T) {
	InitBackgroundJobs()
	ResetBackgroundJobsForTests()
	sess.App = sess.AppState{}
	bp := sess.BufferCreate(&sess.App.EditorRuntimeState)
	wp := sess.WindowCreate()
	sess.WindowSelect(wp)
	sess.SetCurrentBuffer(bp)
	wp.Buffer = bp
	PackageHooks = Hooks{
		MBWrite: func(string, ...interface{}) {},
	}

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

	select {
	case done := <-BackgroundJobDoneChan():
		if !done.Cancelled {
			t.Fatalf("expected cancelled result, got %+v", done)
		}
		HandleBackgroundJobDone(done)
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled grep job timed out")
	}
}

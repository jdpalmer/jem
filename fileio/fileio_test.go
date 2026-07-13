package fileio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jdpalmer/jem/session"
)

func resetAppForFileIoTests() {
	session.App = session.AppState{}
}

func initBufferWindowForFileIoTests(t *testing.T) *session.Buffer {
	t.Helper()
	bp := session.BufferCreate(&session.App.EditorRuntimeState)
	if bp == nil {
		t.Fatal("buffer create failed")
	}
	session.SetCurrentBuffer(bp)
	wp := session.WindowCreate()
	if wp == nil {
		t.Fatal("window create failed")
	}
	session.WindowSelect(wp)
	return bp
}

func TestReloadCurrentBufferFromDiskRequiresFilename(t *testing.T) {
	resetAppForFileIoTests()
	_ = initBufferWindowForFileIoTests(t)
	if ReloadCurrentBufferFromDisk("", 1, nil, nil) {
		t.Fatal("expected reload to fail without a filename")
	}
}

func TestReloadCurrentBufferFromDiskReloads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "revert.txt")
	if err := os.WriteFile(path, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resetAppForFileIoTests()
	bp := initBufferWindowForFileIoTests(t)
	bp.FileName = path

	if !LoadCurrentBuffer(path, nil) {
		t.Fatal("LoadCurrentBuffer failed")
	}

	if err := os.WriteFile(path, []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !ReloadCurrentBufferFromDisk(path, 1, nil, nil) {
		t.Fatal("ReloadCurrentBufferFromDisk failed")
	}
	if bp.IsChanged {
		t.Fatal("buffer should be clean after reload")
	}
	line := session.BufferGetLine(bp, 1)
	if line == nil || string(line.Data) != "second" {
		got := ""
		if line != nil {
			got = string(line.Data)
		}
		t.Fatalf("buffer content = %q, want %q", got, "second")
	}
}

func TestCheckReloadCurrentBufferCleanBuffer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watch.txt")
	if err := os.WriteFile(path, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resetAppForFileIoTests()
	bp := initBufferWindowForFileIoTests(t)
	bp.FileName = path

	if !LoadCurrentBuffer(path, nil) {
		t.Fatal("LoadCurrentBuffer failed")
	}
	session.App.CurrentWindow.Cursor = session.MakeLocation(1, 3)

	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	CheckReloadCurrentBuffer(nil, nil, nil)

	line := session.BufferGetLine(bp, 1)
	if line == nil || string(line.Data) != "alpha" {
		got := ""
		if line != nil {
			got = string(line.Data)
		}
		t.Fatalf("buffer line 1 = %q, want %q", got, "alpha")
	}
	if bp.LineCount != 3 {
		t.Fatalf("line_count = %d, want 3", bp.LineCount)
	}
	if session.App.CurrentWindow.Cursor.Line != 1 {
		t.Fatalf("cursor line = %d, want 1", session.App.CurrentWindow.Cursor.Line)
	}
	if bp.IsChanged {
		t.Fatal("buffer should be clean after auto-reload")
	}
}

func TestLoadCommandLineFiles(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		filepath.Join(dir, "a.go"),
		filepath.Join(dir, "b.go"),
		filepath.Join(dir, "c.go"),
	}
	for _, p := range paths {
		if err := os.WriteFile(p, []byte(filepath.Base(p)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	resetAppForFileIoTests()
	initial := initBufferWindowForFileIoTests(t)
	initial.Name = filepath.Base(paths[0])
	LoadCommandLineFiles(paths, filepath.Base, func(path string) bool {
		return LoadCurrentBuffer(path, nil)
	})

	if session.App.BufferCount != 3 {
		t.Fatalf("buffer_count = %d, want 3", session.App.BufferCount)
	}
	wp := session.App.CurrentWindow
	if wp == nil {
		t.Fatal("no window")
	}
	if session.App.CurrentBuffer != wp.Buffer {
		t.Fatal("current buffer should be the first file's buffer")
	}
	line := session.BufferGetLine(session.App.CurrentBuffer, 1)
	if line == nil || string(line.Data) != "a.go" {
		got := ""
		if line != nil {
			got = string(line.Data)
		}
		t.Fatalf("first buffer text = %q, want %q", got, "a.go")
	}
	names := map[string]bool{}
	for i := 0; i < int(session.App.BufferCount); i++ {
		bp := session.App.Buffers[i]
		if bp != nil {
			names[bp.Name] = true
		}
	}
	for _, want := range []string{"a.go", "b.go", "c.go"} {
		if !names[want] {
			t.Fatalf("missing buffer %q, got %v", want, names)
		}
	}
}

func TestCheckReloadCurrentBufferSkipsDirtyBuffer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dirty.txt")
	if err := os.WriteFile(path, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resetAppForFileIoTests()
	bp := initBufferWindowForFileIoTests(t)
	bp.FileName = path

	if !LoadCurrentBuffer(path, nil) {
		t.Fatal("LoadCurrentBuffer failed")
	}
	bp.IsChanged = true

	if err := os.WriteFile(path, []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Avoid prompting by simulating that the user already declined for this mtime.
	bp.DiskChangeNotifiedMtime = FileMtime(path)

	CheckReloadCurrentBuffer(func(string) bool { return false }, nil, nil)

	line := session.BufferGetLine(bp, 1)
	if line == nil || string(line.Data) != "first" {
		got := ""
		if line != nil {
			got = string(line.Data)
		}
		t.Fatalf("buffer content = %q, want %q", got, "first")
	}
}

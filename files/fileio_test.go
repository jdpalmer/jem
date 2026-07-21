package files

import (
	"errors"
	"github.com/jdpalmer/jem/window"
	"os"
	"path/filepath"
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func resetAppForFileIoTests() {
	PackageHooks = Hooks{}
	*buffer.All = buffer.List{}
	*window.Active = window.State{}
}

func initBufferWindowForFileIoTests(t *testing.T) *buffer.Buffer {
	t.Helper()
	bp := buffer.Create()
	if bp == nil {
		t.Fatal("buffer create failed")
	}
	buffer.SetCurrent(bp)
	wp := window.WindowCreate()
	if wp == nil {
		t.Fatal("window create failed")
	}
	wp.Buffer = bp
	window.WindowSelect(wp)
	return bp
}

func TestReloadCurrentBufferFromDiskRequiresFilename(t *testing.T) {
	resetAppForFileIoTests()
	_ = initBufferWindowForFileIoTests(t)
	if err := ReloadCurrentBufferFromDisk("", 1, nil, nil); err == nil {
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

	if err := LoadCurrentBuffer(path, nil); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ReloadCurrentBufferFromDisk(path, 1, nil, nil); err != nil {
		t.Fatal(err)
	}
	if bp.IsChanged {
		t.Fatal("buffer should be clean after reload")
	}
	line := bp.Line(1)
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

	if err := LoadCurrentBuffer(path, nil); err != nil {
		t.Fatal(err)
	}
	window.Active.CurrentWindow.Cursor = buffer.MakeLocation(1, 3)

	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	CheckReloadCurrentBuffer(nil, nil, nil)

	line := bp.Line(1)
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
	if window.Active.CurrentWindow.Cursor.Line != 1 {
		t.Fatalf("cursor line = %d, want 1", window.Active.CurrentWindow.Cursor.Line)
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
	LoadCommandLineFiles(paths, filepath.Base, func(path string) error {
		return LoadCurrentBuffer(path, nil)
	})

	if len(buffer.All.Buffers) != 3 {
		t.Fatalf("buffer_count = %d, want 3", len(buffer.All.Buffers))
	}
	wp := window.Active.CurrentWindow
	if wp == nil {
		t.Fatal("no window")
	}
	if buffer.All.Current != wp.Buffer {
		t.Fatal("current buffer should be the first file's buffer")
	}
	line := buffer.All.Current.Line(1)
	if line == nil || string(line.Data) != "a.go" {
		got := ""
		if line != nil {
			got = string(line.Data)
		}
		t.Fatalf("first buffer text = %q, want %q", got, "a.go")
	}
	names := map[string]bool{}
	for _, bp := range buffer.All.Buffers {
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

	if err := LoadCurrentBuffer(path, nil); err != nil {
		t.Fatal(err)
	}
	bp.IsChanged = true

	if err := os.WriteFile(path, []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Avoid prompting by simulating that the user already declined for this mtime.
	bp.DiskChangeNotifiedMtime = FileMtime(path)

	CheckReloadCurrentBuffer(nil, nil, nil)

	line := bp.Line(1)
	if line == nil || string(line.Data) != "first" {
		got := ""
		if line != nil {
			got = string(line.Data)
		}
		t.Fatalf("buffer content = %q, want %q", got, "first")
	}
}

func TestLoadCurrentBufferReadonly(t *testing.T) {
	resetAppForFileIoTests()
	bp := initBufferWindowForFileIoTests(t)
	bp.IsReadonly = true
	err := LoadCurrentBuffer("anything.txt", nil)
	if !errors.Is(err, ErrReadonly) {
		t.Fatalf("err = %v, want ErrReadonly", err)
	}
}

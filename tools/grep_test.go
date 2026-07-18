package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jdpalmer/jem/app"
)

func TestGrepCompilePatternSmartCase(t *testing.T) {
	re, err := grepCompilePattern("foo")
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString("FOO") {
		t.Fatal("expected smart-case lowercase pattern to match uppercase")
	}

	re, err = grepCompilePattern("Foo")
	if err != nil {
		t.Fatal(err)
	}
	if re.MatchString("foo") {
		t.Fatal("expected case-sensitive pattern not to match different case")
	}
}

func TestGrepProjectSearch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alpha.go"), []byte("package main\nfunc alpha() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "beta.go"), []byte("package main\n// TODO fix\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("alpha.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := grepProjectSearch(t.Context(), dir, "TODO")
	if result.err != nil {
		t.Fatalf("search failed: %v", result.err)
	}
	if len(result.matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(result.matches))
	}
	if result.matches[0].line != 2 {
		t.Fatalf("line = %d, want 2", result.matches[0].line)
	}
	if filepath.Base(result.matches[0].path) != "beta.go" {
		t.Fatalf("path = %q, want beta.go", result.matches[0].path)
	}
}

func TestGrepFillBuffer(t *testing.T) {
	bp := app.BufferCreate(&app.State.EditorRuntimeState)
	if bp == nil {
		t.Fatal("buffer create failed")
	}
	root := "/proj"
	matches := []grepMatch{
		{path: "/proj/src/a.go", line: 10, column: 3, text: "fmt.Println"},
		{path: "/proj/src/b.go", line: 2, column: 1, text: "var x int"},
	}
	count, ok := grepFillBuffer(bp, root, matches, "fmt", false)
	if !ok || count != 2 {
		t.Fatalf("fill failed ok=%v count=%d", ok, count)
	}
	if bp.Name != "" {
		// unnamed buffer is fine
	}
	line := bp.Line(3)
	if line == nil || line.Metadata == nil {
		t.Fatal("expected metadata on match line")
	}
	data, ok := line.Metadata.(*GrepLineData)
	if !ok || data.Path != "/proj/src/a.go" || data.Line != 10 {
		t.Fatalf("metadata = %+v", line.Metadata)
	}
}

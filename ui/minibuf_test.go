package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jdpalmer/jem/term"
)

func TestChoiceRenderIgnoresGutterClip(t *testing.T) {
	DisplayInit()
	*session.App = App{}
	bp := bufferCreate(&session.App.EditorRuntimeState)
	if bp == nil {
		t.Fatal("buffer create failed")
	}
	bp.Name = "alpha"

	clipLeftCol = 10
	mlChoiceRender("Buffer: ", []*Buffer{bp}, bufferChoiceLabel, 1, 0, 0, 0)
	clipLeftCol = 0

	row := &backScreen.Rows[term.Rows()]
	if row.Text[0] != 'B' {
		t.Fatalf("prompt should start at column 0, got %q", row.Text[0])
	}
	want := []rune("Buffer: alpha")
	for i, c := range want {
		if row.Text[i] != c {
			t.Fatalf("col %d = %q, want %q", i, row.Text[i], c)
		}
	}
}

func TestCollectFuzzyPathsIncludesParent(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}

	paths := collectFuzzyPaths(child, "")
	if len(paths) == 0 || paths[0] != "../" {
		t.Fatalf("collectFuzzyPaths(child) = %v, want ../ first", paths)
	}
}

func TestCollectFuzzyPathsRootHasNoParent(t *testing.T) {
	root := string(filepath.Separator)
	paths := collectFuzzyPaths(root, "")
	for _, p := range paths {
		if p == "../" {
			t.Fatalf("collectFuzzyPaths(%q) should not include ../, got %v", root, paths)
		}
	}
}

func TestApplyFilenameSelectionFile(t *testing.T) {
	got := applyFilenameSelection("src/", "foo.go")
	if got != "src/foo.go" {
		t.Fatalf("applyFilenameSelection = %q, want src/foo.go", got)
	}
}

func TestApplyFilenameSelectionParent(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "a")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	childPrefix := child + string(filepath.Separator)
	got := applyFilenameSelection(childPrefix, "../")
	want := dir + string(filepath.Separator)
	if got != want {
		t.Fatalf("applyFilenameSelection = %q, want %q", got, want)
	}
}

func TestFilenameFuzzyScoreParentEntry(t *testing.T) {
	if filenameFuzzyScore("../", "..") <= -1000000 {
		t.Fatal("expected ../ to fuzzy-match ..")
	}
}

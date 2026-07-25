package display

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jdpalmer/jem/file"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/term"
)

func TestChoiceRenderIgnoresGutterClip(t *testing.T) {
	DisplayInit()
	Reset()
	buf := buffer.Create()
	if buf == nil {
		t.Fatal("buffer create failed")
	}
	buf.Name = "alpha"

	clipLeftCol = 10
	label := func(ctx any, idx int) []byte {
		buffers := ctx.([]*buffer.Buffer)
		if int(idx) >= len(buffers) || buffers[idx] == nil {
			return nil
		}
		return []byte(buffers[idx].Name)
	}
	mlChoiceRender("Buffer: ", []*buffer.Buffer{buf}, label, 1, 0, 0, 0)
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

func TestEditorlyFilenameSelectionFile(t *testing.T) {
	got := file.ApplyFilenameSelection("src/", "foo.go")
	if got != "src/foo.go" {
		t.Fatalf("file.ApplyFilenameSelection = %q, want src/foo.go", got)
	}
}

func TestEditorlyFilenameSelectionParent(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "a")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	childPrefix := child + string(filepath.Separator)
	got := file.ApplyFilenameSelection(childPrefix, "../")
	want := dir + string(filepath.Separator)
	if got != want {
		t.Fatalf("file.ApplyFilenameSelection = %q, want %q", got, want)
	}
}

func TestFilenameFuzzyScoreParentEntry(t *testing.T) {
	ok, _ := filenameFuzzyScore("../", "..")
	if !ok {
		t.Fatal("expected ../ to fuzzy-match ..")
	}
}

func TestFuzzyMatchesExceedsSixteen(t *testing.T) {
	names := make([]string, 40)
	for i := range names {
		names[i] = fmt.Sprintf("cmd%02d", i)
	}
	provider := func(ctx any, idx int) []byte {
		list := ctx.([]string)
		if idx < 0 || idx >= len(list) {
			return nil
		}
		return []byte(list[idx])
	}
	got := fuzzyMatches(provider, names, len(names), nil, fuzzyMaxMatches)
	if len(got) < 40 {
		t.Fatalf("fuzzyMatches returned %d, want all 40 (cap is %d)", len(got), fuzzyMaxMatches)
	}
}

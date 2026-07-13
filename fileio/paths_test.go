package fileio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizePathRelative(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join("foo", "bar.txt")
	got := NormalizePath(filepath.Join(dir, rel))
	want := filepath.Join(dir, "foo", "bar.txt")
	if got != want {
		t.Fatalf("NormalizePath = %q, want %q", got, want)
	}
}

func TestPathsEqual(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "foo", "bar.txt")
	b := filepath.Join(dir, "foo", ".", "bar.txt")
	if !PathsEqual(a, b) {
		t.Fatalf("expected %q and %q to be equal", a, b)
	}
}

func TestFindDirWalkUpGit(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(dir, "src")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	root, ok := FindDirWalkUp(child, ".git")
	if !ok || root != dir {
		t.Fatalf("FindDirWalkUp = (%q, %v), want (%q, true)", root, ok, dir)
	}
}

func TestFindFileWalkUp(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "tags.json")
	if err := os.WriteFile(marker, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(dir, "pkg")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	got, ok := FindFileWalkUp(child, "tags.json")
	if !ok || got != marker {
		t.Fatalf("FindFileWalkUp = (%q, %v), want (%q, true)", got, ok, marker)
	}
}

func TestContractHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	got := ContractHome(filepath.Join(home, "src"))
	if got != "~/src/" {
		t.Fatalf("ContractHome = %q, want ~/src/", got)
	}
}

func TestPromptSplit(t *testing.T) {
	dir, name := PromptSplit("src/foo.go")
	if dir != "src"+string(filepath.Separator) || name != "foo.go" {
		t.Fatalf("PromptSplit = (%q, %q)", dir, name)
	}
}

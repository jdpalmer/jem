package editor

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/completion"
)

func TestCompletionPrefixAtPoint(t *testing.T) {
	te := NewTestEditor(t)
	te.LoadText("foo bar")
	te.SetCursor(1, 3)

	got := completion.PrefixAtPoint(te.WP())
	if got != "foo" {
		t.Fatalf("prefix = %q, want foo", got)
	}
}

func TestCompletionCollectCandidates(t *testing.T) {
	te := NewTestEditor(t)
	te.SetLangMode(buffer.LModeGo)
	te.LoadText("fmt.Println(formatter)\nformat := true\n")

	candidates := completion.CollectCandidates(te.BP(), "form")
	if len(candidates) == 0 {
		t.Fatal("expected candidates for prefix form")
	}
	foundFormatter := false
	foundFormat := false
	for _, c := range candidates {
		if c == "formatter" {
			foundFormatter = true
		}
		if c == "format" {
			foundFormat = true
		}
	}
	if !foundFormatter || !foundFormat {
		t.Fatalf("candidates = %v, want formatter and format", candidates)
	}
}

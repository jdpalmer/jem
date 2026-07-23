package syntax

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestDelimiterHighlightAfterKeyword(t *testing.T) {
	line := &buffer.Line{}
	line.Data = []byte("if(")
	line.LangMode = buffer.LModeGo
	line.EnsureCache()

	start := buffer.SynState{DFA: SynStateNormal}
	_, _, styles := tokenizeLineFromState(line, start)
	if len(styles) == 0 || styles[len(styles)-1] == buffer.TextStyleDefault {
		t.Fatalf("expected '(' to be painted, styles=%v", styles)
	}
	if len(styles) != len(line.RuneCache) {
		t.Fatalf("expected styles len %d, got %d", len(line.RuneCache), len(styles))
	}
}

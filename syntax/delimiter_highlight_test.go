package syntax

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestDelimiterHighlightAfterKeyword(t *testing.T) {
	lp := &buffer.Line{}
	lp.Data = []byte("if(")
	lp.LangMode = buffer.LModeGo
	buffer.EnsureLineCache(lp)

	start := SynState{DFA: SS_NORMAL}
	_, _, styles := tokenizeLineFromStateExported(lp, start)
	if len(styles) == 0 || styles[len(styles)-1] == TextStyleDefault {
		t.Fatalf("expected '(' to be painted, styles=%v", styles)
	}
	if len(styles) != len(lp.RuneCache) {
		t.Fatalf("expected styles len %d, got %d", len(lp.RuneCache), len(styles))
	}
}

func TestDelimiterHighlightViaEnterHook(t *testing.T) {
	lp := &buffer.Line{}
	lp.Data = []byte("(")
	lp.LangMode = buffer.LModeGo
	buffer.EnsureLineCache(lp)
	PackagePalette.CommentStyle = buffer.MakeTextStyle(TermColorBlue, TermColorDefault, 0)

	setOnEnterHook(SS_NORMAL, func(line *buffer.Line, syn *SynState, i *int, tokenStart *int, summary *SyntaxLineSummary, styles []buffer.TextStyle, pendingChar int) {
		if *i != 0 {
			return
		}
		callReenterState(line, syn, i, *tokenStart, int('('), styles, summary)
	})
	defer clearOnEnterHooks()

	start := SynState{DFA: SS_NORMAL}
	end, _, styles := tokenizeLineFromStateExported(lp, start)
	if len(styles) != 1 {
		t.Fatalf("expected styles length 1, got %d", len(styles))
	}
	if styles[0] == TextStyleDefault {
		t.Fatalf("expected non-default style after hook pending paint")
	}
	if end.Paren == 0 {
		t.Fatalf("expected paren depth > 0 after painting, got %d", end.Paren)
	}
}

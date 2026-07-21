package syntax

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestOperatorStyleForLang(t *testing.T) {
	if operatorStyleForLang(buffer.LModeGo, ":=") != keywordStyle {
		t.Fatal("expected Go := to use keyword style")
	}
	if operatorStyleForLang(buffer.LModeGo, "++") != keywordStyle {
		t.Fatal("expected Go ++ to use keyword style")
	}
	if operatorStyleForLang(buffer.LModeGo, "===") != buffer.TextStyleDefault {
		t.Fatal("expected unknown operator not to highlight in Go")
	}
	if operatorStyleForLang(buffer.LModeJavaScript, "===") != keywordStyle {
		t.Fatal("expected JS === to use keyword style")
	}
	if operatorStyleForLang(buffer.LModePython, "++") != buffer.TextStyleDefault {
		t.Fatal("expected Python not to treat ++ as an operator")
	}
	if operatorStyleForLang(buffer.LModeNone, "=") != buffer.TextStyleDefault {
		t.Fatal("expected plain text mode not to highlight operators")
	}
}

func TestSyntaxOperatorHighlight(t *testing.T) {
	cases := []struct {
		lang buffer.LangMode
		line string
		at   int
	}{
		{buffer.LModeGo, "x := 1", 2},
		{buffer.LModeGo, "i++", 1},
		{buffer.LModeGo, "a != b", 2},
		{buffer.LModeJavaScript, "a === b", 2},
		{buffer.LModePython, "a ** 2", 2},
	}
	for _, tc := range cases {
		line := makeLine(tc.line)
		line.LangMode = tc.lang
		SyntaxEnsureLine(line)
		if line.SyntaxStyles == nil {
			t.Fatalf("SyntaxStyles nil for %q", tc.line)
		}
		if line.SyntaxStyles[tc.at] != keywordStyle {
			t.Fatalf("%v at %d in %q: expected keyword style, got %v", tc.lang, tc.at, tc.line, line.SyntaxStyles[tc.at])
		}
	}
}

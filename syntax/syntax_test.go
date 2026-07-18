package syntax

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func makeLine(s string) *buffer.Line {
	b := []byte(s)
	lp := &buffer.Line{Data: b, CacheValid: false}
	return lp
}

func TestSyntaxLineComment(t *testing.T) {
	old := PackagePalette.CommentStyle
	defer func() { PackagePalette.CommentStyle = old }()
	PackagePalette.CommentStyle = buffer.MakeTextStyle(buffer.TermColorRed, buffer.TermColorDefault, 0)

	lp := makeLine("  // hello world")
	lp.LangMode = buffer.LModeGo
	SyntaxEnsureLine(lp)
	if lp.SyntaxStyles == nil {
		t.Fatalf("SyntaxStyles nil")
	}
	// find first slash
	first := -1
	for i, r := range lp.RuneCache {
		if r == '/' {
			first = i
			break
		}
	}
	if first < 0 {
		t.Fatalf("could not find '//' in rune cache")
	}
	// all runes from first to end should be comment style
	for i := first; i < len(lp.RuneCache); i++ {
		if lp.SyntaxStyles[i] != PackagePalette.CommentStyle {
			t.Fatalf("expected comment style at %d got %v", i, lp.SyntaxStyles[i])
		}
	}
}

func TestSyntaxString(t *testing.T) {
	lp := makeLine("\"hello\" world")
	lp.LangMode = buffer.LModeGo
	SyntaxEnsureLine(lp)
	if lp.SyntaxStyles == nil {
		t.Fatalf("SyntaxStyles nil")
	}
	if len(lp.RuneCache) < 7 {
		t.Fatalf("unexpected rune cache length: %d", len(lp.RuneCache))
	}
	strStyle := buffer.MakeTextStyle(buffer.TermColorCyan, buffer.TermColorDefault, 0)
	// Opening quote, contents, and closing quote are all A_STRING.
	for i := 0; i < 7; i++ {
		if lp.SyntaxStyles[i] != strStyle {
			t.Fatalf("expected string style at %d got %v", i, lp.SyntaxStyles[i])
		}
	}
}

func TestSyntaxSingleQuotedString(t *testing.T) {
	lp := makeLine("'hello' world")
	lp.LangMode = buffer.LModeGo
	SyntaxEnsureLine(lp)
	if lp.SyntaxStyles == nil {
		t.Fatalf("SyntaxStyles nil")
	}
	if len(lp.RuneCache) < 7 {
		t.Fatalf("unexpected rune cache length: %d", len(lp.RuneCache))
	}
	strStyle := buffer.MakeTextStyle(buffer.TermColorCyan, buffer.TermColorDefault, 0)
	for i := 0; i < 7; i++ {
		if lp.SyntaxStyles[i] != strStyle {
			t.Fatalf("expected string style at %d got %v", i, lp.SyntaxStyles[i])
		}
	}
}

func TestDelimiterSummary(t *testing.T) {
	lp := makeLine("( )")
	lp.LangMode = buffer.LModeGo
	SyntaxEnsureLine(lp)
	if lp.SyntaxSummary.OpenOffsets[0] != 0 {
		t.Fatalf("expected open offset 0 got %d", lp.SyntaxSummary.OpenOffsets[0])
	}
	if lp.SyntaxSummary.CloseOffsets[0] == ^uint(0) {
		t.Fatalf("expected close offset set, got sentinel")
	}
}

func makeBufferFromLines(lines []string) *buffer.Buffer {
	bp := &buffer.Buffer{LangMode: buffer.LModeGo}
	for _, l := range lines {
		b := []byte(l)
		line := buffer.Line{Data: b, CacheValid: false, LangMode: buffer.LModeGo, Buffer: bp}
		bp.Lines = append(bp.Lines, line)
	}
	bp.LineCount = uint(len(bp.Lines))
	return bp
}

func TestRainbowParensIncremental(t *testing.T) {
	bp := makeBufferFromLines([]string{"(", "("})
	// ensure rune caches
	for i := 1; i <= int(bp.LineCount); i++ {
		lp := bp.Line(uint(i))
		lp.EnsureCache()
	}
	// incremental reparse from first line
	IncrementalReparse(bp, 1)
	// check styles
	first := bp.Line(1)
	second := bp.Line(2)
	if first == nil || second == nil {
		t.Fatalf("buffer lines missing")
	}
	if len(first.SyntaxStyles) == 0 || len(second.SyntaxStyles) == 0 {
		t.Fatalf("syntax styles not produced")
	}
	style0 := parenStyleExported(buffer.TermColorMagenta, 0)
	style1 := parenStyleExported(buffer.TermColorMagenta, 1)
	if first.SyntaxStyles[0] != style0 {
		t.Fatalf("expected depth0 style on first line, got %v", first.SyntaxStyles[0])
	}
	if second.SyntaxStyles[0] != style1 {
		t.Fatalf("expected depth1 style on second line, got %v", second.SyntaxStyles[0])
	}
}

func TestRainbowParensSingleLine(t *testing.T) {
	lp := makeLine("((()))")
	lp.LangMode = buffer.LModeGo
	lp.EnsureCache()
	// Tokenize the single line
	syn, summary, styles := tokenizeLineFromStateExported(lp, buffer.SynState{DFA: SynStateNormal})
	_ = syn
	_ = summary
	if styles == nil || len(styles) == 0 {
		t.Fatalf("no styles produced")
	}
	// expected colors: positions 0..5 -> depth 0,1,2,2,1,0
	expectedDepths := []int{0, 1, 2, 2, 1, 0}
	for i, d := range expectedDepths {
		expStyle := parenStyleExported(buffer.TermColorMagenta, d)
		if styles[i] != expStyle {
			t.Fatalf("paren at %d: expected depth %d style %v, got %v", i, d, expStyle, styles[i])
		}
	}
}

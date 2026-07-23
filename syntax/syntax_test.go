package syntax

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func makeLine(s string) *buffer.Line {
	b := []byte(s)
	line := &buffer.Line{Data: b, CacheValid: false}
	return line
}

func TestSyntaxLineComment(t *testing.T) {
	old := PackagePalette.CommentStyle
	defer func() { PackagePalette.CommentStyle = old }()
	PackagePalette.CommentStyle = buffer.MakeTextStyle(buffer.TermColorRed, buffer.TermColorDefault, 0)

	line := makeLine("  // hello world")
	line.LangMode = buffer.LModeGo
	SyntaxEnsureLine(line)
	if line.SyntaxStyles == nil {
		t.Fatalf("SyntaxStyles nil")
	}
	// find first slash
	first := -1
	for i, r := range line.RuneCache {
		if r == '/' {
			first = i
			break
		}
	}
	if first < 0 {
		t.Fatalf("could not find '//' in rune cache")
	}
	// all runes from first to end should be comment style
	for i := first; i < len(line.RuneCache); i++ {
		if line.SyntaxStyles[i] != PackagePalette.CommentStyle {
			t.Fatalf("expected comment style at %d got %v", i, line.SyntaxStyles[i])
		}
	}
}

func TestSyntaxString(t *testing.T) {
	line := makeLine("\"hello\" world")
	line.LangMode = buffer.LModeGo
	SyntaxEnsureLine(line)
	if line.SyntaxStyles == nil {
		t.Fatalf("SyntaxStyles nil")
	}
	if len(line.RuneCache) < 7 {
		t.Fatalf("unexpected rune cache length: %d", len(line.RuneCache))
	}
	strStyle := buffer.MakeTextStyle(buffer.TermColorCyan, buffer.TermColorDefault, 0)
	// Opening quote, contents, and closing quote are all A_STRING.
	for i := 0; i < 7; i++ {
		if line.SyntaxStyles[i] != strStyle {
			t.Fatalf("expected string style at %d got %v", i, line.SyntaxStyles[i])
		}
	}
}

func TestSyntaxSingleQuotedString(t *testing.T) {
	line := makeLine("'hello' world")
	line.LangMode = buffer.LModeGo
	SyntaxEnsureLine(line)
	if line.SyntaxStyles == nil {
		t.Fatalf("SyntaxStyles nil")
	}
	if len(line.RuneCache) < 7 {
		t.Fatalf("unexpected rune cache length: %d", len(line.RuneCache))
	}
	strStyle := buffer.MakeTextStyle(buffer.TermColorCyan, buffer.TermColorDefault, 0)
	for i := 0; i < 7; i++ {
		if line.SyntaxStyles[i] != strStyle {
			t.Fatalf("expected string style at %d got %v", i, line.SyntaxStyles[i])
		}
	}
}

func TestDelimiterSummary(t *testing.T) {
	line := makeLine("( )")
	line.LangMode = buffer.LModeGo
	SyntaxEnsureLine(line)
	if line.SyntaxSummary.OpenOffsets[0] != 0 {
		t.Fatalf("expected open offset 0 got %d", line.SyntaxSummary.OpenOffsets[0])
	}
	if line.SyntaxSummary.CloseOffsets[0] == buffer.OffsetUnset {
		t.Fatalf("expected close offset set, got sentinel")
	}
}

func makeBufferFromLines(lines []string) *buffer.Buffer {
	buf := &buffer.Buffer{LangMode: buffer.LModeGo}
	for _, l := range lines {
		b := []byte(l)
		line := buffer.Line{Data: b, CacheValid: false, LangMode: buffer.LModeGo, Buffer: buf}
		buf.Lines = append(buf.Lines, line)
	}
	return buf
}

func TestRainbowParensIncremental(t *testing.T) {
	buf := makeBufferFromLines([]string{"(", "("})
	// ensure rune caches
	for i := 1; i <= len(buf.Lines); i++ {
		line := buf.Line(i)
		line.EnsureCache()
	}
	// incremental reparse from first line
	IncrementalReparse(buf, 1)
	// check styles
	first := buf.Line(1)
	second := buf.Line(2)
	if first == nil || second == nil {
		t.Fatalf("buffer lines missing")
	}
	if len(first.SyntaxStyles) == 0 || len(second.SyntaxStyles) == 0 {
		t.Fatalf("syntax styles not produced")
	}
	style0 := parenStyle(buffer.TermColorMagenta, 0)
	style1 := parenStyle(buffer.TermColorMagenta, 1)
	if first.SyntaxStyles[0] != style0 {
		t.Fatalf("expected depth0 style on first line, got %v", first.SyntaxStyles[0])
	}
	if second.SyntaxStyles[0] != style1 {
		t.Fatalf("expected depth1 style on second line, got %v", second.SyntaxStyles[0])
	}
}

func TestRainbowParensSingleLine(t *testing.T) {
	line := makeLine("((()))")
	line.LangMode = buffer.LModeGo
	line.EnsureCache()
	// Tokenize the single line
	syn, summary, styles := tokenizeLineFromState(line, buffer.SynState{DFA: SynStateNormal})
	_ = syn
	_ = summary
	if styles == nil || len(styles) == 0 {
		t.Fatalf("no styles produced")
	}
	// expected colors: positions 0..5 -> depth 0,1,2,2,1,0
	expectedDepths := []int{0, 1, 2, 2, 1, 0}
	for i, d := range expectedDepths {
		expStyle := parenStyle(buffer.TermColorMagenta, d)
		if styles[i] != expStyle {
			t.Fatalf("paren at %d: expected depth %d style %v, got %v", i, d, expStyle, styles[i])
		}
	}
}

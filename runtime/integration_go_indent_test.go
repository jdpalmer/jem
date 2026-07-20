package runtime

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/mode"
	"github.com/jdpalmer/jem/term"
)

func TestGoNewlineIndentUsesTab(t *testing.T) {
	te := NewTestEditor(t)
	te.SetLangMode(buffer.LModeGo)

	te.LoadText("func main() {")
	_ = te.NewlineIndent()
	lp := te.BP().Line(te.Cursor().Line)
	if lp == nil || len(lp.Data) == 0 || lp.Data[0] != '\t' {
		t.Fatalf("after open brace indent = %q, want leading tab", string(lp.Data))
	}
	if lp.IndentColumn() != 8 {
		t.Fatalf("IndentColumn = %d, want 8", lp.IndentColumn())
	}
}

func TestGoIndentLineInsideBlock(t *testing.T) {
	te := NewTestEditor(t)
	te.SetLangMode(buffer.LModeGo)

	te.LoadText("func main() {\nx := 1")
	te.SetCursor(2, 0)
	if !mode.CmdModeIndentLine(false, 1) {
		t.Fatal("indent-line failed")
	}
	lp := te.BP().Line(2)
	if lp == nil || string(lp.Data) != "\tx := 1" {
		t.Fatalf("line = %q, want %q", string(lp.Data), "\tx := 1")
	}
}

func TestGoCloseBraceAligns(t *testing.T) {
	te := NewTestEditor(t)
	te.SetLangMode(buffer.LModeGo)

	te.LoadText("func main() {\n\tx := 1\n")
	te.SetCursor(3, 0)
	if !mode.CmdModeCloseBrace(false, 1) {
		t.Fatal("close-brace failed")
	}
	lp := te.BP().Line(3)
	if lp == nil || string(lp.Data) != "}" {
		t.Fatalf("close brace line = %q, want %q", string(lp.Data), "}")
	}
}

// Nested blocks: Tab on '}' must match the opening line indent, not the '{' column
// and not an outer brace (regression from findClosingDelimiterIndent depth/align bugs).
func TestGoTabAlignsNestedCloseBraces(t *testing.T) {
	te := NewTestEditor(t)
	te.SetLangMode(buffer.LModeGo)

	src := "" +
		"func LangModeInfo(mode buffer.LangMode) *mode.ModeInfo {\n" +
		"\tfor i := range modeTable {\n" +
		"\t\tif modeTable[i].Mode == mode {\n" +
		"\t\t\treturn &modeTable[i]\n" +
		"}\n" +
		"}\n" +
		"\treturn &modeTable[0]\n" +
		"}\n"
	te.LoadText(src)

	cases := []struct {
		line uint
		want string
	}{
		{5, "\t\t}"},
		{6, "\t}"},
		{8, "}"},
	}
	for _, tc := range cases {
		te.SetCursor(tc.line, 0)
		if !mode.CmdModeIndentLine(false, 1) {
			t.Fatalf("indent line %d failed", tc.line)
		}
		got := string(te.BP().Line(tc.line).Data)
		if got != tc.want {
			t.Fatalf("line %d after Tab = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestGoTabDoesNotCollapseToColumnZero(t *testing.T) {
	te := NewTestEditor(t)
	te.SetLangMode(buffer.LModeGo)

	te.LoadText("func main() {\n\tx := 1")
	te.SetCursor(2, 1) // on 'x'
	if !te.Key(term.KeyTab) {
		t.Fatal("Tab failed")
	}
	lp := te.BP().Line(2)
	if lp == nil || string(lp.Data) != "\tx := 1" {
		t.Fatalf("after Tab = %q, want still tab-indented", string(lp.Data))
	}
}

func TestCIndentStillUsesSpacesAfterDefaults(t *testing.T) {
	te := NewTestEditor(t)
	te.SetLangMode(buffer.LModeC)
	// OnBufferCreate set CIndent=2; SetLangMode must not wipe it for C.
	if te.BP().CIndent != 2 {
		t.Fatalf("C CIndent = %d, want 2", te.BP().CIndent)
	}

	te.LoadText("if (x) {")
	off := te.NewlineIndent()
	if off != 2 {
		t.Fatalf("C indent offset = %d, want 2 spaces", off)
	}
	lp := te.BP().Line(te.Cursor().Line)
	if string(lp.Data) != "  " {
		t.Fatalf("C indent line = %q, want two spaces", string(lp.Data))
	}
}

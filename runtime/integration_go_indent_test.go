package runtime

import (
	"strings"
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
	line := te.BP().Line(te.Cursor().Line)
	if line == nil || len(line.Data) == 0 || line.Data[0] != '\t' {
		t.Fatalf("after open brace indent = %q, want leading tab", string(line.Data))
	}
	if line.IndentColumn() != 8 {
		t.Fatalf("IndentColumn = %d, want 8", line.IndentColumn())
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
	line := te.BP().Line(2)
	if line == nil || string(line.Data) != "\tx := 1" {
		t.Fatalf("line = %q, want %q", string(line.Data), "\tx := 1")
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
	line := te.BP().Line(3)
	if line == nil || string(line.Data) != "}" {
		t.Fatalf("close brace line = %q, want %q", string(line.Data), "}")
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
		line int
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
	line := te.BP().Line(2)
	if line == nil || string(line.Data) != "\tx := 1" {
		t.Fatalf("after Tab = %q, want still tab-indented", string(line.Data))
	}
}

func TestCIndentStillUsesSpacesAfterDefaults(t *testing.T) {
	te := NewTestEditor(t)
	te.SetLangMode(buffer.LModeC)
	// Create defaults set Width=2; SetLangMode(C) keeps the C default of 2.
	if te.BP().Indent.Width != 2 {
		t.Fatalf("C Indent.Width = %d, want 2", te.BP().Indent.Width)
	}

	te.LoadText("if (x) {")
	off := te.NewlineIndent()
	if off != 2 {
		t.Fatalf("C indent offset = %d, want 2 spaces", off)
	}
	line := te.BP().Line(te.Cursor().Line)
	if string(line.Data) != "  " {
		t.Fatalf("C indent line = %q, want two spaces", string(line.Data))
	}
}

// Char literals like '(' / ')' must not drive continuation indent (commands_registry.go).
func TestGoIndentIgnoresParensInCharLiterals(t *testing.T) {
	te := NewTestEditor(t)
	te.SetLangMode(buffer.LModeGo)
	src := "" +
		"func InitCommands() {\n" +
		"\tcommandTable = []Command{\n" +
		"\t\t{Name: \"macro_end\", Keys: []uint32{term.CTLX | ')'}},\n" +
		"\t\t{Name: \"macro_start\", Keys: []uint32{term.CTLX | '('}},\n" +
		"\t\t{Name: \"mark_pop\", Keys: []uint32{term.CTLX | term.CTL | ' '}},\n" +
		"\t}\n" +
		"}\n"
	te.LoadText(src)
	te.SetCursor(5, 2)
	if !te.Key(term.KeyTab) {
		t.Fatal("Tab failed")
	}
	line := te.BP().Line(5)
	got := string(line.Data)
	if !strings.HasPrefix(got, "\t\t{Name:") {
		t.Fatalf("after Tab = %q (col %d); want two-tab composite-literal indent", got, line.IndentColumn())
	}
}

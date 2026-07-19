package mode_test

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/mode"
)

func TestIndentBytesForCol(t *testing.T) {
	tests := []struct {
		col  int
		want string
	}{
		{0, ""},
		{8, "\t"},
		{16, "\t\t"},
		{10, "\t  "},
		{3, "   "},
	}
	for _, tc := range tests {
		got := string(mode.IndentBytesForColForTest(tc.col))
		if got != tc.want {
			t.Fatalf("col %d: got %q, want %q", tc.col, got, tc.want)
		}
	}
}

func TestApplyLangIndentDefaultsGo(t *testing.T) {
	bp := buffer.New()
	bp.CIndent = 2
	bp.LangMode = buffer.LModeGo
	mode.ApplyLangIndentDefaults(bp)
	if bp.CIndent != 8 {
		t.Fatalf("Go CIndent = %d, want 8", bp.CIndent)
	}
}

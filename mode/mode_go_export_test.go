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

func TestEditorlyLangIndentDefaultsGo(t *testing.T) {
	buf := buffer.New()
	buf.Indent.Width = 2
	buf.LangMode = buffer.LModeGo
	mode.ApplyLangIndentDefaults(buf)
	if buf.Indent.Width != 8 {
		t.Fatalf("Go Indent.Width = %d, want 8", buf.Indent.Width)
	}
}

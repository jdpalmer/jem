package editor

import (
	"testing"

	"github.com/jdpalmer/jem/model"
)

func TestParseNumericText(t *testing.T) {
	tests := []struct {
		in   string
		want uint32
		ok   bool
	}{
		{"72", 72, true},
		{"0x10", 16, true},
		{"0X0a", 10, true},
		{"", 0, false},
		{"12abc", 0, false},
		{"-1", 0, false},
	}
	for _, tc := range tests {
		got, ok := parseNumericText(tc.in)
		if ok != tc.ok || (tc.ok && got != tc.want) {
			t.Fatalf("parseNumericText(%q) = (%d, %v), want (%d, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestVarsInitDefaults(t *testing.T) {
	VarsInit()
	if model.State.FillCol != 80 {
		t.Fatalf("FillCol = %d, want 80", model.State.FillCol)
	}
	if model.State.CIndent != 2 {
		t.Fatalf("CIndent = %d, want 2", model.State.CIndent)
	}
	if model.State.StartupQuote != true {
		t.Fatal("StartupQuote should default to true")
	}
}

func TestVarSetFromJSON(t *testing.T) {
	VarsInit()
	v := varFindByName("fill-column")
	if v == nil {
		t.Fatal("fill-column not found")
	}
	if !varSetFromJSON(v, []byte("100")) {
		t.Fatal("expected numeric JSON set to succeed")
	}
	if model.State.FillCol != 100 {
		t.Fatalf("FillCol = %d, want 100", model.State.FillCol)
	}
}

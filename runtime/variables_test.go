package runtime

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"testing"
)

func TestParseNumericText(t *testing.T) {
	tests := []struct {
		in   string
		want int
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
	if display.Active.FillCol != 80 {
		t.Fatalf("FillCol = %d, want 80", display.Active.FillCol)
	}
	if State.Indent.Width != 2 {
		t.Fatalf("Indent.Width = %d, want 2", State.Indent.Width)
	}
}

func TestBufferCreateAppliesIndentDefaults(t *testing.T) {
	_ = NewTestEditor(t)
	buf := buffer.All.Current
	if buf == nil {
		t.Fatal("no buffer")
	}
	if buf.Indent.Width != 2 {
		t.Fatalf("buffer Indent.Width = %d, want 2 (OnBufferCreate)", buf.Indent.Width)
	}
	if buf.Indent.Continued != 4 {
		t.Fatalf("buffer Indent.Continued = %d, want 4", buf.Indent.Continued)
	}
	if buf.FillCol != 80 {
		t.Fatalf("buffer FillCol = %d, want 80", buf.FillCol)
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
	if display.Active.FillCol != 100 {
		t.Fatalf("FillCol = %d, want 100", display.Active.FillCol)
	}
}

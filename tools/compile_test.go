package tools

import (
	"github.com/jdpalmer/jem/window"
	"strings"
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestCompileParseColonDiag(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantNil  bool
		path     string
		lineNum  int
		column   int
		severity CompileDiagSeverity
	}{
		{
			name:     "with column",
			line:     "main.go:10:5: error: undefined: foo",
			path:     "main.go",
			lineNum:  10,
			column:   5,
			severity: CompileDiagError,
		},
		{
			name:     "no column",
			line:     "src/util.go:42: note: unused",
			lineNum:  42,
			column:   1,
			severity: CompileDiagNote,
		},
		{
			name:    "invalid",
			line:    "not a diagnostic",
			wantNil: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := compileParseColonDiag(tc.line)
			if tc.wantNil {
				if d != nil {
					t.Fatal("expected nil for non-diagnostic line")
				}
				return
			}
			if d == nil {
				t.Fatal("expected diagnostic")
			}
			if tc.path != "" && d.Path != tc.path {
				t.Fatalf("path = %q, want %q", d.Path, tc.path)
			}
			if d.Line != tc.lineNum || d.Column != tc.column {
				t.Fatalf("got line=%d col=%d, want line=%d col=%d", d.Line, d.Column, tc.lineNum, tc.column)
			}
			if d.Severity != tc.severity {
				t.Fatalf("severity = %d, want %d", d.Severity, tc.severity)
			}
		})
	}
}

func TestCompileFillBuffer(t *testing.T) {
	buf := buffer.New()
	win := &window.Window{Buffer: buf}
	window.Active.CurrentWindow = win
	buffer.All.Current = buf
	counts, ok := compileFillBuffer(buf, "make -k",
		"main.go:2:3: error: boom\n",
		"warning: something\n",
		1, false, false)
	if !ok {
		t.Fatal("compileFillBuffer failed")
	}
	if counts.diag < 1 {
		t.Fatalf("diag count = %d", counts.diag)
	}
	summary := buf.Line(1)
	if summary == nil || !strings.Contains(string(summary.Data), "compile exit=1") {
		t.Fatalf("summary = %q", summary.Data)
	}
}

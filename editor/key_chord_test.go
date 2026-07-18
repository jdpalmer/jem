package editor

import (
	"github.com/jdpalmer/jem/term"
	"testing"
)

func TestApplyCtlxPrefix(t *testing.T) {
	KeybindingsInit()

	if got := applyCtlxPrefix('b'); got != term.CTLX|'B' {
		t.Fatalf("C-x b: got 0x%x, want CTLX|B (0x%x)", got, term.CTLX|'B')
	}
	if got := applyCtlxPrefix(term.CTL | 'B'); got != term.CTLX|'B' {
		t.Fatalf("C-x C-b (ctrl held): got 0x%x, want CTLX|B (0x%x)", got, term.CTLX|'B')
	}
	if got := applyCtlxPrefix(term.CTL | 'C'); got != term.CTLX|term.CTL|'C' {
		t.Fatalf("C-x C-c: got 0x%x, want CTLX|CTL|C (0x%x)", got, term.CTLX|term.CTL|'C')
	}
	if got := applyCtlxPrefix(term.CTL | 'F'); got != term.CTLX|term.CTL|'F' {
		t.Fatalf("C-x C-f: got 0x%x, want CTLX|CTL|F (0x%x)", got, term.CTLX|term.CTL|'F')
	}
}

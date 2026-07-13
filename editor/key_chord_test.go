package editor

import "testing"

func TestApplyCtlxPrefix(t *testing.T) {
	KeybindingsInit()

	if got := applyCtlxPrefix('b'); got != CTLX|'B' {
		t.Fatalf("C-x b: got 0x%x, want CTLX|B (0x%x)", got, CTLX|'B')
	}
	if got := applyCtlxPrefix(CTL | 'B'); got != CTLX|'B' {
		t.Fatalf("C-x C-b (ctrl held): got 0x%x, want CTLX|B (0x%x)", got, CTLX|'B')
	}
	if got := applyCtlxPrefix(CTL | 'C'); got != CTLX|CTL|'C' {
		t.Fatalf("C-x C-c: got 0x%x, want CTLX|CTL|C (0x%x)", got, CTLX|CTL|'C')
	}
	if got := applyCtlxPrefix(CTL | 'F'); got != CTLX|CTL|'F' {
		t.Fatalf("C-x C-f: got 0x%x, want CTLX|CTL|F (0x%x)", got, CTLX|CTL|'F')
	}
}

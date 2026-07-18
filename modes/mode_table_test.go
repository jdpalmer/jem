package modes

import (
	"testing"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/syntax"
)

func TestModeTableMatchesSyntaxSpec(t *testing.T) {
	for _, info := range modeTable {
		gotKind, gotFlags := syntax.LangModeSpecForTest(info.Mode)
		if int(gotKind) != int(info.SyntaxKind) {
			t.Errorf("mode %v: syntax kind = %d, want %d", info.Mode, gotKind, info.SyntaxKind)
		}
		if gotFlags != info.SyntaxFlags {
			t.Errorf("mode %v: syntax flags = %#x, want %#x", info.Mode, gotFlags, info.SyntaxFlags)
		}
	}
}

func TestModeTableUnknownModeFallback(t *testing.T) {
	const unknown = app.LangMode(255)
	gotKind, gotFlags := syntax.LangModeSpecForTest(unknown)
	wantKind := syntax.ModeSyntaxNone
	wantFlags := syntax.ModeFlagCommentSlashLine | syntax.ModeFlagCommentSlashBlock
	if gotKind != wantKind {
		t.Fatalf("unknown mode kind = %d, want %d", gotKind, wantKind)
	}
	if gotFlags != wantFlags {
		t.Fatalf("unknown mode flags = %#x, want %#x", gotFlags, wantFlags)
	}
}

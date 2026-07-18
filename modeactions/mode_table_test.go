package modeactions

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/modesyntax"
)

func TestModeInfoSpecCoversModeTable(t *testing.T) {
	for _, info := range modeTable {
		s := modesyntax.For(info.Mode)
		if info.SyntaxKind != s.Kind || info.SyntaxFlags != s.Flags {
			t.Errorf("mode %v: table kind/flags = %d/%#x, modesyntax.For = %d/%#x",
				info.Mode, info.SyntaxKind, info.SyntaxFlags, s.Kind, s.Flags)
		}
	}
}

func TestModeInfoUnknownModeFallback(t *testing.T) {
	const unknown = buffer.LangMode(255)
	s := modesyntax.For(unknown)
	if s.Kind != modesyntax.ModeSyntaxNone {
		t.Fatalf("unknown mode kind = %d, want %d", s.Kind, modesyntax.ModeSyntaxNone)
	}
	wantFlags := modesyntax.ModeFlagCommentSlashLine | modesyntax.ModeFlagCommentSlashBlock
	if s.Flags != wantFlags {
		t.Fatalf("unknown mode flags = %#x, want %#x", s.Flags, wantFlags)
	}
}

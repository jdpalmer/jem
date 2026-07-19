package mode

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/syntax"
)

func TestModeInfoSpecCoversModeTable(t *testing.T) {
	for _, info := range modeTable {
		s := syntax.For(info.Mode)
		if info.SyntaxKind != s.Kind || info.SyntaxFlags != s.Flags {
			t.Errorf("mode %v: table kind/flags = %d/%#x, syntax.For = %d/%#x",
				info.Mode, info.SyntaxKind, info.SyntaxFlags, s.Kind, s.Flags)
		}
	}
}

func TestModeInfoUnknownModeFallback(t *testing.T) {
	const unknown = buffer.LangMode(255)
	s := syntax.For(unknown)
	if s.Kind != syntax.ModeSyntaxNone {
		t.Fatalf("unknown mode kind = %d, want %d", s.Kind, syntax.ModeSyntaxNone)
	}
	wantFlags := syntax.ModeFlagCommentSlashLine | syntax.ModeFlagCommentSlashBlock
	if s.Flags != wantFlags {
		t.Fatalf("unknown mode flags = %#x, want %#x", s.Flags, wantFlags)
	}
}

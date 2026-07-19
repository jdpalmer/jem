package syntax

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestForGo(t *testing.T) {
	s := For(buffer.LModeGo)
	if s.Kind != ModeSyntaxGeneral {
		t.Fatalf("kind = %d, want General", s.Kind)
	}
	want := ModeFlagCommentSlashLine | ModeFlagCommentSlashBlock
	if s.Flags != want {
		t.Fatalf("flags = %#x, want %#x", s.Flags, want)
	}
}

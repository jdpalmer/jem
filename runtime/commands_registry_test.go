package runtime

import (
	"strings"
	"testing"
)

func TestCommandRegistryDocs(t *testing.T) {
	InitCommands()
	for i := range commandTable {
		cmd := &commandTable[i]
		if cmd.Name == "" {
			continue
		}
		if cmd.Fn == nil {
			t.Fatalf("command %q has nil handler", cmd.Name)
		}
		if cmd.Doc == "" {
			t.Fatalf("command %q missing doc string", cmd.Name)
		}
	}
	if commandByName("undo") == nil {
		t.Fatal("undo command not registered")
	}
}

func TestCommandsMatchFormatterPadsDoc(t *testing.T) {
	InitCommands()
	names := []string{"abort", "command_palette"}
	ctx := newCommandFuzzyCtx(names)
	if ctx.width != len("command_palette") {
		t.Fatalf("width = %d, want %d", ctx.width, len("command_palette"))
	}
	out := make([]byte, 256)
	commandsMatchFormatter(out, len(out), 0, ctx)
	end := 0
	for end < len(out) && out[end] != 0 {
		end++
	}
	got := string(out[:end])
	wantPrefix := "abort" + strings.Repeat(" ", ctx.width-len("abort")) + "  "
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("formatter = %q, want prefix %q", got, wantPrefix)
	}
	if !strings.Contains(got, "Abort") {
		t.Fatalf("formatter = %q, want doc text", got)
	}
}

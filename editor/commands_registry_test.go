package editor

import "testing"

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

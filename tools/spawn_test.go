package tools

import (
	"github.com/jdpalmer/jem/minibuffer"
	"runtime"
	"testing"
)

func TestSpawnShellCommandBuilder(t *testing.T) {
	cmd := spawnRunCommand("echo hello")
	if cmd == nil {
		t.Fatal("spawnRunCommand returned nil")
	}
	if runtime.GOOS == "windows" && len(cmd.Args) < 3 {
		t.Fatalf("args = %v, want COMSPEC /C <cmd>", cmd.Args)
	}
	if runtime.GOOS != "windows" && len(cmd.Args) < 3 {
		t.Fatalf("args = %v, want /bin/sh -c <cmd>", cmd.Args)
	}
}

func TestRunSpawnCommandRejectsEmptyInput(t *testing.T) {
	if runSpawnAfterPrompt("", minibuffer.PromptResultYes) {
		t.Fatal("expected false for empty command")
	}
	if runSpawnAfterPrompt("echo", minibuffer.PromptResultAbort) {
		t.Fatal("expected false for abort")
	}
}

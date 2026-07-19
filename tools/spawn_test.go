package tools

import (
	"runtime"
	"testing"

	"github.com/jdpalmer/jem/model"
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
	if runSpawnAfterPrompt("", model.PromptResultYes) {
		t.Fatal("expected false for empty command")
	}
	if runSpawnAfterPrompt("echo", model.PromptResultAbort) {
		t.Fatal("expected false for abort")
	}
}

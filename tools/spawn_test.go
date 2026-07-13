package tools

import (
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
	wrote := false
	PackageHooks = Hooks{
		MBReadStringCap: func(prompt, initial string, capacity int) (string, PromptResult) {
			return "", PromptResultYes
		},
		MBWrite: func(format string, args ...interface{}) {
			wrote = true
		},
	}
	if RunSpawnCommand() {
		t.Fatal("expected false for empty command")
	}
	if !wrote {
		t.Fatal("expected minibuffer warning for empty command")
	}
}

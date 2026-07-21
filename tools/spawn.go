package tools

// Shell one-liner and interactive CLI spawn.

import (
	"fmt"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/jdpalmer/jem/term"
)

const CommandPromptCapacity = 256

func spawnPrintNotice(label, command string) {
	fmt.Fprint(os.Stdout, "\n[jem] ", label)
	if command != "" {
		fmt.Fprint(os.Stdout, ": ", command)
	}
	fmt.Fprint(os.Stdout, "\n[jem] Terminal handed to shell. Exit shell to return to jem.\n\n")
}

func spawnShellCommand() *exec.Cmd {
	if runtime.GOOS == "windows" {
		shell := os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
		return exec.Command(shell)
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return exec.Command(shell)
}

func spawnRunCommand(line string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		shell := os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
		return exec.Command(shell, "/C", line)
	}
	return exec.Command("/bin/sh", "-c", line)
}

func SpawnShell(command *string) int {
	var cmd *exec.Cmd
	if command == nil || *command == "" {
		cmd = spawnShellCommand()
	} else {
		cmd = spawnRunCommand(*command)
	}

	if !TermFreezeInput() {
		mbWrite("[spawn unavailable: input reader did not pause]")
		return -1
	}
	defer TermThawInput()

	term.DrainInput()

	hadTTY := term.IsTTY()

	term.Move(term.Rows(), 0)
	term.Flush()
	term.Close()

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if command == nil || *command == "" {
		spawnPrintNotice("Launching interactive shell", "")
	} else {
		spawnPrintNotice("Running shell command", *command)
	}

	err := spawnRunForeground(cmd)
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	display.Active.ScreenDirty = true

	if runtime.GOOS != "windows" {
		time.Sleep(2 * time.Second)
	}

	term.DrainInput()
	if hadTTY {
		term.Resume()
	} else {
		term.Open()
	}

	if command != nil && *command != "" {
		mbWrite("[End]")
		term.Flush()
		if hadTTY {
			for {
				var pauseKey uint32
				if !editorReadKey(&pauseKey) {
					break
				}
				if pauseKey == term.KeyEnter {
					break
				}
				if runtime.GOOS != "windows" && pauseKey == ' ' {
					break
				}
			}
		}
		mbClear()
	}

	for i := 0; i < int(len(window.Active.Windows)); i++ {
		if wp := window.Active.Windows[i]; wp != nil {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
	}
	windowRetile()
	return exitCode
}

// RunSpawnCLI hands the terminal to an interactive shell (M-!).
func RunSpawnCLI() bool {
	rc := SpawnShell(nil)
	if rc == 0 {
		mbWrite("[shell exited]")
	} else {
		mbWrite("[shell exit %d]", rc)
	}
	return rc != -1
}

// RunSpawnCommand runs a one-line shell command (C-x !).
func RunSpawnCommand() bool {
	prompt := "! "
	if runtime.GOOS == "windows" {
		prompt = "Command: "
	}
	askStringCap(prompt, "", CommandPromptCapacity, func(command string, pr minibuffer.PromptResult) {
		_ = runSpawnAfterPrompt(command, pr)
	})
	return true
}

// runSpawnAfterPrompt finishes C-x ! after the minibuffer prompt.
// Separated so tests can exercise empty/abort guards without interactive input.
func runSpawnAfterPrompt(command string, pr minibuffer.PromptResult) bool {
	if pr != minibuffer.PromptResultYes {
		return false
	}
	if command == "" {
		mbWrite("[empty command]")
		return false
	}
	mbHistoryAdd(command)

	if runtime.GOOS != "windows" {
		fmt.Fprint(os.Stdout, "\n")
	}

	rc := SpawnShell(&command)
	display.Active.ScreenDirty = true
	return rc != -1
}

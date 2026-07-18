package tools

// spawn.go — shell one-liner and interactive CLI (translation of src/cmd_spawn.c)

import (
	"fmt"
	"github.com/jdpalmer/jem/app"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/jdpalmer/jem/term"
)

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

	// Dispatching routes keys to GlobalMinibufKeyCh; clear it during spawn.
	app.State.Dispatching = false

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

	app.State.ScreenDirty = true

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
				if pauseKey == KeyEnter {
					break
				}
				if runtime.GOOS != "windows" && pauseKey == ' ' {
					break
				}
			}
		}
		mbClear()
	}

	for i := 0; i < int(app.State.WindowCount); i++ {
		if wp := app.State.WINDOWS[i]; wp != nil {
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
	command, pr := mbReadStringCap(prompt, "", CommandPromptCapacity)
	if pr != PromptResultYes {
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
	app.State.ScreenDirty = true
	return rc != -1
}

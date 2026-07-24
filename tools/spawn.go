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

func spawnComspec() string {
	if shell := os.Getenv("COMSPEC"); shell != "" {
		return shell
	}
	return "cmd.exe"
}

func spawnShellCommand() *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command(spawnComspec())
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return exec.Command(shell)
}

func spawnRunCommand(line string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command(spawnComspec(), "/C", line)
	}
	return exec.Command("/bin/sh", "-c", line)
}

func SpawnShell(command string) int {
	var cmd *exec.Cmd
	if command == "" {
		cmd = spawnShellCommand()
	} else {
		cmd = spawnRunCommand(command)
	}

	if !display.TermFreezeInput() {
		display.MBWrite("[spawn unavailable: input reader did not pause]")
		return -1
	}
	defer display.TermThawInput()

	term.DrainInput()

	hadTTY := term.IsTTY()

	term.Move(term.Rows(), 0)
	term.Flush()
	term.Close()

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if command == "" {
		spawnPrintNotice("Launching interactive shell", "")
	} else {
		spawnPrintNotice("Running shell command", command)
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

	if command != "" {
		display.MBWrite("[End]")
		term.Flush()
		if hadTTY {
			for {
				pauseKey, ok := display.ReadEditorKey()
				if !ok {
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
		display.MBClear()
	}

	for i := 0; i < len(window.Active.Windows); i++ {
		if win := window.Active.Windows[i]; win != nil {
			win.ShouldRedraw = true
			win.ShouldUpdateModeLine = true
		}
	}
	window.WindowRetile()
	return exitCode
}

// RunSpawnCLI hands the terminal to an interactive shell (M-!).
func RunSpawnCLI() bool {
	rc := SpawnShell("")
	if rc == 0 {
		display.MBWrite("[shell exited]")
	} else {
		display.MBWrite("[shell exit %d]", rc)
	}
	return rc != -1
}

// SpawnPrompt returns the minibuffer prompt for C-x !.
func SpawnPrompt() string {
	if runtime.GOOS == "windows" {
		return "Command: "
	}
	return "! "
}

// RunSpawnAfterPrompt finishes C-x ! after the minibuffer prompt.
func RunSpawnAfterPrompt(command string, pr minibuffer.PromptResult) bool {
	if pr != minibuffer.PromptResultYes {
		return false
	}
	if command == "" {
		display.MBWrite("[empty command]")
		return false
	}
	display.MBHistoryAdd(command)

	if runtime.GOOS != "windows" {
		fmt.Fprint(os.Stdout, "\n")
	}

	rc := SpawnShell(command)
	return rc != -1
}

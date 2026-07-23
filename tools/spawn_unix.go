//go:build !windows

package tools

import (
	"os"
	"os/exec"
	"syscall"
)

// clearNonblock undoes the O_NONBLOCK flag that Go's runtime poller sets on a
// file descriptor once SetReadDeadline has been called on it (e.g. by the
// background key reader in input.go). That flag is a file-status flag on the
// underlying open file description, so it survives fork+exec: if left set,
// the spawned shell inherits a non-blocking stdin, and its blocking reads
// return EAGAIN instead of waiting for keystrokes — which looks like the
// terminal freezing after M-!/C-x !. SetReadDeadline(time.Time{}) clears the
// Go-level deadline but does NOT revert this OS-level flag, so we must clear
// it explicitly before exec'ing the child.
func clearNonblock(f *os.File) {
	_ = syscall.SetNonblock(int(f.Fd()), false)
}

func spawnRunForeground(cmd *exec.Cmd) error {
	clearNonblock(os.Stdin)
	clearNonblock(os.Stdout)
	clearNonblock(os.Stderr)
	return cmd.Run()
}

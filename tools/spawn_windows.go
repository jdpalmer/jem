//go:build windows

package tools

import "os/exec"

func spawnRunForeground(cmd *exec.Cmd) error {
	return cmd.Run()
}

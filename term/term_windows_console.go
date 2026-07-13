//go:build windows

package term

import (
	"os"

	"golang.org/x/sys/windows"
)

var (
	winInHandle          windows.Handle
	winInModeDesired     uint32
	winConsoleConfigured bool
)

// termPlatformInitConsole applies the full editor console input mode on Windows.
// Windows Terminal may reset the mode after ANSI output; winInModeDesired is
// re-applied from termPlatformAfterFlush (see docs/LESSONS.md).
func termPlatformInitConsole() error {
	in := windows.Handle(termFd)
	var mode uint32
	if err := windows.GetConsoleMode(in, &mode); err != nil {
		return err
	}

	mode &^= windows.ENABLE_LINE_INPUT |
		windows.ENABLE_ECHO_INPUT |
		windows.ENABLE_PROCESSED_INPUT |
		windows.ENABLE_QUICK_EDIT_MODE
	mode |= windows.ENABLE_EXTENDED_FLAGS |
		windows.ENABLE_WINDOW_INPUT |
		windows.ENABLE_MOUSE_INPUT |
		windows.ENABLE_VIRTUAL_TERMINAL_INPUT

	if err := windows.SetConsoleMode(in, mode); err != nil {
		return err
	}

	winInHandle = in
	winInModeDesired = mode
	winConsoleConfigured = true

	out := windows.Handle(os.Stdout.Fd())
	var outMode uint32
	if err := windows.GetConsoleMode(out, &outMode); err == nil {
		_ = windows.SetConsoleMode(out, outMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
	return nil
}

// termPlatformAfterFlush re-applies the desired input mode if the terminal reset it.
func termPlatformAfterFlush() {
	if !winConsoleConfigured || winInModeDesired == 0 {
		return
	}
	var current uint32
	if err := windows.GetConsoleMode(winInHandle, &current); err != nil {
		return
	}
	if current != winInModeDesired {
		_ = windows.SetConsoleMode(winInHandle, winInModeDesired)
	}
}

func termPlatformCloseConsole() {
	winConsoleConfigured = false
	winInModeDesired = 0
	winInHandle = 0
}

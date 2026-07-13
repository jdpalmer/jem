//go:build windows

package term

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procPeekNamedPipe = windows.NewLazySystemDLL("kernel32.dll").NewProc("PeekNamedPipe")

// termWaitReadable reports whether fd has data available to read within the
// given timeout. Console handles poll GetNumberOfConsoleInputEvents; pipes
// use PeekNamedPipe (mirroring term_unix.go's poll(2) role).
func termWaitReadable(fd int, timeout time.Duration) bool {
	if fd < 0 {
		return false
	}
	handle := windows.Handle(fd)

	switch fileType(handle) {
	case windows.FILE_TYPE_CHAR:
		return waitConsoleReadable(handle, timeout)
	case windows.FILE_TYPE_PIPE:
		return waitPipeReadable(handle, timeout)
	default:
		// Redirected files and other handle types: let the subsequent read
		// decide (matches the prior always-true stub for non-console stdin).
		return true
	}
}

func fileType(handle windows.Handle) uint32 {
	ft, err := windows.GetFileType(handle)
	if err != nil {
		return windows.FILE_TYPE_UNKNOWN
	}
	return ft
}

func waitConsoleReadable(handle windows.Handle, timeout time.Duration) bool {
	h := handle
	if winConsoleConfigured && winInHandle != 0 {
		h = winInHandle
	}
	if consoleInputEvents(h) > 0 {
		return true
	}
	if timeout == 0 {
		return false
	}
	const pollInterval = 10 * time.Millisecond
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		if consoleInputEvents(h) > 0 {
			return true
		}
	}
	return false
}

func consoleInputEvents(handle windows.Handle) uint32 {
	var n uint32
	if err := windows.GetNumberOfConsoleInputEvents(handle, &n); err != nil {
		return 0
	}
	return n
}

func waitPipeReadable(handle windows.Handle, timeout time.Duration) bool {
	if pipeBytesAvailable(handle) {
		return true
	}
	if timeout == 0 {
		return false
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		sleep := time.Until(deadline)
		if sleep > time.Millisecond {
			sleep = time.Millisecond
		}
		time.Sleep(sleep)
		if pipeBytesAvailable(handle) {
			return true
		}
	}
	return false
}

func pipeBytesAvailable(handle windows.Handle) bool {
	var avail uint32
	err := peekNamedPipe(handle, &avail)
	if err != nil {
		// Write end closed: schedule a read so the caller observes EOF.
		return err == windows.ERROR_BROKEN_PIPE
	}
	return avail > 0
}

func peekNamedPipe(handle windows.Handle, avail *uint32) error {
	r, _, errno := procPeekNamedPipe.Call(
		uintptr(handle),
		0,
		0,
		0,
		uintptr(unsafe.Pointer(avail)),
		0,
	)
	if r == 0 {
		if errno != nil {
			if e, ok := errno.(windows.Errno); ok {
				return e
			}
		}
		return windows.ERROR_GEN_FAILURE
	}
	return nil
}

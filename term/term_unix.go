//go:build !windows

package term

import (
	"time"

	"golang.org/x/sys/unix"
)

// termWaitReadable reports whether fd has data available to read within the
// given timeout, using a real OS-level poll(2) instead of os.File's
// SetReadDeadline.
//
// os.Stdin is created by the Go runtime via NewFile with kind kindNewFile,
// which is only registered with the runtime netpoller (made "pollable") if
// the fd already has O_NONBLOCK set at startup. A terminal fd normally does
// not, so os.Stdin.SetReadDeadline silently returns internal/poll.ErrNoDeadline
// ("file type does not support deadline") on every call — every "timed" read
// in this codebase was actually a plain, unbounded blocking read. That is
// what caused M-!/C-x ! to hang: termDrainPendingInput's supposedly-10ms
// read would block forever if no further input happened to arrive. Polling
// the fd directly works regardless of netpoller registration.
func termWaitReadable(fd int, timeout time.Duration) bool {
	if fd < 0 {
		return false
	}
	ms := int(timeout / time.Millisecond)
	if timeout > 0 && ms == 0 {
		ms = 1
	}
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for {
		n, err := unix.Poll(fds, ms)
		if err == unix.EINTR {
			continue
		}
		if err != nil || n <= 0 {
			return false
		}
		return fds[0].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) != 0
	}
}

package term

// Terminal I/O and raw mode.

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jdpalmer/jem/buffer"
	xterm "golang.org/x/term"
)

var (
	termRows           int = 24
	termCols           int = 80
	termOut                = bufio.NewWriter(os.Stdout)
	termScrollTop      int = -1
	termScrollBottom   int = -1
	termAnsiStyle      buffer.TextStyle
	savedTermState     *xterm.State
	termFile           *os.File
	termFd             int
	termReader         *bufio.Reader
	termIsTTY          bool
	termAltbuf         bool
	termReadMu         sync.Mutex
	termKittySupported bool

	lastMouseCol int
	lastMouseRow int

	// debugLogs enables /tmp/jem-keys.log and /tmp/jem-display.log diagnostics.
	debugLogs bool = false
)

// termDrainPendingInput discards buffered stdin before handing the TTY to a shell.
//
// This used to set a 10ms os.File read deadline and then call a plain
// blocking Read, on the assumption that the deadline would cap the wait.
// That assumption is false for os.Stdin on Unix: os.Stdin is only
// registered with Go's runtime netpoller if its fd already has O_NONBLOCK
// set at process startup, which a terminal fd normally does not. So
// SetReadDeadline silently no-ops (returns internal/poll.ErrNoDeadline,
// which was being discarded), and the "deadline-bounded" Read below was
// actually an unbounded blocking read — hanging here forever whenever there
// was no more buffered input, which is exactly what made M-!/C-x ! freeze
// before the shell ever launched. termWaitReadable polls the fd directly
// (via poll(2) on Unix) so the timeout is real regardless of netpoller
// registration.
func DrainInput() {
	termReadMu.Lock()
	defer termReadMu.Unlock()
	for {
		if termReader != nil && termReader.Buffered() > 0 {
			_, _ = termReader.ReadByte()
			continue
		}
		if !termWaitReadable(termFd, 10*time.Millisecond) {
			break
		}
		var buf [256]byte
		n, err := termFile.Read(buf[:])
		if err != nil || n == 0 {
			break
		}
	}
	if termFile != nil {
		termReader = bufio.NewReader(termFile)
	}
}

func termEnableEditorModes() {
	termEmitLit("\x1b[?1049h")
	termEmitLit("\x1b[>1u")
	termEmitLit("\x1b[?2004h")
	termEmitLit("\x1b[?1000h")
	termEmitLit("\x1b[?1002h")
	termEmitLit("\x1b[?1006h")

	termScrollTop = -1
	termScrollBottom = -1

	Flush()
	termAltbuf = true
	termEmitLit("\x1b[?25h")
	termEmitLit("\x1b[?2q")
	Flush()
}

func termProbeKittyKeyboard() bool {
	_, err := os.Stdout.Write([]byte{0x1B, '[', '?', 'u'})
	if err != nil {
		return false
	}
	if b, ok := termReadByte(1000); ok && b == 0x1B {
		if b, ok := termReadByte(100); ok && b == '[' {
			if b, ok := termReadByte(100); ok && b == '?' {
				for {
					if b, ok := termReadByte(100); ok {
						if b >= '0' && b <= '9' {
							continue
						}
						return b == 'u'
					}
					break
				}
			}
		}
	}
	return false
}

func termEmitLit(s string) {
	if len(s) > 0 {
		termOut.WriteString(s)
	}
}

func termAppendU32(buf []byte, v uint32) int {
	if v == 0 {
		buf[0] = '0'
		return 1
	}
	var tmp [10]byte
	i := len(tmp)
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	return copy(buf, tmp[i:])
}

func termEmitCSI(first int, hasSecond bool, second int, final byte) {
	var seq [32]byte
	n := 0
	seq[n] = 0x1b
	n++
	seq[n] = '['
	n++
	n += termAppendU32(seq[n:], uint32(first))
	if hasSecond {
		seq[n] = ';'
		n++
		n += termAppendU32(seq[n:], uint32(second))
	}
	seq[n] = final
	n++
	termOut.Write(seq[:n])
}

func termReadByte(timeoutMs int) (byte, bool) {
	if timeoutMs > 0 {
		if !termWaitReadable(termFd, time.Duration(timeoutMs)*time.Millisecond) {
			return 0, false
		}
	}

	var buf [1]byte
	n, err := termFile.Read(buf[:])
	if err != nil {
		return 0, false
	}
	if n != 1 {
		return 0, false
	}
	return buf[0], true
}

// termReadByteFromReader reads one byte via termReader (the shared buffered
// reader), unlike termReadByte which reads termFile directly. It must be
// used once termReader is in use (post-Open) so buffered bytes aren't
// bypassed. Caller must hold termReadMu.
func termReadByteFromReader(timeoutMs int) (byte, bool) {
	if termReader.Buffered() == 0 && !termWaitReadable(termFd, time.Duration(timeoutMs)*time.Millisecond) {
		return 0, false
	}
	var buf [1]byte
	n, err := termReader.Read(buf[:])
	if err != nil || n != 1 {
		return 0, false
	}
	return buf[0], true
}

// PingRepaint sends a Device Status Report query and discards the reply.
//
// Some terminals (observed with iTerm2) defer painting a full-screen app's
// already-flushed output until another input round-trip occurs, so a
// bracketed-paste redraw can sit unpainted until the next keypress. Forcing
// a quick query/response round trip nudges those terminals to flush the
// pending paint. If the terminal doesn't reply promptly, this gives up
// without blocking the editor for long.
func PingRepaint() {
	termReadMu.Lock()
	defer termReadMu.Unlock()
	if !termIsTTY || termReader == nil {
		return
	}
	termOut.WriteString("\x1b[5n")
	termOut.Flush()

	b, ok := termReadByteFromReader(150)
	if !ok || b != 0x1B {
		return
	}
	if b, ok := termReadByteFromReader(50); !ok || b != '[' {
		return
	}
	for {
		b, ok := termReadByteFromReader(50)
		if !ok || b == 'n' {
			return
		}
	}
}

// Open initializes the terminal, enters raw mode, and probes for keyboard features.
func Open() {
	termFile = os.Stdin
	termFd = int(os.Stdin.Fd())

	if !xterm.IsTerminal(termFd) {
		fmt.Fprintln(os.Stderr, "jem: not a TTY; continuing with default size 24x80")
		termRows = 24
		termCols = 80
		termIsTTY = false
		termReader = bufio.NewReader(termFile)
		return
	}

	termIsTTY = true
	cols, rows, err := xterm.GetSize(termFd)
	if err != nil || rows < 2 || cols < 1 {
		fmt.Fprintln(os.Stderr, "jem: could not get terminal size; continuing with default size 24x80")
		termRows = 24
		termCols = 80
		termIsTTY = false
		return
	}

	termRows = rows - 1
	termCols = cols

	state, err := xterm.MakeRaw(termFd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jem: failed to set raw mode; continuing")
		termIsTTY = false
		return
	}
	savedTermState = state
	if err := termPlatformInitConsole(); err != nil {
		fmt.Fprintln(os.Stderr, "jem: failed to configure Windows console:", err)
	}

	if !termKittySupported {
		if !termProbeKittyKeyboard() {
			_ = xterm.Restore(termFd, savedTermState)
			savedTermState = nil
			fmt.Fprintln(os.Stderr, "jem: this terminal does not support the Kitty keyboard protocol.")
			fmt.Fprintln(os.Stderr, "jem: please use a supported terminal such as Kitty, WezTerm, Ghostty, foot, or Windows Terminal.")
			os.Exit(1)
		}
		termKittySupported = true
	}

	termReader = bufio.NewReader(termFile)
	termEnableEditorModes()
}

// Resume re-enters raw mode after a shell subjob without re-probing Kitty.
func Resume() {
	if !termIsTTY {
		return
	}
	cols, rows, err := xterm.GetSize(termFd)
	if err == nil && rows >= 2 && cols >= 1 {
		termRows = rows - 1
		termCols = cols
	}
	state, err := xterm.MakeRaw(termFd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jem: failed to restore raw mode after shell")
		return
	}
	savedTermState = state
	if err := termPlatformInitConsole(); err != nil {
		fmt.Fprintln(os.Stderr, "jem: failed to configure Windows console after shell:", err)
	}
	termFile = os.Stdin
	termFd = int(termFile.Fd())
	termReader = bufio.NewReader(termFile)
	termOut.Reset(os.Stdout)
	Flush()
	termEnableEditorModes()
	if PackageHooks.OnResume != nil {
		PackageHooks.OnResume()
	}
}

// Close restores the terminal to normal mode and exits raw mode.
func Close() {
	termEmitLit("\x1b[<u")
	termEmitLit("\x1b[?2004l")
	termEmitLit("\x1b[?1006l")
	termEmitLit("\x1b[?1002l")
	termEmitLit("\x1b[?1000l")
	ResetScrollRegion()
	if termAltbuf {
		termEmitLit("\x1b[?1049l")
		termAltbuf = false
	}
	Flush()

	termPlatformCloseConsole()
	if termIsTTY && savedTermState != nil {
		_ = xterm.Restore(termFd, savedTermState)
		savedTermState = nil
	}
	if termFile != nil && termFile != os.Stdin {
		termFile.Close()
		termFile = nil
	}
}

// RefreshSize reads the current TTY size and updates internal dimensions if changed.
func RefreshSize() bool {
	cols, rows, err := xterm.GetSize(termFd)
	if err != nil || rows < 2 || cols < 1 {
		return false
	}

	newRows := rows - 1
	newCols := cols

	if newRows == termRows && newCols == termCols {
		return false
	}

	termRows = newRows
	termCols = newCols
	termScrollTop = -1
	termScrollBottom = -1
	return true
}

// Move moves the cursor to the specified row and column.
func Move(row, col int) {
	termEmitCSI(row+1, true, col+1, 'H')
}

// EraseEol erases from the cursor to the end of the current line.
func EraseEol() {
	termEmitLit("\x1b[K")
}

// EraseEos resets the scroll region and erases from the cursor to the end of the screen.
func EraseEos() {
	ResetScrollRegion()
	termEmitLit("\x1b[J")
}

// ResetScrollRegion resets the scroll region to cover the full screen.
func ResetScrollRegion() {
	if termScrollTop == 0 && termScrollBottom == termRows {
		return
	}
	SetScrollRegion(0, termRows)
}

// SetScrollRegion sets the top and bottom rows of the scrollable area.
func SetScrollRegion(top, bottom int) {
	if termScrollTop == top && termScrollBottom == bottom {
		return
	}
	termEmitCSI(top+1, true, bottom+1, 'r')
	termScrollTop = top
	termScrollBottom = bottom
}

// Write writes raw bytes to the terminal output buffer.
func Write(bytes []byte) {
	if len(bytes) == 0 {
		return
	}
	termOut.Write(bytes)
}

// ScrollRows scrolls the specified region up (positive rowCount) or down (negative rowCount).
func ScrollRows(top, bottom int, rowCount int) {
	absRowCount := rowCount
	if absRowCount < 0 {
		absRowCount = -absRowCount
	}
	height := bottom - top + 1

	if absRowCount <= 0 || absRowCount >= height {
		return
	}

	SetScrollRegion(top, bottom)
	if rowCount > 0 {
		termEmitCSI(absRowCount, false, 0, 'S')
	} else {
		termEmitCSI(absRowCount, false, 0, 'T')
	}
}

// Beep emits a terminal bell character.
func Beep() {
	termOut.WriteByte(0x07)
	Flush()
}

// Flush writes the output buffer to the terminal.
func Flush() {
	termOut.Flush()
	termPlatformAfterFlush()
}

// MousePos returns the last reported mouse cell (col, row). For tests.
func MousePos() (col, row int) { return lastMouseCol, lastMouseRow }

// Rows returns the usable terminal height in cells.
// After Open or RefreshSize this is one less than the TTY row count (status line reserved).
func Rows() int { return termRows }

// Cols returns the terminal width in cells.
func Cols() int { return termCols }

// SetSize sets dimensions for headless use and tests.
// Unlike Open/RefreshSize, rows is used as-is (no status-line subtraction).
func SetSize(rows, cols int) {
	if rows < 1 {
		rows = 24
	}
	if cols < 1 {
		cols = 80
	}
	termRows = rows
	termCols = cols
}

// IsTTY reports whether the terminal was opened on a real TTY.
func IsTTY() bool { return termIsTTY }

// SetTestInput wires a pipe or other reader for tests that need termWaitReadable.
func SetTestInput(f *os.File) {
	termFile = f
	termFd = int(f.Fd())
	termReader = bufio.NewReader(f)
	termIsTTY = false
}

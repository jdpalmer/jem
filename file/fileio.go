package file

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/mode"
	"github.com/jdpalmer/jem/window"
)

var (
	ErrNoBuffer   = errors.New("no current buffer")
	ErrReadonly   = errors.New("read-only buffer")
	ErrNoFilename = errors.New("no filename")
)

// NoOpWriter is a writef that discards messages. Pass it when callers do not
// need load/save status output.
var NoOpWriter = func(string, ...any) {}

func writeMessage(writef func(string, ...any), format string, args ...any) {
	writef(format, args...)
}

// FileModTime returns the modification time of fname, or zero time on error.
func FileModTime(fname string) time.Time {
	if fname == "" {
		return time.Time{}
	}
	fi, err := os.Stat(fname)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

// BufferNameFromPath picks a unique buffer name from a filesystem path.
func BufferNameFromPath(fname string) string {
	base := filepath.Base(fname)
	if i := strings.IndexByte(base, ';'); i >= 0 {
		base = base[:i]
	}
	if buffer.Find(base) == nil {
		return base
	}
	for suffix := 2; ; suffix++ {
		name := fmt.Sprintf("%s:%d", base, suffix)
		if buffer.Find(name) == nil {
			return name
		}
	}
}

// LoadCommandLineFiles loads the first path as the active buffer and subsequent paths as additional buffers.
func LoadCommandLineFiles(paths []string, nameFromPath func(string) string, loadFile func(string) error) {
	if len(paths) == 0 {
		return
	}
	_ = loadFile(paths[0])

	for i := len(paths) - 1; i >= 1; i-- {
		path := paths[i]
		otherBuf := buffer.Create()
		if otherBuf == nil {
			continue
		}
		if nameFromPath != nil {
			otherBuf.Name = nameFromPath(path)
		}
		buffer.SetCurrent(otherBuf)
		_ = loadFile(path)
	}

	if cw := window.Active.CurrentWindow; cw != nil && cw.Buffer != nil {
		buffer.SetCurrent(cw.Buffer)
	}
}

// LoadCurrentBuffer reads fname into the current buffer, clearing existing contents.
func LoadCurrentBuffer(fname string, writef func(string, ...any)) error {
	resolved := NormalizePath(fname)
	buf := buffer.All.Current
	if buf == nil {
		return ErrNoBuffer
	}
	if buf.IsReadonly {
		writeMessage(writef, "[read-only buffer]")
		return ErrReadonly
	}

	buf.Clear()
	buf.IsChanged = false
	buf.EolMode = buffer.EModeLF
	buf.FileName = resolved
	buf.LangMode = DetectLangMode(resolved)
	mode.ApplyLangIndentDefaults(buf)

	fh, err := os.Open(resolved)
	if err != nil {
		// Missing path is a successful "new file" buffer, matching historical behavior.
		writeMessage(writef, "[New file]")
		buf.Cursor = buffer.Location{Line: 1, Offset: 0}
		buf.Mark = buffer.Location{Line: 0, Offset: 0}
		return nil
	}
	defer fh.Close()

	writeMessage(writef, "[Reading file]")
	buf.DiscardLines()
	reader := bufio.NewReader(fh)
	nline := 0
	eolMode := buffer.EModeLF

	var lineBuf bytes.Buffer
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				if lineBuf.Len() > 0 {
					buf.AppendLineBytes(lineBuf.Bytes())
					nline++
				}
				break
			}
			writeMessage(writef, "File read error")
			buf.Clear()
			return fmt.Errorf("file read: %w", err)
		}

		if b == '\r' {
			next, err := reader.Peek(1)
			if err == nil && next[0] == '\n' {
				_, _ = reader.ReadByte()
				if eolMode == buffer.EModeLF {
					eolMode = buffer.EModeCRLF
				}
			} else if eolMode == buffer.EModeLF {
				eolMode = buffer.EModeCR
			}
			buf.AppendLineBytes(lineBuf.Bytes())
			lineBuf.Reset()
			nline++
			continue
		}

		if b == '\n' {
			buf.AppendLineBytes(lineBuf.Bytes())
			lineBuf.Reset()
			nline++
			continue
		}

		lineBuf.WriteByte(b)
	}

	buf.EnsureMinLines()
	buf.EolMode = eolMode
	if nline == 1 {
		writeMessage(writef, "[Read 1 line]")
	} else {
		writeMessage(writef, "[Read lines]")
	}

	buf.Cursor = buffer.Location{Line: 1, Offset: 0}
	buf.Mark = buffer.Location{Line: 0, Offset: 0}

	if win := window.Active.CurrentWindow; win != nil && win.Buffer == buf {
		win.TopLine = 1
		win.Cursor = buffer.Location{Line: 1, Offset: 0}
		win.Mark = buffer.Location{Line: 0, Offset: 0}
		win.ShouldRedraw = true
		win.ShouldUpdateModeLine = true
	}

	buf.FileModTime = FileModTime(resolved)
	buf.NotifiedModTime = time.Time{}
	return nil
}

// NeedsOverwriteConfirm reports whether fn's on-disk mtime differs from the buffer's.
func NeedsOverwriteConfirm(fn string) bool {
	buf := buffer.All.Current
	if buf == nil || buf.FileModTime.IsZero() {
		return false
	}
	curModTime := FileModTime(fn)
	return !curModTime.IsZero() && !curModTime.Equal(buf.FileModTime)
}

// SaveCurrentBufferForce writes the buffer without the overwrite-mtime prompt.
func SaveCurrentBufferForce(fn string, writef func(string, ...any)) error {
	buf := buffer.All.Current
	if buf == nil {
		return ErrNoBuffer
	}

	if buf.WhitespaceCleanup {
		for i := 1; i <= len(buf.Lines); i++ {
			buf.TrimTrailingWhitespace(i)
		}
	}

	fh, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		writeMessage(writef, "[cannot open file for writing]")
		return fmt.Errorf("open for write: %w", err)
	}
	defer fh.Close()

	eol := []byte("\n")
	if buf.EolMode == buffer.EModeCRLF {
		eol = []byte("\r\n")
	} else if buf.EolMode == buffer.EModeCR {
		eol = []byte("\r")
	}

	writer := bufio.NewWriter(fh)
	nline := 0
	for i := 1; i <= len(buf.Lines); i++ {
		line := buf.Line(i)
		if line == nil {
			continue
		}
		if len(line.Data) > 0 {
			if _, err := writer.Write(line.Data); err != nil {
				writeMessage(writef, "Write I/O error")
				return fmt.Errorf("write: %w", err)
			}
		}
		if _, err := writer.Write(eol); err != nil {
			writeMessage(writef, "Write I/O error")
			return fmt.Errorf("write eol: %w", err)
		}
		nline++
	}

	if err := writer.Flush(); err != nil {
		writeMessage(writef, "Write I/O error")
		return fmt.Errorf("flush: %w", err)
	}

	if nline == 1 {
		writeMessage(writef, "[wrote 1 line]")
	} else {
		writeMessage(writef, "[wrote lines]")
	}

	buf.FileModTime = FileModTime(fn)
	buf.NotifiedModTime = time.Time{}
	buf.IsChanged = false
	return nil
}

// ReloadCurrentBufferFromDisk reloads fname into the current buffer and restores cursor position.
func ReloadCurrentBufferFromDisk(fname string, lineNumber int, noteBufferSaved func(*buffer.Buffer), writef func(string, ...any)) error {
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil {
		return ErrNoBuffer
	}
	if fname == "" {
		return ErrNoFilename
	}
	if err := LoadCurrentBuffer(fname, writef); err != nil {
		return err
	}
	if noteBufferSaved != nil {
		noteBufferSaved(buf)
	}
	buf.NotifiedModTime = time.Time{}
	if win != nil && lineNumber > 0 && lineNumber <= len(buf.Lines) {
		win.Cursor = buffer.MakeLocation(lineNumber, 0)
		win.ShouldRedraw = true
		win.ShouldUpdateModeLine = true
	}
	for _, w := range window.Active.Windows {
		if w != nil && w.Buffer == buf {
			w.ShouldRedraw = true
			w.ShouldUpdateModeLine = true
		}
	}
	return nil
}

// CheckReloadCurrentBuffer reloads the current buffer when its file changes on disk.
// askConfirm, when non-nil, is used for dirty-buffer revert prompts (async-friendly:
// callers may schedule work in onYes/onNo and return immediately).
func CheckReloadCurrentBuffer(askConfirm func(prompt string, onYes, onNo func()), writef func(string, ...any), noteBufferSaved func(*buffer.Buffer), dispatching, autoRevert bool) {
	if minibuffer.Active != nil || dispatching {
		return
	}
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || buf.IsReadonly {
		return
	}
	fname := buf.FileName
	if fname == "" || buf.FileModTime.IsZero() {
		return
	}

	cur := FileModTime(fname)
	if cur.IsZero() || cur.Equal(buf.FileModTime) {
		if !buf.NotifiedModTime.IsZero() {
			buf.NotifiedModTime = time.Time{}
			if win != nil {
				win.ShouldUpdateModeLine = true
			}
		}
		return
	}

	lineNumber := 1
	if win != nil {
		lineNumber = win.Cursor.Line
	}

	if buf.IsChanged {
		if autoRevert {
			_ = ReloadCurrentBufferFromDisk(fname, lineNumber, noteBufferSaved, writef)
			return
		}
		if cur.Equal(buf.NotifiedModTime) {
			return
		}
		buf.NotifiedModTime = cur
		if win != nil {
			win.ShouldUpdateModeLine = true
		}
		if askConfirm == nil {
			writeMessage(writef, "[keeping edited buffer]")
			return
		}
		askConfirm("File changed on disk; revert", func() {
			_ = ReloadCurrentBufferFromDisk(fname, lineNumber, noteBufferSaved, writef)
			writeMessage(writef, "[Reverted]")
		}, func() {
			writeMessage(writef, "[keeping edited buffer]")
		})
		return
	}

	_ = ReloadCurrentBufferFromDisk(fname, lineNumber, noteBufferSaved, writef)
}

// DetectLangMode returns the language mode inferred from the file extension of fname.
func DetectLangMode(fname string) buffer.LangMode {
	ext := strings.ToLower(filepath.Ext(fname))
	switch ext {
	case ".go":
		return buffer.LModeGo
	case ".py":
		return buffer.LModePython
	case ".c", ".h":
		return buffer.LModeC
	case ".java":
		return buffer.LModeJava
	case ".md", ".markdown":
		return buffer.LModeMarkdown
	case ".html", ".htm":
		return buffer.LModeHTML
	case ".css":
		return buffer.LModeCSS
	case ".js":
		return buffer.LModeJavaScript
	case ".ts":
		return buffer.LModeTypeScript
	case ".rs":
		return buffer.LModeRust
	default:
		return buffer.LModeNone
	}
}

package files

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/mode"
)

var (
	ErrNoBuffer   = errors.New("no current buffer")
	ErrReadonly   = errors.New("read-only buffer")
	ErrNoFilename = errors.New("no filename")
	// AutoRevertMode and Dispatching are process settings installed by runtime.
	AutoRevertMode bool
	Dispatching    bool
)

func writeMessage(writef func(string, ...any), format string, args ...any) {
	if writef != nil {
		writef(format, args...)
	}
}

func FileMtime(fname string) time.Time {
	if fname == "" {
		return time.Time{}
	}
	fi, err := os.Stat(fname)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

func LoadCommandLineFiles(paths []string, nameFromPath func(string) string, loadFile func(string) error) {
	if len(paths) == 0 || loadFile == nil {
		return
	}
	_ = loadFile(paths[0])

	for i := len(paths) - 1; i >= 1; i-- {
		path := paths[i]
		abp := buffer.Create()
		if abp == nil {
			continue
		}
		if nameFromPath != nil {
			abp.Name = nameFromPath(path)
		}
		buffer.SetCurrent(abp)
		_ = loadFile(path)
	}

	if cw := window.Active.CurrentWindow; cw != nil && cw.Buffer != nil {
		buffer.SetCurrent(cw.Buffer)
	}
}

func LoadCurrentBuffer(fname string, writef func(string, ...any)) error {
	resolved := NormalizePath(fname)
	bp := buffer.All.Current
	if bp == nil {
		return ErrNoBuffer
	}
	if bp.IsReadonly {
		writeMessage(writef, "[read-only buffer]")
		return ErrReadonly
	}

	bp.Clear()
	bp.IsChanged = false
	bp.EolMode = buffer.EModeLF
	bp.FileName = resolved
	bp.LangMode = DetectLangMode(resolved)
	mode.ApplyLangIndentDefaults(bp)

	fh, err := os.Open(resolved)
	if err != nil {
		// Missing path is a successful "new file" buffer, matching historical behavior.
		writeMessage(writef, "[New file]")
		bp.Cursor = buffer.Location{Line: 1, Offset: 0}
		bp.Mark = buffer.Location{Line: 0, Offset: 0}
		return nil
	}
	defer fh.Close()

	writeMessage(writef, "[Reading file]")
	reader := bufio.NewReader(fh)
	nline := uint(0)
	eolMode := buffer.EModeLF

	var lineBuf bytes.Buffer
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				if lineBuf.Len() > 0 {
					bp.AppendLineBytes(lineBuf.Bytes())
					nline++
				}
				break
			}
			writeMessage(writef, "File read error")
			bp.Clear()
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
			bp.AppendLineBytes(lineBuf.Bytes())
			lineBuf.Reset()
			nline++
			continue
		}

		if b == '\n' {
			bp.AppendLineBytes(lineBuf.Bytes())
			lineBuf.Reset()
			nline++
			continue
		}

		lineBuf.WriteByte(b)
	}

	bp.EolMode = eolMode
	if nline == 1 {
		writeMessage(writef, "[Read 1 line]")
	} else {
		writeMessage(writef, "[Read lines]")
	}

	bp.Cursor = buffer.Location{Line: 1, Offset: 0}
	bp.Mark = buffer.Location{Line: 0, Offset: 0}

	if wp := window.Active.CurrentWindow; wp != nil && wp.Buffer == bp {
		wp.TopLine = 1
		wp.Cursor = buffer.Location{Line: 1, Offset: 0}
		wp.Mark = buffer.Location{Line: 0, Offset: 0}
		wp.ShouldRedraw = true
		wp.ShouldUpdateModeLine = true
	}

	bp.FileMtime = FileMtime(resolved)
	bp.DiskChangeNotifiedMtime = time.Time{}
	return nil
}

// NeedsOverwriteConfirm reports whether fn's on-disk mtime differs from the buffer's.
func NeedsOverwriteConfirm(fn string) bool {
	bp := buffer.All.Current
	if bp == nil || bp.FileMtime.IsZero() {
		return false
	}
	curMtime := FileMtime(fn)
	return !curMtime.IsZero() && !curMtime.Equal(bp.FileMtime)
}

// SaveCurrentBufferForce writes the buffer without the overwrite-mtime prompt.
func SaveCurrentBufferForce(fn string, writef func(string, ...any)) error {
	bp := buffer.All.Current
	if bp == nil {
		return ErrNoBuffer
	}

	if bp.WhitespaceCleanup {
		for i := uint(1); i <= bp.LineCount; i++ {
			bp.TrimTrailingWhitespace(i)
		}
	}

	fh, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		writeMessage(writef, "[cannot open file for writing]")
		return fmt.Errorf("open for write: %w", err)
	}
	defer fh.Close()

	eol := []byte("\n")
	if bp.EolMode == buffer.EModeCRLF {
		eol = []byte("\r\n")
	} else if bp.EolMode == buffer.EModeCR {
		eol = []byte("\r")
	}

	writer := bufio.NewWriter(fh)
	nline := 0
	for i := uint(1); i <= bp.LineCount; i++ {
		line := bp.Line(i)
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

	bp.FileMtime = FileMtime(fn)
	bp.DiskChangeNotifiedMtime = time.Time{}
	bp.IsChanged = false
	return nil
}

func ReloadCurrentBufferFromDisk(fname string, lineNumber uint, noteBufferSaved func(*buffer.Buffer), writef func(string, ...any)) error {
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil {
		return ErrNoBuffer
	}
	if fname == "" {
		return ErrNoFilename
	}
	if err := LoadCurrentBuffer(fname, writef); err != nil {
		return err
	}
	if noteBufferSaved != nil {
		noteBufferSaved(bp)
	}
	bp.DiskChangeNotifiedMtime = time.Time{}
	if wp != nil && lineNumber > 0 && lineNumber <= bp.LineCount {
		wp.Cursor = buffer.MakeLocation(lineNumber, 0)
		wp.ShouldRedraw = true
		wp.ShouldUpdateModeLine = true
	}
	for _, w := range window.Active.Windows {
		if w != nil && w.Buffer == bp {
			w.ShouldRedraw = true
			w.ShouldUpdateModeLine = true
		}
	}
	return nil
}

// CheckReloadCurrentBuffer mirrors src/file.c file_check_reload behavior.
// askConfirm, when non-nil, is used for dirty-buffer revert prompts (async-friendly:
// callers may schedule work in onYes/onNo and return immediately).
func CheckReloadCurrentBuffer(askConfirm func(prompt string, onYes, onNo func()), writef func(string, ...any), noteBufferSaved func(*buffer.Buffer)) {
	if minibuffer.Active != nil || Dispatching {
		return
	}
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil || bp.IsReadonly {
		return
	}
	fname := bp.FileName
	if fname == "" || bp.FileMtime.IsZero() {
		return
	}

	cur := FileMtime(fname)
	if cur.IsZero() || cur.Equal(bp.FileMtime) {
		if !bp.DiskChangeNotifiedMtime.IsZero() {
			bp.DiskChangeNotifiedMtime = time.Time{}
			if wp != nil {
				wp.ShouldUpdateModeLine = true
			}
		}
		return
	}

	lineNumber := uint(1)
	if wp != nil {
		lineNumber = wp.Cursor.Line
	}

	if bp.IsChanged {
		if AutoRevertMode {
			_ = ReloadCurrentBufferFromDisk(fname, lineNumber, noteBufferSaved, writef)
			return
		}
		if cur.Equal(bp.DiskChangeNotifiedMtime) {
			return
		}
		bp.DiskChangeNotifiedMtime = cur
		if wp != nil {
			wp.ShouldUpdateModeLine = true
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

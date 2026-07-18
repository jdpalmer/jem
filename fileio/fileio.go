package fileio

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/session"
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

func LoadCommandLineFiles(paths []string, nameFromPath func(string) string, loadFile func(string) bool) {
	if len(paths) == 0 || loadFile == nil {
		return
	}
	_ = loadFile(paths[0])

	for i := len(paths) - 1; i >= 1; i-- {
		path := paths[i]
		abp := session.BufferCreate(&session.App.EditorRuntimeState)
		if abp == nil {
			continue
		}
		if nameFromPath != nil {
			abp.Name = nameFromPath(path)
		}
		session.SetCurrentBuffer(abp)
		_ = loadFile(path)
	}

	if cw := session.App.CurrentWindow; cw != nil && cw.Buffer != nil {
		session.SetCurrentBuffer(cw.Buffer)
	}
}

func LoadCurrentBuffer(fname string, writef func(string, ...any)) bool {
	resolved := NormalizePath(fname)
	bp := session.App.CurrentBuffer
	if bp == nil {
		return false
	}
	if bp.IsReadonly {
		writeMessage(writef, "[read-only buffer]")
		return false
	}

	buffer.Clear(bp)
	bp.IsChanged = false
	bp.EolMode = buffer.EModeLF
	bp.FileName = resolved
	bp.LangMode = DetectLangMode(resolved)

	fh, err := os.Open(resolved)
	if err != nil {
		writeMessage(writef, "[New file]")
		bp.Cursor = session.Location{Line: 1, Offset: 0}
		bp.Mark = session.Location{Line: 0, Offset: 0}
		return true
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
					buffer.AppendLineBytes(bp, lineBuf.Bytes(), uint(lineBuf.Len()))
					nline++
				}
				break
			}
			writeMessage(writef, "File read error")
			buffer.Clear(bp)
			return false
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
			buffer.AppendLineBytes(bp, lineBuf.Bytes(), uint(lineBuf.Len()))
			lineBuf.Reset()
			nline++
			continue
		}

		if b == '\n' {
			buffer.AppendLineBytes(bp, lineBuf.Bytes(), uint(lineBuf.Len()))
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

	bp.Cursor = session.Location{Line: 1, Offset: 0}
	bp.Mark = session.Location{Line: 0, Offset: 0}

	if wp := session.App.CurrentWindow; wp != nil && wp.Buffer == bp {
		wp.TopLine = 1
		wp.Cursor = session.Location{Line: 1, Offset: 0}
		wp.Mark = session.Location{Line: 0, Offset: 0}
		wp.ShouldRedraw = true
		wp.ShouldUpdateModeLine = true
	}

	bp.FileMtime = FileMtime(resolved)
	bp.DiskChangeNotifiedMtime = time.Time{}
	return true
}

func SaveCurrentBuffer(fn string, confirmOverwrite func(string) bool, writef func(string, ...any)) bool {
	bp := session.App.CurrentBuffer
	if bp == nil {
		return false
	}

	if bp.WhitespaceCleanup {
		for i := uint(1); i <= bp.LineCount; i++ {
			buffer.TrimLineTrailingWhitespace(bp, i)
		}
	}

	if !bp.FileMtime.IsZero() {
		curMtime := FileMtime(fn)
		if !curMtime.IsZero() && !curMtime.Equal(bp.FileMtime) {
			if confirmOverwrite == nil || !confirmOverwrite("file changed on disk. overwrite") {
				return false
			}
		}
	}

	fh, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		writeMessage(writef, "[cannot open file for writing]")
		return false
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
		line := buffer.GetLine(bp, i)
		if line == nil {
			continue
		}
		if len(line.Data) > 0 {
			if _, err := writer.Write(line.Data); err != nil {
				writeMessage(writef, "Write I/O error")
				return false
			}
		}
		if _, err := writer.Write(eol); err != nil {
			writeMessage(writef, "Write I/O error")
			return false
		}
		nline++
	}

	if err := writer.Flush(); err != nil {
		writeMessage(writef, "Write I/O error")
		return false
	}

	if nline == 1 {
		writeMessage(writef, "[wrote 1 line]")
	} else {
		writeMessage(writef, "[wrote lines]")
	}

	bp.FileMtime = FileMtime(fn)
	bp.DiskChangeNotifiedMtime = time.Time{}
	bp.IsChanged = false
	return true
}

func ReloadCurrentBufferFromDisk(fname string, lineNumber uint, noteBufferSaved func(*session.Buffer), writef func(string, ...any)) bool {
	bp := session.App.CurrentBuffer
	wp := session.App.CurrentWindow
	if bp == nil || fname == "" {
		return false
	}
	if !LoadCurrentBuffer(fname, writef) {
		return false
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
	for i := 0; i < int(session.App.WindowCount); i++ {
		w := session.App.WINDOWS[i]
		if w != nil && w.Buffer == bp {
			w.ShouldRedraw = true
			w.ShouldUpdateModeLine = true
		}
	}
	return true
}

// CheckReloadCurrentBuffer mirrors src/file.c file_check_reload behavior.
func CheckReloadCurrentBuffer(confirm func(string) bool, writef func(string, ...any), noteBufferSaved func(*session.Buffer)) {
	if session.App.ActiveMinibuffer != nil || session.App.Dispatching {
		return
	}
	bp := session.App.CurrentBuffer
	wp := session.App.CurrentWindow
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
		if session.App.AutoRevertMode {
			ReloadCurrentBufferFromDisk(fname, lineNumber, noteBufferSaved, writef)
			return
		}
		if cur.Equal(bp.DiskChangeNotifiedMtime) {
			return
		}
		bp.DiskChangeNotifiedMtime = cur
		if wp != nil {
			wp.ShouldUpdateModeLine = true
		}
		if confirm != nil && confirm("File changed on disk; revert") {
			ReloadCurrentBufferFromDisk(fname, lineNumber, noteBufferSaved, writef)
			writeMessage(writef, "[Reverted]")
		} else {
			writeMessage(writef, "[keeping edited buffer]")
		}
		return
	}

	ReloadCurrentBufferFromDisk(fname, lineNumber, noteBufferSaved, writef)
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

package runtime

// File visit/save/revert commands and disk-change reload checks.

import (
	"path/filepath"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/files"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
)

// loadCommandLineFiles loads paths into buffers at startup.
// The first path replaces the initial buffer; each remaining path gets its own
// buffer. On return the first file's buffer is current and shown in the window.
func loadCommandLineFiles(paths []string) {
	files.LoadCommandLineFiles(paths, files.BufferNameFromPath, func(path string) error {
		return files.LoadCurrentBuffer(path, display.MBWrite)
	})
}

func fileLoad(fname string) bool {
	return files.LoadCurrentBuffer(fname, display.MBWrite) == nil
}

// fileSaveBuffer saves fn. onDone is optional; when overwrite confirmation is
// needed, save completes asynchronously via AskYesNo.
func fileSaveBuffer(fn string, onDone func(ok bool)) {
	finish := func(ok bool) {
		if onDone != nil {
			onDone(ok)
		}
	}
	if files.NeedsOverwriteConfirm(fn) {
		AskYesNo("file changed on disk. overwrite", func() {
			finish(files.SaveCurrentBufferForce(fn, display.MBWrite) == nil)
		}, func() {
			finish(false)
		})
		return
	}
	finish(files.SaveCurrentBufferForce(fn, display.MBWrite) == nil)
}

// fileReloadFromDisk reloads fname into the current buffer and restores lineNumber.
func fileReloadFromDisk(fname string, lineNumber int) bool {
	return files.ReloadCurrentBufferFromDisk(fname, lineNumber, NoteBufferSaved, display.MBWrite) == nil
}

// fileCheckReload silently reloads unmodified buffers when the on-disk file
// changes; prompts before reverting modified buffers.
func fileCheckReload() {
	files.CheckReloadCurrentBuffer(func(prompt string, onYes, onNo func()) {
		AskYesNo(prompt, onYes, onNo)
	}, display.MBWrite, NoteBufferSaved, State.Dispatching, State.AutoRevertMode)
}

// CmdFileSave saves the current buffer to its filename, or prompts for a
// filename if none is set. Returns true on success.
func CmdFileSave(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	if buf.FileName != "" {
		fileSaveBuffer(buf.FileName, func(ok bool) {
			if ok {
				display.MBWrite("[Saved]")
			}
		})
		return true
	}
	AskStringCap("Write file: ", "", files.PromptPathCapacity, func(fname string, res minibuffer.PromptResult) {
		if res != minibuffer.PromptResultYes || fname == "" {
			return
		}
		fileSaveBuffer(fname, func(ok bool) {
			if ok {
				buf.FileName = fname
				display.MBWrite("[Saved]")
			}
		})
	})
	return true
}

// visitFilePath opens path in a buffer, reusing an existing buffer when possible.
func visitFilePath(path string) bool {
	if path == "" {
		return false
	}
	fileName := files.NormalizePath(path)

	var found *buffer.Buffer
	for i := 0; i < len(buffer.All.Buffers); i++ {
		buf := buffer.All.Buffers[i]
		if buf != nil && files.PathsEqual(buf.FileName, fileName) {
			found = buf
			break
		}
	}

	if found != nil {
		markring.PushCurrent()
		window.SwitchBuffer(found)
		if win := window.Active.CurrentWindow; win != nil {
			win.ShouldRedraw = true
			win.ShouldUpdateModeLine = true
		}
		display.MBWrite("[old buffer]")
		return true
	}

	buf := buffer.Create()
	if buf == nil {
		display.MBWrite("[cannot create buffer]")
		return false
	}
	markring.PushCurrent()
	buf.Name = files.BufferNameFromPath(fileName)
	window.SwitchBuffer(buf)
	if !fileLoad(fileName) {
		return false
	}
	if win := window.Active.CurrentWindow; win != nil {
		win.CenterCursor()
		win.ShouldRedraw = true
		win.ShouldUpdateModeLine = true
	}
	return true
}

// CmdFileVisit prompts for a filename, creates a new buffer and loads the
// file into it. Returns true on success.
func CmdFileVisit(f bool, n int) bool {
	_ = f
	_ = n
	initial := ""
	if buf := buffer.All.Current; buf != nil {
		if fname := buf.FileName; fname != "" {
			dir := filepath.Dir(files.ExpandPath(fname))
			initial = dir + string(filepath.Separator)
		}
	}
	AskFilename("Visit file: ", initial, func(path string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes || path == "" {
			return
		}
		visitFilePath(path)
	})
	return true
}

// CmdFileWrite prompts for a filename and writes the current buffer (save-as).
func CmdFileWrite(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	AskFilename("Write file: ", buf.FileName, func(path string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes || path == "" {
			return
		}
		path = files.NormalizePath(path)
		fileSaveBuffer(path, func(ok bool) {
			if !ok {
				return
			}
			buf.FileName = path
			buf.LangMode = files.DetectLangMode(path)
			NoteBufferSaved(buf)
			for i := 0; i < len(window.Active.Windows); i++ {
				win := window.Active.Windows[i]
				if win != nil && win.Buffer == buf {
					win.ShouldRedraw = true
					win.ShouldUpdateModeLine = true
				}
			}
		})
	})
	return true
}

// CmdRevertFile reloads the current buffer from its file on disk, discarding
// unsaved edits. Bound to C-x C-v (Emacs revert-buffer).
func CmdRevertFile(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	fname := buf.FileName
	if fname == "" {
		display.MBWrite("[no file associated with buffer]")
		return false
	}
	lineNumber := 1
	if win != nil {
		lineNumber = win.Cursor.Line
	}
	doRevert := func() {
		if !fileReloadFromDisk(fname, lineNumber) {
			return
		}
		display.MBWrite("[Reverted]")
	}
	if buf.IsChanged {
		AskYesNo("Buffer modified; revert anyway", doRevert, func() {
			display.MBWrite("[not reverted]")
		})
		return true
	}
	doRevert()
	return true
}

// fileVisitLocation opens path in a buffer and moves the cursor to line/column (1-based).
func fileVisitLocation(path string, line, column int) bool {
	if line == 0 {
		return false
	}
	if !visitFilePath(path) {
		return false
	}

	win := window.Active.CurrentWindow
	if win == nil || win.Buffer == nil {
		return false
	}
	if line > len(win.Buffer.Lines) {
		display.MBWrite("[file line out of range]")
		return false
	}
	bline := win.Buffer.Line(line)
	off := column
	if column > 0 {
		off--
	}
	if bline != nil && off > bline.Len() {
		off = bline.Len()
	}
	win.SetCursor(buffer.MakeLocation(line, off))
	win.DidMove = true
	win.ShouldUpdateModeLine = true
	win.ShouldRedraw = true
	win.CenterCursor()
	return true
}

// CmdFileRead loads a file into the current buffer. Bound to C-x C-r.
func CmdFileRead(f bool, n int) bool {
	_ = f
	_ = n
	AskFilename("Read file: ", "", func(path string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes || path == "" {
			return
		}
		fileLoad(files.NormalizePath(path))
	})
	return true
}

func eolModeLabel(mode buffer.EolMode) string {
	switch mode {
	case buffer.EModeCRLF:
		return "CRLF"
	case buffer.EModeCR:
		return "CR"
	default:
		return "LF"
	}
}

// CmdSetEolMode sets the line-ending mode for the current buffer.
func CmdSetEolMode(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	if buf.IsReadonly {
		display.MBWrite("[read-only buffer]")
		return false
	}
	choices := []string{"LF", "CRLF", "CR"}
	modes := []buffer.EolMode{buffer.EModeLF, buffer.EModeCRLF, buffer.EModeCR}
	defaultIdx := 0
	for i, mode := range modes {
		if buf.EolMode == mode {
			defaultIdx = i
			break
		}
	}
	labelFn := func(ctx any, idx int) []byte {
		sl := ctx.([]string)
		if idx < len(sl) {
			return []byte(sl[idx])
		}
		return nil
	}
	AskChoose("EOL mode: ", choices, labelFn, 3, defaultIdx, func(selected int) {
		if selected == -2 {
			CmdAbort(false, 1)
			return
		}
		if selected < 0 {
			return
		}
		chosen := modes[selected]
		if buf.EolMode != chosen {
			buf.EolMode = chosen
			buf.IsChanged = true
			for i := 0; i < len(window.Active.Windows); i++ {
				win := window.Active.Windows[i]
				if win != nil && win.Buffer == buf {
					win.ShouldUpdateModeLine = true
				}
			}
		}
		display.MBWrite("[EOL mode: %s]", eolModeLabel(chosen))
	})
	return true
}

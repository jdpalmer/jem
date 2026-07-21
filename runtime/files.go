package runtime

// File visit/save/revert commands and disk-change reload checks.

import (
	"fmt"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
	"path/filepath"
	"strings"

	"github.com/jdpalmer/jem/buffer"

	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/files"
)

func bufferNameFromPath(fname string) string {
	base := filepath.Base(fname)
	if i := strings.IndexByte(base, ';'); i >= 0 {
		base = base[:i]
	}
	if buffer.Find(base) == nil {
		return buffer.TruncateName(base)
	}
	for suffix := 2; ; suffix++ {
		name := fmt.Sprintf("%s:%d", base, suffix)
		if buffer.Find(name) == nil {
			return buffer.TruncateName(name)
		}
	}
}

// loadCommandLineFiles loads paths into buffers at startup.
// The first path replaces the initial buffer; each remaining path gets its own
// buffer. On return the first file's buffer is current and shown in the window.
func loadCommandLineFiles(paths []string) {
	files.LoadCommandLineFiles(paths, bufferNameFromPath, func(path string) error {
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
func fileReloadFromDisk(fname string, lineNumber uint) bool {
	return files.ReloadCurrentBufferFromDisk(fname, lineNumber, NoteBufferSaved, display.MBWrite) == nil
}

// fileCheckReload silently reloads unmodified buffers when the on-disk file
// changes; prompts before reverting modified buffers.
func fileCheckReload() {
	files.CheckReloadCurrentBuffer(func(prompt string, onYes, onNo func()) {
		AskYesNo(prompt, onYes, onNo)
	}, display.MBWrite, NoteBufferSaved)
}

// CmdFileSave saves the current buffer to its filename, or prompts for a
// filename if none is set. Returns true on success.
func CmdFileSave(f bool, n int) bool {
	_ = f
	_ = n
	bp := buffer.All.Current
	if bp == nil {
		display.MBWrite("[no buffer]")
		return false
	}
	if bp.FileName != "" {
		fileSaveBuffer(bp.FileName, func(ok bool) {
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
				bp.FileName = fname
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
	for i := 0; i < int(len(buffer.All.Buffers)); i++ {
		bp := buffer.All.Buffers[i]
		if bp != nil && files.PathsEqual(bp.FileName, fileName) {
			found = bp
			break
		}
	}

	if found != nil {
		markring.PushCurrent()
		window.SwitchBuffer(found)
		if wp := window.Active.CurrentWindow; wp != nil {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
		display.MBWrite("[old buffer]")
		return true
	}

	bp := buffer.Create()
	if bp == nil {
		display.MBWrite("[cannot create buffer]")
		return false
	}
	markring.PushCurrent()
	bp.Name = bufferNameFromPath(fileName)
	window.SwitchBuffer(bp)
	if !fileLoad(fileName) {
		return false
	}
	if wp := window.Active.CurrentWindow; wp != nil {
		wp.CenterCursor()
		wp.ShouldRedraw = true
		wp.ShouldUpdateModeLine = true
	}
	return true
}

// CmdFileVisit prompts for a filename, creates a new buffer and loads the
// file into it. Returns true on success.
func CmdFileVisit(f bool, n int) bool {
	_ = f
	_ = n
	initial := ""
	if bp := buffer.All.Current; bp != nil {
		if fname := bp.FileName; fname != "" {
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
	bp := buffer.All.Current
	if bp == nil {
		display.MBWrite("[no buffer]")
		return false
	}
	AskFilename("Write file: ", bp.FileName, func(path string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes || path == "" {
			return
		}
		path = files.NormalizePath(path)
		fileSaveBuffer(path, func(ok bool) {
			if !ok {
				return
			}
			bp.FileName = path
			bp.LangMode = files.DetectLangMode(path)
			NoteBufferSaved(bp)
			for i := 0; i < int(len(window.Active.Windows)); i++ {
				wp := window.Active.Windows[i]
				if wp != nil && wp.Buffer == bp {
					wp.ShouldRedraw = true
					wp.ShouldUpdateModeLine = true
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
	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil {
		display.MBWrite("[no buffer]")
		return false
	}
	fname := bp.FileName
	if fname == "" {
		display.MBWrite("[no file associated with buffer]")
		return false
	}
	lineNumber := uint(1)
	if wp != nil {
		lineNumber = wp.Cursor.Line
	}
	doRevert := func() {
		if !fileReloadFromDisk(fname, lineNumber) {
			return
		}
		display.MBWrite("[Reverted]")
	}
	if bp.IsChanged {
		AskYesNo("Buffer modified; revert anyway", doRevert, func() {
			display.MBWrite("[not reverted]")
		})
		return true
	}
	doRevert()
	return true
}

// fileVisitLocation opens path in a buffer and moves the cursor to line/column (1-based).
func fileVisitLocation(path string, line, column uint32) bool {
	if line == 0 {
		return false
	}
	if !visitFilePath(path) {
		return false
	}

	wp := window.Active.CurrentWindow
	if wp == nil || wp.Buffer == nil {
		return false
	}
	if uint(line) > wp.Buffer.LineCount {
		display.MBWrite("[file line out of range]")
		return false
	}
	lp := wp.Buffer.Line(uint(line))
	off := uint(column)
	if column > 0 {
		off--
	}
	if off > lp.Len() {
		off = lp.Len()
	}
	wp.SetCursor(buffer.MakeLocation(uint(line), off))
	wp.DidMove = true
	wp.ShouldUpdateModeLine = true
	wp.ShouldRedraw = true
	wp.CenterCursor()
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
	bp := buffer.All.Current
	if bp == nil {
		display.MBWrite("[no buffer]")
		return false
	}
	if bp.IsReadonly {
		display.MBWrite("[read-only buffer]")
		return false
	}
	choices := []string{"LF", "CRLF", "CR"}
	modes := []buffer.EolMode{buffer.EModeLF, buffer.EModeCRLF, buffer.EModeCR}
	defaultIdx := uint8(0)
	for i, mode := range modes {
		if bp.EolMode == mode {
			defaultIdx = uint8(i)
			break
		}
	}
	labelFn := func(ctx any, idx uint8) []byte {
		sl := ctx.([]string)
		if int(idx) < len(sl) {
			return []byte(sl[idx])
		}
		return nil
	}
	AskChoose("EOL mode: ", choices, labelFn, 3, defaultIdx, func(selected int16) {
		if selected == -2 {
			CmdAbort(false, 1)
			return
		}
		if selected < 0 {
			return
		}
		chosen := modes[selected]
		if bp.EolMode != chosen {
			bp.EolMode = chosen
			bp.IsChanged = true
			for i := 0; i < int(len(window.Active.Windows)); i++ {
				wp := window.Active.Windows[i]
				if wp != nil && wp.Buffer == bp {
					wp.ShouldUpdateModeLine = true
				}
			}
		}
		display.MBWrite("[EOL mode: %s]", eolModeLabel(chosen))
	})
	return true
}

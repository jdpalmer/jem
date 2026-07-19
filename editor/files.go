package editor

// files.go - File I/O operations (translation of file.c)

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jdpalmer/jem/buffer"

	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/fileio"
	"github.com/jdpalmer/jem/view"
)

func bufferNameFromPath(fname string) string {
	base := filepath.Base(fname)
	if i := strings.IndexByte(base, ';'); i >= 0 {
		base = base[:i]
	}
	if model.BufferFind(base) == nil {
		return model.TruncateBufferName(base)
	}
	for suffix := 2; ; suffix++ {
		name := fmt.Sprintf("%s:%d", base, suffix)
		if model.BufferFind(name) == nil {
			return model.TruncateBufferName(name)
		}
	}
}

// loadCommandLineFiles loads paths into buffers, mirroring src/main.c startup.
// The first path replaces the initial buffer; each remaining path gets its own
// buffer. On return the first file's buffer is current and shown in the window.
func loadCommandLineFiles(paths []string) {
	fileio.LoadCommandLineFiles(paths, bufferNameFromPath, func(path string) error {
		return fileio.LoadCurrentBuffer(path, view.MBWrite)
	})
}

func fileLoad(fname string) bool {
	return fileio.LoadCurrentBuffer(fname, view.MBWrite) == nil
}

// fileSaveBuffer saves fn. onDone is optional; when overwrite confirmation is
// needed, save completes asynchronously via AskYesNo.
func fileSaveBuffer(fn string, onDone func(ok bool)) {
	finish := func(ok bool) {
		if onDone != nil {
			onDone(ok)
		}
	}
	if fileio.NeedsOverwriteConfirm(fn) {
		AskYesNo("file changed on disk. overwrite", func() {
			finish(fileio.SaveCurrentBufferForce(fn, view.MBWrite) == nil)
		}, func() {
			finish(false)
		})
		return
	}
	finish(fileio.SaveCurrentBufferForce(fn, view.MBWrite) == nil)
}

// fileReloadFromDisk reloads fname into the current buffer and restores lineNumber.
func fileReloadFromDisk(fname string, lineNumber uint) bool {
	return fileio.ReloadCurrentBufferFromDisk(fname, lineNumber, model.NoteBufferSaved, view.MBWrite) == nil
}

// fileCheckReload mirrors src/file.c file_check_reload: silently reload unmodified
// buffers when the on-disk file changes; prompt before reverting modified buffers.
func fileCheckReload() {
	fileio.CheckReloadCurrentBuffer(func(prompt string, onYes, onNo func()) {
		AskYesNo(prompt, onYes, onNo)
	}, view.MBWrite, model.NoteBufferSaved)
}

// CmdFileSave saves the current buffer to its filename, or prompts for a
// filename if none is set. Returns true on success.
func CmdFileSave(f bool, n int) bool {
	_ = f
	_ = n
	bp := model.State.CurrentBuffer
	if bp == nil {
		view.MBWrite("[no buffer]")
		return false
	}
	if bp.FileName != "" {
		fileSaveBuffer(bp.FileName, func(ok bool) {
			if ok {
				view.MBWrite("[Saved]")
			}
		})
		return true
	}
	AskStringCap("Write file: ", "", fileio.PromptPathCapacity, func(fname string, res model.PromptResult) {
		if res != model.PromptResultYes || fname == "" {
			return
		}
		fileSaveBuffer(fname, func(ok bool) {
			if ok {
				bp.FileName = fname
				view.MBWrite("[Saved]")
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
	fileName := fileio.NormalizePath(path)

	var buffer *buffer.Buffer
	for i := 0; i < int(len(model.State.Buffers)); i++ {
		bp := model.State.Buffers[i]
		if bp != nil && fileio.PathsEqual(bp.FileName, fileName) {
			buffer = bp
			break
		}
	}

	if buffer != nil {
		model.MarkPushCurrent()
		model.SwitchBuffer(buffer)
		if wp := model.State.CurrentWindow; wp != nil {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
		view.MBWrite("[old buffer]")
		return true
	}

	buffer = model.BufferCreate(&model.State.EditorRuntimeState)
	if buffer == nil {
		view.MBWrite("[cannot create buffer]")
		return false
	}
	model.MarkPushCurrent()
	buffer.Name = bufferNameFromPath(fileName)
	model.SwitchBuffer(buffer)
	if !fileLoad(fileName) {
		return false
	}
	if wp := model.State.CurrentWindow; wp != nil {
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
	if bp := model.State.CurrentBuffer; bp != nil {
		if fname := bp.FileName; fname != "" {
			dir := filepath.Dir(fileio.ExpandPath(fname))
			initial = dir + string(filepath.Separator)
		}
	}
	AskFilename("Visit file: ", initial, func(path string, pr model.PromptResult) {
		if pr != model.PromptResultYes || path == "" {
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
	bp := model.State.CurrentBuffer
	if bp == nil {
		view.MBWrite("[no buffer]")
		return false
	}
	AskFilename("Write file: ", bp.FileName, func(path string, pr model.PromptResult) {
		if pr != model.PromptResultYes || path == "" {
			return
		}
		path = fileio.NormalizePath(path)
		fileSaveBuffer(path, func(ok bool) {
			if !ok {
				return
			}
			bp.FileName = path
			bp.LangMode = fileio.DetectLangMode(path)
			model.NoteBufferSaved(bp)
			for i := 0; i < int(len(model.State.Windows)); i++ {
				wp := model.State.Windows[i]
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
	bp := model.State.CurrentBuffer
	wp := model.State.CurrentWindow
	if bp == nil {
		view.MBWrite("[no buffer]")
		return false
	}
	fname := bp.FileName
	if fname == "" {
		view.MBWrite("[no file associated with buffer]")
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
		view.MBWrite("[Reverted]")
	}
	if bp.IsChanged {
		AskYesNo("Buffer modified; revert anyway", doRevert, func() {
			view.MBWrite("[not reverted]")
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

	wp := model.State.CurrentWindow
	if wp == nil || wp.Buffer == nil {
		return false
	}
	if uint(line) > wp.Buffer.LineCount {
		view.MBWrite("[file line out of range]")
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
	AskFilename("Read file: ", "", func(path string, pr model.PromptResult) {
		if pr != model.PromptResultYes || path == "" {
			return
		}
		fileLoad(fileio.NormalizePath(path))
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
	bp := model.State.CurrentBuffer
	if bp == nil {
		view.MBWrite("[no buffer]")
		return false
	}
	if bp.IsReadonly {
		view.MBWrite("[read-only buffer]")
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
			for i := 0; i < int(len(model.State.Windows)); i++ {
				wp := model.State.Windows[i]
				if wp != nil && wp.Buffer == bp {
					wp.ShouldUpdateModeLine = true
				}
			}
		}
		view.MBWrite("[EOL mode: %s]", eolModeLabel(chosen))
	})
	return true
}

package editor

// fileio.go - File I/O operations (translation of file.c)

import (
	"github.com/jdpalmer/jem/buffer"
	"path/filepath"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/fileio"
	"github.com/jdpalmer/jem/ui"
)

// loadCommandLineFiles loads paths into buffers, mirroring src/main.c startup.
// The first path replaces the initial buffer; each remaining path gets its own
// buffer. On return the first file's buffer is current and shown in the window.
func loadCommandLineFiles(paths []string) {
	fileio.LoadCommandLineFiles(paths, bufferNameFromPath, func(path string) error {
		return fileio.LoadCurrentBuffer(path, ui.MBWrite)
	})
}

func fileLoad(fname string) bool {
	return fileio.LoadCurrentBuffer(fname, ui.MBWrite) == nil
}

func fileSaveBuffer(fn string) bool {
	return fileio.SaveCurrentBuffer(fn, func(prompt string) bool {
		return ui.MBYesNo(prompt) == app.PromptResultYes
	}, ui.MBWrite) == nil
}

// fileReloadFromDisk reloads fname into the current buffer and restores lineNumber.
func fileReloadFromDisk(fname string, lineNumber uint) bool {
	return fileio.ReloadCurrentBufferFromDisk(fname, lineNumber, UndoNoteBufferSaved, ui.MBWrite) == nil
}

// fileCheckReload mirrors src/file.c file_check_reload: silently reload unmodified
// buffers when the on-disk file changes; prompt before reverting modified buffers.
func fileCheckReload() {
	fileio.CheckReloadCurrentBuffer(func(prompt string) bool {
		return ui.MBYesNo(prompt) == app.PromptResultYes
	}, ui.MBWrite, UndoNoteBufferSaved)
}

// CmdFileSave saves the current buffer to its filename, or prompts for a
// filename if none is set. Returns true on success.
func CmdFileSave(f bool, n int) bool {
	_ = f
	_ = n
	bp := app.State.CurrentBuffer
	if bp == nil {
		ui.MBWrite("[no buffer]")
		return false
	}
	if bp.FileName != "" {
		if fileSaveBuffer(bp.FileName) {
			ui.MBWrite("[Saved]")
			return true
		}
		return false
	}
	fname, res := ui.MBReadStringCap("Write file: ", "", fileio.PromptPathCapacity)
	if res != app.PromptResultYes {
		return false
	}
	if fname == "" {
		return false
	}
	if fileSaveBuffer(fname) {
		bp.FileName = fname
		ui.MBWrite("[Saved]")
		return true
	}
	return false
}

// visitFilePath opens path in a buffer, reusing an existing buffer when possible.
func visitFilePath(path string) bool {
	if path == "" {
		return false
	}
	fileName := fileio.NormalizePath(path)

	var buffer *buffer.Buffer
	for i := 0; i < int(len(app.State.Buffers)); i++ {
		bp := app.State.Buffers[i]
		if bp != nil && fileio.PathsEqual(bp.FileName, fileName) {
			buffer = bp
			break
		}
	}

	if buffer != nil {
		app.MarkPushCurrent()
		editorSwitchBuffer(buffer)
		if wp := app.State.CurrentWindow; wp != nil {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
		ui.MBWrite("[old buffer]")
		return true
	}

	buffer = app.BufferCreate(&app.State.EditorRuntimeState)
	if buffer == nil {
		ui.MBWrite("[cannot create buffer]")
		return false
	}
	app.MarkPushCurrent()
	buffer.Name = bufferNameFromPath(fileName)
	editorSwitchBuffer(buffer)
	if !fileLoad(fileName) {
		return false
	}
	if wp := app.State.CurrentWindow; wp != nil {
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
	if bp := app.State.CurrentBuffer; bp != nil {
		if fname := bp.FileName; fname != "" {
			dir := filepath.Dir(fileio.ExpandPath(fname))
			initial = dir + string(filepath.Separator)
		}
	}
	path, pr := ui.MBReadFilenameString("Visit file: ", initial)
	if pr != app.PromptResultYes {
		return false
	}
	if path == "" {
		return false
	}
	return visitFilePath(path)
}

// CmdFileWrite prompts for a filename and writes the current buffer (save-as).
func CmdFileWrite(f bool, n int) bool {
	_ = f
	_ = n
	bp := app.State.CurrentBuffer
	if bp == nil {
		ui.MBWrite("[no buffer]")
		return false
	}
	path, pr := ui.MBReadFilenameString("Write file: ", bp.FileName)
	if pr != app.PromptResultYes {
		return false
	}
	if path == "" {
		return false
	}
	path = fileio.NormalizePath(path)
	if !fileSaveBuffer(path) {
		return false
	}
	bp.FileName = path
	bp.LangMode = fileio.DetectLangMode(path)
	UndoNoteBufferSaved(bp)
	for i := 0; i < int(len(app.State.WINDOWS)); i++ {
		wp := app.State.WINDOWS[i]
		if wp != nil && wp.Buffer == bp {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
	}
	return true
}

// CmdRevertFile reloads the current buffer from its file on disk, discarding
// unsaved edits. Bound to C-x C-v (Emacs revert-buffer).
func CmdRevertFile(f bool, n int) bool {
	_ = f
	_ = n
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil {
		ui.MBWrite("[no buffer]")
		return false
	}
	fname := bp.FileName
	if fname == "" {
		ui.MBWrite("[no file associated with buffer]")
		return false
	}
	if bp.IsChanged {
		if ui.MBYesNo("Buffer modified; revert anyway") != app.PromptResultYes {
			ui.MBWrite("[not reverted]")
			return false
		}
	}
	lineNumber := uint(1)
	if wp != nil {
		lineNumber = wp.Cursor.Line
	}
	if !fileReloadFromDisk(fname, lineNumber) {
		return false
	}
	ui.MBWrite("[Reverted]")
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

	wp := app.State.CurrentWindow
	if wp == nil || wp.Buffer == nil {
		return false
	}
	if uint(line) > wp.Buffer.LineCount {
		ui.MBWrite("[file line out of range]")
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
	path, pr := ui.MBReadFilenameString("Read file: ", "")
	if pr != app.PromptResultYes {
		return false
	}
	if path == "" {
		return false
	}
	return fileLoad(fileio.NormalizePath(path))
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
	bp := app.State.CurrentBuffer
	if bp == nil {
		ui.MBWrite("[no buffer]")
		return false
	}
	if bp.IsReadonly {
		ui.MBWrite("[read-only buffer]")
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
	selected := ui.MBChoose("EOL mode: ", choices, labelFn, 3, defaultIdx)
	if selected == -2 {
		CmdAbort(false, 1)
		return false
	}
	if selected < 0 {
		return false
	}
	chosen := modes[selected]
	if bp.EolMode != chosen {
		bp.EolMode = chosen
		bp.IsChanged = true
		for i := 0; i < int(len(app.State.WINDOWS)); i++ {
			wp := app.State.WINDOWS[i]
			if wp != nil && wp.Buffer == bp {
				wp.ShouldUpdateModeLine = true
			}
		}
	}
	ui.MBWrite("[EOL mode: %s]", eolModeLabel(chosen))
	return true
}

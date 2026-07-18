package editor

// fileio.go - File I/O operations (translation of file.c)

import (
	"path/filepath"
	"time"

	"github.com/jdpalmer/jem/fileio"
	sess "github.com/jdpalmer/jem/session"
)

func fileMtime(fname string) time.Time {
	return fileio.FileMtime(fname)
}

// loadCommandLineFiles loads paths into buffers, mirroring src/main.c startup.
// The first path replaces the initial buffer; each remaining path gets its own
// buffer. On return the first file's buffer is current and shown in the window.
func loadCommandLineFiles(paths []string) {
	fileio.LoadCommandLineFiles(paths, bufferNameFromPath, fileLoad)
}

func fileLoad(fname string) bool {
	return fileio.LoadCurrentBuffer(fname, mbWrite)
}

func fileSaveBuffer(fn string) bool {
	return fileio.SaveCurrentBuffer(fn, func(prompt string) bool {
		return mbYesNo(prompt) == PromptResultYes
	}, mbWrite)
}

// fileReloadFromDisk reloads fname into the current buffer and restores lineNumber.
func fileReloadFromDisk(fname string, lineNumber uint) bool {
	return fileio.ReloadCurrentBufferFromDisk(fname, lineNumber, UndoNoteBufferSaved, mbWrite)
}

// fileCheckReload mirrors src/file.c file_check_reload: silently reload unmodified
// buffers when the on-disk file changes; prompt before reverting modified buffers.
func fileCheckReload() {
	fileio.CheckReloadCurrentBuffer(func(prompt string) bool {
		return mbYesNo(prompt) == PromptResultYes
	}, mbWrite, UndoNoteBufferSaved)
}

func langModeDetect(fname string) LangMode {
	return fileio.DetectLangMode(fname)
}

// CmdFileSave saves the current buffer to its filename, or prompts for a
// filename if none is set. Returns true on success.
func CmdFileSave(f bool, n int) bool {
	_ = f
	_ = n
	bp := session.App.CurrentBuffer
	if bp == nil {
		mbWrite("[no buffer]")
		return false
	}
	if bp.FileName != "" {
		if fileSaveBuffer(bp.FileName) {
			mbWrite("[Saved]")
			return true
		}
		return false
	}
	fname, res := mbReadStringCap("Write file: ", "", PromptPathCapacity)
	if res != PromptResultYes {
		return false
	}
	if fname == "" {
		return false
	}
	if fileSaveBuffer(fname) {
		bp.FileName = fname
		mbWrite("[Saved]")
		return true
	}
	return false
}

// visitFilePath opens path in a buffer, reusing an existing buffer when possible.
func visitFilePath(path string) bool {
	if path == "" {
		return false
	}
	fileName := fileNormalizePath(path)

	var buffer *Buffer
	for i := 0; i < int(session.App.BufferCount); i++ {
		bp := session.App.Buffers[i]
		if bp != nil && filePathsEqual(bp.FileName, fileName) {
			buffer = bp
			break
		}
	}

	if buffer != nil {
		markPushCurrent()
		editorSwitchBuffer(buffer)
		if wp := session.App.CurrentWindow; wp != nil {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
		mbWrite("[old buffer]")
		return true
	}

	buffer = sess.BufferCreate(&session.App.EditorRuntimeState)
	if buffer == nil {
		mbWrite("[cannot create buffer]")
		return false
	}
	markPushCurrent()
	buffer.Name = bufferNameFromPath(fileName)
	editorSwitchBuffer(buffer)
	if !fileLoad(fileName) {
		return false
	}
	if wp := session.App.CurrentWindow; wp != nil {
		sess.WindowCenterCursor(wp)
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
	if bp := session.App.CurrentBuffer; bp != nil {
		if fname := bp.FileName; fname != "" {
			dir := filepath.Dir(fileExpandPath(fname))
			initial = dir + string(filepath.Separator)
		}
	}
	path, pr := mbReadFilenameString("Visit file: ", initial)
	if pr != PromptResultYes {
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
	bp := session.App.CurrentBuffer
	if bp == nil {
		mbWrite("[no buffer]")
		return false
	}
	path, pr := mbReadFilenameString("Write file: ", bp.FileName)
	if pr != PromptResultYes {
		return false
	}
	if path == "" {
		return false
	}
	path = fileNormalizePath(path)
	if !fileSaveBuffer(path) {
		return false
	}
	bp.FileName = path
	bp.LangMode = langModeDetect(path)
	UndoNoteBufferSaved(bp)
	for i := 0; i < int(session.App.WindowCount); i++ {
		wp := session.App.WINDOWS[i]
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
	bp := session.App.CurrentBuffer
	wp := session.App.CurrentWindow
	if bp == nil {
		mbWrite("[no buffer]")
		return false
	}
	fname := bp.FileName
	if fname == "" {
		mbWrite("[no file associated with buffer]")
		return false
	}
	if bp.IsChanged {
		if mbYesNo("Buffer modified; revert anyway") != PromptResultYes {
			mbWrite("[not reverted]")
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
	mbWrite("[Reverted]")
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

	wp := session.App.CurrentWindow
	if wp == nil || wp.Buffer == nil {
		return false
	}
	if uint(line) > wp.Buffer.LineCount {
		mbWrite("[file line out of range]")
		return false
	}
	lp := BufferGetLine(wp.Buffer, uint(line))
	off := uint(column)
	if column > 0 {
		off--
	}
	if off > LineLength(lp) {
		off = LineLength(lp)
	}
	sess.WindowSetCursor(wp, MakeLocation(uint(line), off))
	wp.DidMove = true
	wp.ShouldUpdateModeLine = true
	wp.ShouldRedraw = true
	sess.WindowCenterCursor(wp)
	return true
}

// CmdFileRead loads a file into the current buffer. Bound to C-x C-r.
func CmdFileRead(f bool, n int) bool {
	_ = f
	_ = n
	path, pr := mbReadFilenameString("Read file: ", "")
	if pr != PromptResultYes {
		return false
	}
	if path == "" {
		return false
	}
	return fileLoad(fileNormalizePath(path))
}

func eolModeLabel(mode EolMode) string {
	switch mode {
	case EModeCRLF:
		return "CRLF"
	case EModeCR:
		return "CR"
	default:
		return "LF"
	}
}

// CmdSetEolMode sets the line-ending mode for the current buffer.
func CmdSetEolMode(f bool, n int) bool {
	_ = f
	_ = n
	bp := session.App.CurrentBuffer
	if bp == nil {
		mbWrite("[no buffer]")
		return false
	}
	if bp.IsReadonly {
		mbWrite("[read-only buffer]")
		return false
	}
	choices := []string{"LF", "CRLF", "CR"}
	modes := []EolMode{EModeLF, EModeCRLF, EModeCR}
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
	selected := mbChoose("EOL mode: ", choices, labelFn, 3, defaultIdx)
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
		for i := 0; i < int(session.App.WindowCount); i++ {
			wp := session.App.WINDOWS[i]
			if wp != nil && wp.Buffer == bp {
				wp.ShouldUpdateModeLine = true
			}
		}
	}
	mbWrite("[EOL mode: %s]", eolModeLabel(chosen))
	return true
}

package editor

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/completion"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/ui"
)

// macro.go - Macro recording/playback and command execution (translation of src/macro.c)

func macroInit() {
	if len(app.State.Keys) < app.MacroCapacity {
		app.State.Keys = make([]int32, app.MacroCapacity)
	} else {
		clear(app.State.Keys)
	}
	app.State.Keys[0] = int32(term.CTLX | ')')
	app.State.RecordPos = -1
	app.State.PlayPos = -1
}

func macroRefreshModelines() {
	for i := 0; i < int(len(app.State.WINDOWS)); i++ {
		if wp := app.State.WINDOWS[i]; wp != nil {
			wp.ShouldUpdateModeLine = true
		}
	}
}

func macroRecordAppend(k int32) bool {
	if !app.State.IsRecording() {
		return true
	}
	if app.State.RecordPos >= app.MacroCapacity {
		return CmdAbort(false, 1)
	}
	app.State.Keys[app.State.RecordPos] = k
	app.State.RecordPos++
	return true
}

// macroRecordBytes appends raw bytes (e.g. buffer names, prompt input) to the macro.
func macroRecordBytes(data []byte) bool {
	if !app.State.IsRecording() {
		return true
	}
	if app.State.RecordPos+len(data) > app.MacroCapacity-3 {
		return CmdAbort(false, 1)
	}
	for _, b := range data {
		if !macroRecordAppend(int32(b)) {
			return false
		}
	}
	return true
}

// macroRecordKey records one key for the active macro, if any.
func macroRecordKey(c int, f bool, n int) bool {
	if !app.State.IsRecording() {
		return true
	}
	if c != int(term.CTLX|'E') && c != int(term.CTLX|')') && app.State.RecordPos > app.MacroCapacity-6 {
		return CmdAbort(false, 1)
	}
	if c != int(term.CTLX|'E') {
		if f {
			if !macroRecordAppend(int32(term.CTL | 'U')) {
				return false
			}
			if !macroRecordAppend(int32(n)) {
				return false
			}
		}
		if !macroRecordAppend(int32(c)) {
			return false
		}
	}
	return true
}

// Execute dispatches a key through the command table or self-insert path.
// Mirrors execute() in src/macro.c.
func Execute(c int, f bool, n int) bool {
	if app.State.CurrentBuffer == nil || app.State.CurrentWindow == nil {
		return false
	}

	if bp := app.State.CurrentBuffer; bp != nil && bp.Name == grepBufferName {
		keycode := uint32(c)
		if keycode == term.KeyEnter || keycode == '\r' || keycode == '\n' {
			return CmdGrepVisitMatch(f, n)
		}
	}
	if bp := app.State.CurrentBuffer; bp != nil && bp.Name == compileBufferName {
		keycode := uint32(c)
		if keycode == term.KeyEnter || keycode == '\r' || keycode == '\n' {
			return CmdCompileVisitDiag(f, n)
		}
	}

	if app.State.MovementState > app.CmdStateNone {
		app.State.MovementState--
	}
	if app.State.KillState > app.CmdStateNone {
		app.State.KillState--
	}

	keycode := uint32(c)
	var cmd CommandFunc
	if (keycode&term.KeyMask) == 0 && c >= '!' && c <= '~' && c != '}' {
		cmd = nil
	} else if fn, ok := keybindingsMap[keycode]; ok {
		cmd = fn
	}

	trackUndo := keycode != (term.CTL | 'Z')

	if cmd != nil {
		if f && !commandAcceptsArgByKey[keycode] {
			ui.MBWrite("[command does not take an argument]")
			return false
		}
		if trackUndo {
			UndoBeginCommand()
		}
		clearCompletionPending()
		result := cmd(f, n)
		if trackUndo {
			UndoEndCommand()
		}
		return result
	}

	if trackUndo {
		UndoBeginCommand()
	}
	clearCompletionPending()

	if (keycode&term.KeyMask) == 0 && keycode >= 0x20 && keycode <= 0x10FFFF {
		if n <= 0 {
			if trackUndo {
				UndoEndCommand()
			}
			return n >= 0
		}
		ok := true
		for i := 0; i < n && ok; i++ {
			if keycode == term.KeyEnter || keycode == '\r' || keycode == '\n' {
				ok = CmdInsertChar('\n')
			} else if keycode < 127 {
				ok = CmdInsertChar(byte(keycode))
			} else {
				ok = false
			}
		}
		if trackUndo {
			UndoEndCommand()
		}
		return ok
	}

	if trackUndo {
		UndoEndCommand()
	}
	return false
}

func clearCompletionPending() {
	completion.ClearPending()
}

// processEditorKey reads one key (with optional C-u prefix) and executes it.
func processEditorKey(k uint32) bool {
	if k == 0x03 {
		return false
	}

	f := false
	n := 1

	if k == (term.CTL | 'U') {
		f = true
		n = 4
		mflag := 0
		ui.MBWrite("Arg: 4")
		for {
			next, ok := <-GlobalKeyCh
			if !ok {
				return true
			}
			if !((next >= '0' && next <= '9') || next == (term.CTL|'U') || next == '-') {
				k = next
				break
			}
			if next == (term.CTL | 'U') {
				n *= 4
			} else if next == '-' {
				if mflag != 0 {
					k = next
					break
				}
				n = 0
				mflag = -1
			} else {
				if mflag == 0 {
					n = 0
					mflag = 1
				}
				n = 10*n + int(next-'0')
			}
			displayN := n
			if mflag < 0 {
				if n == 0 {
					displayN = -1
				} else {
					displayN = -n
				}
			}
			ui.MBWrite("Arg: %d", displayN)
		}
		if mflag == -1 {
			if n == 0 {
				n++
			}
			n = -n
		}
	}

	if app.State.IsRecording() {
		if !macroRecordKey(int(k), f, n) {
			return true
		}
	}
	return Execute(int(k), f, n)
}

// CmdMacroStart begins keyboard macro recording (C-x ().
func CmdMacroStart(f bool, n int) bool {
	_ = f
	_ = n
	if app.State.IsPlaying() {
		ui.MBWrite("Not now")
		return false
	}
	if app.State.IsRecording() {
		ui.MBWrite("Not now")
		return false
	}
	ui.MBWrite("[start macro]")
	app.State.RecordPos = 0
	macroRefreshModelines()
	return true
}

// CmdMacroEnd stops keyboard macro recording (C-x )).
func CmdMacroEnd(f bool, n int) bool {
	_ = f
	_ = n
	if !app.State.IsRecording() {
		ui.MBWrite("[not now]")
		return false
	}
	ui.MBWrite("[end macro]")
	app.State.RecordPos = -1
	macroRefreshModelines()
	return true
}

// CmdMacroExec replays the recorded macro (C-x e).
func CmdMacroExec(f bool, n int) bool {
	repeat := 1
	if f {
		repeat = n
	}
	if app.State.IsRecording() || app.State.IsPlaying() {
		ui.MBWrite("[not now]")
		return false
	}
	if repeat <= 0 {
		return true
	}

	success := true
	for i := 0; i < repeat; i++ {
		app.State.PlayPos = 0
		for {
			af := false
			an := 1
			if app.State.PlayPos >= app.MacroCapacity {
				break
			}
			c := int(app.State.Keys[app.State.PlayPos])
			app.State.PlayPos++
			if c == int(term.CTL|'U') {
				af = true
				if app.State.PlayPos >= app.MacroCapacity {
					success = false
					break
				}
				an = int(app.State.Keys[app.State.PlayPos])
				app.State.PlayPos++
				if app.State.PlayPos >= app.MacroCapacity {
					success = false
					break
				}
				c = int(app.State.Keys[app.State.PlayPos])
				app.State.PlayPos++
			}
			if c == int(term.CTLX|')') {
				break
			}
			if !Execute(c, af, an) {
				success = false
				break
			}
		}
		app.State.PlayPos = -1
		if !success {
			break
		}
	}
	return success
}

// CmdAbort cancels macro recording, a running background job, or other transient state (C-g).
func CmdAbort(f bool, n int) bool {
	_ = f
	_ = n
	term.Beep()
	if backgroundJobRequestCancel() {
		return false
	}
	if app.State.IsRecording() {
		app.State.Keys[0] = int32(term.CTLX | ')')
		app.State.RecordPos = -1
	}
	macroRefreshModelines()
	ui.MBWrite("[cancelled]")
	return false
}

// macroRecordMinibufferResult records accepted minibuffer text during macro recording.
func macroRecordMinibufferResult(text []byte) {
	if !app.State.IsRecording() || len(text) == 0 {
		return
	}
	data := make([]byte, len(text)+1)
	copy(data, text)
	macroRecordBytes(data)
}

// macroRecordBufferName records a buffer name (NUL-terminated) during macro recording.
func macroRecordBufferName(bp *buffer.Buffer) {
	if !app.State.IsRecording() || bp == nil {
		return
	}
	data := append([]byte(bp.Name), 0)
	macroRecordBytes(data)
}

package editor

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/term"
)

// macro.go - Macro recording/playback and command execution (translation of src/macro.c)

func macroInit() {
	app.State.Keys[0] = int32(CTLX | ')')
	app.State.RecordPos = -1
	app.State.PlayPos = -1
}

func macroRefreshModelines() {
	for i := 0; i < int(app.State.WindowCount); i++ {
		if wp := app.State.WINDOWS[i]; wp != nil {
			wp.ShouldUpdateModeLine = true
		}
	}
}

func macroRecordAppend(k int32) bool {
	if !app.State.IsRecording() {
		return true
	}
	if app.State.RecordPos >= MacroCapacity {
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
	if app.State.RecordPos+len(data) > MacroCapacity-3 {
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
	if c != int(CTLX|'E') && c != int(CTLX|')') && app.State.RecordPos > MacroCapacity-6 {
		return CmdAbort(false, 1)
	}
	if c != int(CTLX|'E') {
		if f {
			if !macroRecordAppend(int32(CTL | 'U')) {
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

// macroPlayPrompt fills buf from the macro stream during playback (until a NUL byte).
func macroPlayPrompt(buf []byte) (PromptResult, bool) {
	if !app.State.IsPlaying() {
		return PromptResultAbort, false
	}
	pos := 0
	for app.State.PlayPos < MacroCapacity {
		c := app.State.Keys[app.State.PlayPos]
		app.State.PlayPos++
		if c == 0 {
			break
		}
		if pos+1 < len(buf) {
			buf[pos] = byte(c)
			pos++
		}
	}
	if pos < len(buf) {
		buf[pos] = 0
	}
	if pos > 0 && buf[0] != 0 {
		return PromptResultYes, true
	}
	return PromptResultNo, true
}

// Execute dispatches a key through the command table or self-insert path.
// Mirrors execute() in src/macro.c.
func Execute(c int, f bool, n int) bool {
	if app.State.CurrentBuffer == nil || app.State.CurrentWindow == nil {
		return false
	}

	if bp := app.State.CurrentBuffer; bp != nil && bp.Name == grepBufferName {
		keycode := uint32(c)
		if keycode == KeyEnter || keycode == '\r' || keycode == '\n' {
			return CmdGrepVisitMatch(f, n)
		}
	}
	if bp := app.State.CurrentBuffer; bp != nil && bp.Name == compileBufferName {
		keycode := uint32(c)
		if keycode == KeyEnter || keycode == '\r' || keycode == '\n' {
			return CmdCompileVisitDiag(f, n)
		}
	}

	if app.State.MovementState > CmdStateNone {
		app.State.MovementState--
	}
	if app.State.KillState > CmdStateNone {
		app.State.KillState--
	}

	keycode := uint32(c)
	var cmd CommandFunc
	if (keycode&KeyMask) == 0 && c >= '!' && c <= '~' && c != '}' {
		cmd = nil
	} else if fn, ok := keybindingsMap[keycode]; ok {
		cmd = fn
	}

	trackUndo := keycode != (CTL | 'Z')

	if cmd != nil {
		if f && !commandAcceptsArgByKey[keycode] {
			mbWrite("[command does not take an argument]")
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

	if (keycode&KeyMask) == 0 && keycode >= 0x20 && keycode <= 0x10FFFF {
		if n <= 0 {
			if trackUndo {
				UndoEndCommand()
			}
			return n >= 0
		}
		ok := true
		for i := 0; i < n && ok; i++ {
			if keycode == KeyEnter || keycode == '\r' || keycode == '\n' {
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
	completionPending = ""
}

// processEditorKey reads one key (with optional C-u prefix) and executes it.
func processEditorKey(k uint32) bool {
	if k == 0x03 {
		return false
	}

	f := false
	n := 1

	if k == (CTL | 'U') {
		f = true
		n = 4
		mflag := 0
		mbWrite("Arg: 4")
		for {
			next, ok := <-GlobalKeyCh
			if !ok {
				return true
			}
			if !((next >= '0' && next <= '9') || next == (CTL|'U') || next == '-') {
				k = next
				break
			}
			if next == (CTL | 'U') {
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
			mbWrite("Arg: %d", displayN)
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
		mbWrite("Not now")
		return false
	}
	if app.State.IsRecording() {
		mbWrite("Not now")
		return false
	}
	mbWrite("[start macro]")
	app.State.RecordPos = 0
	macroRefreshModelines()
	return true
}

// CmdMacroEnd stops keyboard macro recording (C-x )).
func CmdMacroEnd(f bool, n int) bool {
	_ = f
	_ = n
	if !app.State.IsRecording() {
		mbWrite("[not now]")
		return false
	}
	mbWrite("[end macro]")
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
		mbWrite("[not now]")
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
			if app.State.PlayPos >= MacroCapacity {
				break
			}
			c := int(app.State.Keys[app.State.PlayPos])
			app.State.PlayPos++
			if c == int(CTL|'U') {
				af = true
				if app.State.PlayPos >= MacroCapacity {
					success = false
					break
				}
				an = int(app.State.Keys[app.State.PlayPos])
				app.State.PlayPos++
				if app.State.PlayPos >= MacroCapacity {
					success = false
					break
				}
				c = int(app.State.Keys[app.State.PlayPos])
				app.State.PlayPos++
			}
			if c == int(CTLX|')') {
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
		app.State.Keys[0] = int32(CTLX | ')')
		app.State.RecordPos = -1
	}
	macroRefreshModelines()
	mbWrite("[cancelled]")
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
func macroRecordBufferName(bp *Buffer) {
	if !app.State.IsRecording() || bp == nil {
		return
	}
	data := append([]byte(bp.Name), 0)
	macroRecordBytes(data)
}

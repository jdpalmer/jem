package editor

import "github.com/jdpalmer/jem/term"

// macro.go - Macro recording/playback and command execution (translation of src/macro.c)

func macroInit() {
	session.App.Keys[0] = int32(CTLX | ')')
	session.App.RecordPos = -1
	session.App.PlayPos = -1
}

func macroRefreshModelines() {
	for i := 0; i < int(session.App.WindowCount); i++ {
		if wp := session.App.WINDOWS[i]; wp != nil {
			wp.ShouldUpdateModeLine = true
		}
	}
}

func macroRecordAppend(k int32) bool {
	if !session.App.IsRecording() {
		return true
	}
	if session.App.RecordPos >= MacroCapacity {
		return CmdAbort(false, 1)
	}
	session.App.Keys[session.App.RecordPos] = k
	session.App.RecordPos++
	return true
}

// macroRecordBytes appends raw bytes (e.g. buffer names, prompt input) to the macro.
func macroRecordBytes(data []byte) bool {
	if !session.App.IsRecording() {
		return true
	}
	if session.App.RecordPos+len(data) > MacroCapacity-3 {
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
	if !session.App.IsRecording() {
		return true
	}
	if c != int(CTLX|'E') && c != int(CTLX|')') && session.App.RecordPos > MacroCapacity-6 {
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
	if !session.App.IsPlaying() {
		return PromptResultAbort, false
	}
	pos := 0
	for session.App.PlayPos < MacroCapacity {
		c := session.App.Keys[session.App.PlayPos]
		session.App.PlayPos++
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
	if session.App.CurrentBuffer == nil || session.App.CurrentWindow == nil {
		return false
	}

	if bp := session.App.CurrentBuffer; bp != nil && bp.Name == grepBufferName {
		keycode := uint32(c)
		if keycode == KeyEnter || keycode == '\r' || keycode == '\n' {
			return CmdGrepVisitMatch(f, n)
		}
	}
	if bp := session.App.CurrentBuffer; bp != nil && bp.Name == compileBufferName {
		keycode := uint32(c)
		if keycode == KeyEnter || keycode == '\r' || keycode == '\n' {
			return CmdCompileVisitDiag(f, n)
		}
	}

	if session.App.MovementState > CmdStateNone {
		session.App.MovementState--
	}
	if session.App.KillState > CmdStateNone {
		session.App.KillState--
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

	if session.App.IsRecording() {
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
	if session.App.IsPlaying() {
		mbWrite("Not now")
		return false
	}
	if session.App.IsRecording() {
		mbWrite("Not now")
		return false
	}
	mbWrite("[start macro]")
	session.App.RecordPos = 0
	macroRefreshModelines()
	return true
}

// CmdMacroEnd stops keyboard macro recording (C-x )).
func CmdMacroEnd(f bool, n int) bool {
	_ = f
	_ = n
	if !session.App.IsRecording() {
		mbWrite("[not now]")
		return false
	}
	mbWrite("[end macro]")
	session.App.RecordPos = -1
	macroRefreshModelines()
	return true
}

// CmdMacroExec replays the recorded macro (C-x e).
func CmdMacroExec(f bool, n int) bool {
	repeat := 1
	if f {
		repeat = n
	}
	if session.App.IsRecording() || session.App.IsPlaying() {
		mbWrite("[not now]")
		return false
	}
	if repeat <= 0 {
		return true
	}

	success := true
	for i := 0; i < repeat; i++ {
		session.App.PlayPos = 0
		for {
			af := false
			an := 1
			if session.App.PlayPos >= MacroCapacity {
				break
			}
			c := int(session.App.Keys[session.App.PlayPos])
			session.App.PlayPos++
			if c == int(CTL|'U') {
				af = true
				if session.App.PlayPos >= MacroCapacity {
					success = false
					break
				}
				an = int(session.App.Keys[session.App.PlayPos])
				session.App.PlayPos++
				if session.App.PlayPos >= MacroCapacity {
					success = false
					break
				}
				c = int(session.App.Keys[session.App.PlayPos])
				session.App.PlayPos++
			}
			if c == int(CTLX|')') {
				break
			}
			if !Execute(c, af, an) {
				success = false
				break
			}
		}
		session.App.PlayPos = -1
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
	if session.App.IsRecording() {
		session.App.Keys[0] = int32(CTLX | ')')
		session.App.RecordPos = -1
	}
	macroRefreshModelines()
	mbWrite("[cancelled]")
	return false
}

// macroRecordMinibufferResult records accepted minibuffer text during macro recording.
func macroRecordMinibufferResult(text []byte) {
	if !session.App.IsRecording() || len(text) == 0 {
		return
	}
	data := make([]byte, len(text)+1)
	copy(data, text)
	macroRecordBytes(data)
}

// macroRecordBufferName records a buffer name (NUL-terminated) during macro recording.
func macroRecordBufferName(bp *Buffer) {
	if !session.App.IsRecording() || bp == nil {
		return
	}
	data := append([]byte(bp.Name), 0)
	macroRecordBytes(data)
}

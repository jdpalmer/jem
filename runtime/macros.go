package runtime

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/killring"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
	"github.com/jdpalmer/jem/window"
)

// macros.go - Macro recording/playback as []event.Event

func macroInit() {
	State.Macro = nil
	display.Active.MacroRecording = false
	display.Active.MacroPlaying = false
	display.Active.MacroPresent = false
	State.PlayPos = -1
}

func macroRefreshModelines() {
	for i := 0; i < int(len(window.Active.Windows)); i++ {
		if wp := window.Active.Windows[i]; wp != nil {
			wp.ShouldUpdateModeLine = true
		}
	}
}

func macroAppend(e event.Event) bool {
	if !State.IsRecording() || e == nil {
		return true
	}
	if len(State.Macro) >= MacroCapacity {
		return CmdAbort(false, 1)
	}
	State.Macro = append(State.Macro, e)
	display.Active.MacroPresent = true
	return true
}

// macroRecordKey records one keystroke (with optional universal arg) on the tape.
func macroRecordKey(c int, f bool, n int) bool {
	if !State.IsRecording() {
		return true
	}
	// Do not record macro control keys.
	if c == int(term.CTLX|'E') || c == int(term.CTLX|'(') || c == int(term.CTLX|')') {
		return true
	}
	if len(State.Macro) >= MacroCapacity {
		return CmdAbort(false, 1)
	}
	stepN := n
	if !f {
		stepN = 1
	}
	return macroAppend(event.MacroStepEvent{Code: uint32(c), F: f, N: stepN})
}

// Execute dispatches a key through the command table or self-insert path.
func Execute(c int, f bool, n int) bool {
	if buffer.All.Current == nil || window.Active.CurrentWindow == nil {
		return false
	}

	if bp := buffer.All.Current; bp != nil && bp.Name == tools.GrepBufferName {
		keycode := uint32(c)
		if keycode == term.KeyEnter || keycode == '\r' || keycode == '\n' {
			return tools.VisitGrepMatch()
		}
	}
	if bp := buffer.All.Current; bp != nil && bp.Name == tools.CompileBufferName {
		keycode := uint32(c)
		if keycode == term.KeyEnter || keycode == '\r' || keycode == '\n' {
			return tools.VisitCompileDiag()
		}
	}

	killring.Tick()

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
			display.MBWrite("[command does not take an argument]")
			return false
		}
		if trackUndo {
			BeginCommand()
		}
		clearCompletionPending()
		result := cmd(f, n)
		if trackUndo {
			EndCommand()
		}
		return result
	}

	if trackUndo {
		BeginCommand()
	}
	clearCompletionPending()

	if (keycode&term.KeyMask) == 0 && keycode >= 0x20 && keycode <= 0x10FFFF {
		if n <= 0 {
			if trackUndo {
				EndCommand()
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
			EndCommand()
		}
		return ok
	}

	if trackUndo {
		EndCommand()
	}
	return false
}

func clearCompletionPending() {
	ClearPending()
}

// CmdMacroStart begins keyboard macro recording (C-x ().
func CmdMacroStart(f bool, n int) bool {
	_ = f
	_ = n
	if State.IsPlaying() {
		display.MBWrite("Not now")
		return false
	}
	if State.IsRecording() {
		display.MBWrite("Not now")
		return false
	}
	display.MBWrite("[start macro]")
	State.Macro = make([]event.Event, 0, 32)
	display.Active.MacroRecording = true
	display.Active.MacroPresent = false
	macroRefreshModelines()
	return true
}

// CmdMacroEnd stops keyboard macro recording (C-x )).
func CmdMacroEnd(f bool, n int) bool {
	_ = f
	_ = n
	if !State.IsRecording() {
		display.MBWrite("[not now]")
		return false
	}
	display.MBWrite("[end macro]")
	display.Active.MacroRecording = false
	macroRefreshModelines()
	return true
}

// CmdMacroExec replays the recorded macro (C-x e).
func CmdMacroExec(f bool, n int) bool {
	repeat := 1
	if f {
		repeat = n
	}
	if State.IsRecording() || State.IsPlaying() {
		display.MBWrite("[not now]")
		return false
	}
	if repeat <= 0 || len(State.Macro) == 0 {
		return true
	}

	success := true
	for i := 0; i < repeat; i++ {
		State.PlayPos = 0
		display.Active.MacroPlaying = true
		for State.PlayPos < len(State.Macro) {
			e := State.Macro[State.PlayPos]
			State.PlayPos++
			switch ev := e.(type) {
			case event.MacroStepEvent:
				n := ev.N
				if !ev.F {
					n = 1
				}
				if !Execute(int(ev.Code), ev.F, n) {
					success = false
				}
			case event.PromptReplyEvent:
				// Orphaned reply (not consumed by Ask*); skip.
			default:
				// Unknown tape entry; skip.
			}
			if !success {
				break
			}
		}
		State.PlayPos = -1
		display.Active.MacroPlaying = false
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
	if tools.RequestBackgroundJobCancel() {
		return false
	}
	if State.IsRecording() {
		State.Macro = nil
		display.Active.MacroRecording = false
		display.Active.MacroPresent = false
	}
	macroRefreshModelines()
	display.MBWrite("[cancelled]")
	return false
}

// macroRecordMinibufferResult records accepted minibuffer text during macro recording.
func macroRecordMinibufferResult(text []byte) {
	if !State.IsRecording() || len(text) == 0 {
		return
	}
	_ = macroAppend(event.PromptReplyEvent{Text: string(text)})
}

// macroRecordBufferName records a buffer name during macro recording.
func macroRecordBufferName(bp *buffer.Buffer) {
	if !State.IsRecording() || bp == nil {
		return
	}
	_ = macroAppend(event.PromptReplyEvent{Text: bp.Name})
}

func TakeMacroPromptReply() (text string, pr minibuffer.PromptResult, playing bool) {
	if !State.IsPlaying() {
		return "", 0, false
	}
	if State.PlayPos >= len(State.Macro) {
		return "", minibuffer.PromptResultNo, true
	}
	ev, isReply := State.Macro[State.PlayPos].(event.PromptReplyEvent)
	if !isReply {
		return "", minibuffer.PromptResultNo, true
	}
	State.PlayPos++
	if ev.Text == "" {
		return "", minibuffer.PromptResultNo, true
	}
	return ev.Text, minibuffer.PromptResultYes, true
}

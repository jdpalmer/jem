package editor

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/event"
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
	"github.com/jdpalmer/jem/view"
)

// macros.go - Macro recording/playback as []event.Event

func macroInit() {
	model.State.Macro = nil
	model.State.Recording = false
	model.State.PlayPos = -1
}

func macroRefreshModelines() {
	for i := 0; i < int(len(model.State.Windows)); i++ {
		if wp := model.State.Windows[i]; wp != nil {
			wp.ShouldUpdateModeLine = true
		}
	}
}

func macroAppend(e event.Event) bool {
	if !model.State.IsRecording() || e == nil {
		return true
	}
	if len(model.State.Macro) >= model.MacroCapacity {
		return CmdAbort(false, 1)
	}
	model.State.Macro = append(model.State.Macro, e)
	return true
}

// macroRecordKey records one keystroke (with optional universal arg) on the tape.
func macroRecordKey(c int, f bool, n int) bool {
	if !model.State.IsRecording() {
		return true
	}
	// Do not record macro control keys.
	if c == int(term.CTLX|'E') || c == int(term.CTLX|'(') || c == int(term.CTLX|')') {
		return true
	}
	if len(model.State.Macro) >= model.MacroCapacity {
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
	if model.State.CurrentBuffer == nil || model.State.CurrentWindow == nil {
		return false
	}

	if bp := model.State.CurrentBuffer; bp != nil && bp.Name == tools.GrepBufferName {
		keycode := uint32(c)
		if keycode == term.KeyEnter || keycode == '\r' || keycode == '\n' {
			return tools.VisitGrepMatch()
		}
	}
	if bp := model.State.CurrentBuffer; bp != nil && bp.Name == tools.CompileBufferName {
		keycode := uint32(c)
		if keycode == term.KeyEnter || keycode == '\r' || keycode == '\n' {
			return tools.VisitCompileDiag()
		}
	}

	if model.State.MovementState > model.CmdStateNone {
		model.State.MovementState--
	}
	if model.State.KillState > model.CmdStateNone {
		model.State.KillState--
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
			view.MBWrite("[command does not take an argument]")
			return false
		}
		if trackUndo {
			model.BeginCommand()
		}
		clearCompletionPending()
		result := cmd(f, n)
		if trackUndo {
			model.EndCommand()
		}
		return result
	}

	if trackUndo {
		model.BeginCommand()
	}
	clearCompletionPending()

	if (keycode&term.KeyMask) == 0 && keycode >= 0x20 && keycode <= 0x10FFFF {
		if n <= 0 {
			if trackUndo {
				model.EndCommand()
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
			model.EndCommand()
		}
		return ok
	}

	if trackUndo {
		model.EndCommand()
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
	if model.State.IsPlaying() {
		view.MBWrite("Not now")
		return false
	}
	if model.State.IsRecording() {
		view.MBWrite("Not now")
		return false
	}
	view.MBWrite("[start macro]")
	model.State.Macro = make([]event.Event, 0, 32)
	model.State.Recording = true
	macroRefreshModelines()
	return true
}

// CmdMacroEnd stops keyboard macro recording (C-x )).
func CmdMacroEnd(f bool, n int) bool {
	_ = f
	_ = n
	if !model.State.IsRecording() {
		view.MBWrite("[not now]")
		return false
	}
	view.MBWrite("[end macro]")
	model.State.Recording = false
	macroRefreshModelines()
	return true
}

// CmdMacroExec replays the recorded macro (C-x e).
func CmdMacroExec(f bool, n int) bool {
	repeat := 1
	if f {
		repeat = n
	}
	if model.State.IsRecording() || model.State.IsPlaying() {
		view.MBWrite("[not now]")
		return false
	}
	if repeat <= 0 || len(model.State.Macro) == 0 {
		return true
	}

	success := true
	for i := 0; i < repeat; i++ {
		model.State.PlayPos = 0
		for model.State.PlayPos < len(model.State.Macro) {
			e := model.State.Macro[model.State.PlayPos]
			model.State.PlayPos++
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
		model.State.PlayPos = -1
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
	if model.State.IsRecording() {
		model.State.Macro = nil
		model.State.Recording = false
	}
	macroRefreshModelines()
	view.MBWrite("[cancelled]")
	return false
}

// macroRecordMinibufferResult records accepted minibuffer text during macro recording.
func macroRecordMinibufferResult(text []byte) {
	if !model.State.IsRecording() || len(text) == 0 {
		return
	}
	_ = macroAppend(event.PromptReplyEvent{Text: string(text)})
}

// macroRecordBufferName records a buffer name during macro recording.
func macroRecordBufferName(bp *buffer.Buffer) {
	if !model.State.IsRecording() || bp == nil {
		return
	}
	_ = macroAppend(event.PromptReplyEvent{Text: bp.Name})
}

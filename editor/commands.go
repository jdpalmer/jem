package editor

import (
	"strings"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/view"
)

// commands.go — command palette and buffer switching

// commandsProvider returns the command name label for the given index. ctx is a []string.
func commandsProvider(ctx any, idx uint) []byte {
	if ctx == nil {
		return nil
	}
	names, ok := ctx.([]string)
	if !ok {
		return nil
	}
	if int(idx) >= len(names) {
		return nil
	}
	return []byte(names[idx])
}

// CmdCommandPalette opens the command palette (M-x) and executes the chosen command.
func CmdCommandPalette(f bool, n int) bool {
	_ = f
	_ = n
	names := buildCommandList()
	if len(names) == 0 {
		view.MBWrite("[no commands]")
		return false
	}
	AskFuzzy("M-x: ", commandsProvider, names, uint(len(names)), func(label string, pr model.PromptResult) {
		if pr != model.PromptResultYes || label == "" {
			return
		}
		cmdName := strings.ToLower(label)
		if cmdFn, ok := commandNameMap[cmdName]; ok {
			cmdFn(false, 1)
			return
		}
		view.MBWrite("[unknown command: %s]", label)
	})
	return true
}

// CmdDescribeCommand shows the name and description of a selected command.
func CmdDescribeCommand(f bool, n int) bool {
	_ = f
	_ = n
	names := buildCommandList()
	if len(names) == 0 {
		view.MBWrite("[no commands]")
		return false
	}
	AskFuzzy("Describe: ", commandsProvider, names, uint(len(names)), func(label string, pr model.PromptResult) {
		if pr != model.PromptResultYes || label == "" {
			return
		}
		if cmd := commandByName(label); cmd != nil && cmd.Doc != "" {
			view.MBWrite("%s: %s", cmd.Name, cmd.Doc)
			return
		}
		view.MBWrite("Command: %s", label)
	})
	return true
}

// CmdKillBuffer kills/releases the current buffer (with yes/no confirmation).
func CmdKillBuffer(f bool, n int) bool {
	_ = f
	// If numeric argument provided, kill that buffer (1-based index)
	if n > 0 {
		if n <= len(model.State.Buffers) {
			bp := model.State.Buffers[n-1]
			if bp == nil {
				view.MBWrite("[no such buffer]")
				return false
			}
			AskYesNo("Kill buffer?", func() {
				model.BufferRelease(bp)
				view.MBWrite("[buffer killed]")
			}, func() {
				view.MBWrite("[aborted]")
			})
			return true
		}
		view.MBWrite("[no such buffer]")
		return false
	}

	bp := model.State.CurrentBuffer
	if bp == nil {
		view.MBWrite("[no buffer to kill]")
		return false
	}
	AskYesNo("Kill current buffer?", func() {
		model.BufferRelease(bp)
		view.MBWrite("[buffer killed]")
	}, func() {
		view.MBWrite("[aborted]")
	})
	return true
}

// CmdKillBufferFuzzy prompts with a fuzzy list of buffers and kills after confirmation.
func CmdKillBufferFuzzy(f bool, n int) bool {
	_ = f
	_ = n
	names := make([]string, 0, len(model.State.Buffers))
	for i := 0; i < len(model.State.Buffers); i++ {
		bp := model.State.Buffers[i]
		if bp == nil {
			continue
		}
		names = append(names, bp.Name)
	}
	if len(names) == 0 {
		view.MBWrite("[no buffers]")
		return false
	}
	AskFuzzy("Kill buffer: ", commandsProvider, names, uint(len(names)), func(label string, pr model.PromptResult) {
		if pr != model.PromptResultYes || label == "" {
			return
		}
		for i := 0; i < len(model.State.Buffers); i++ {
			bp := model.State.Buffers[i]
			if bp == nil {
				continue
			}
			if strings.EqualFold(bp.Name, label) {
				AskYesNo("Kill buffer?", func() {
					model.BufferRelease(bp)
					view.MBWrite("[buffer killed]")
				}, func() {
					view.MBWrite("[aborted]")
				})
				return
			}
		}
		view.MBWrite("[buffer not found: %s]", label)
	})
	return true
}

// pickBufferList returns the active buffers in editor order.
func pickBufferList() []*buffer.Buffer {
	list := make([]*buffer.Buffer, 0, len(model.State.Buffers))
	for i := 0; i < len(model.State.Buffers); i++ {
		if bp := model.State.Buffers[i]; bp != nil {
			list = append(list, bp)
		}
	}
	return list
}

func bufferChoiceLabel(ctx any, idx uint8) []byte {
	list, _ := ctx.([]*buffer.Buffer)
	if int(idx) >= len(list) || list[idx] == nil {
		return nil
	}
	return []byte(list[idx].Name)
}

func findBufferByLabel(label string) *buffer.Buffer {
	for i := 0; i < len(model.State.Buffers); i++ {
		bp := model.State.Buffers[i]
		if bp == nil {
			continue
		}
		if strings.EqualFold(bp.Name, label) {
			return bp
		}
	}
	return nil
}

func switchToBuffer(bp *buffer.Buffer) {
	if bp == nil {
		return
	}
	macroRecordBufferName(bp)
	model.SwitchBuffer(bp)
	view.DisplayUpdate()
}

// CmdUseBuffer switches to a buffer. With a universal argument (f true, n > 0),
// select the nth buffer (1-based) directly. Otherwise show a horizontal picker (C-x b).
func CmdUseBuffer(f bool, n int) bool {
	if f && n > 0 {
		if n <= len(model.State.Buffers) {
			bp := model.State.Buffers[n-1]
			if bp != nil {
				model.SwitchBuffer(bp)
				return true
			}
		}
		return false
	}

	buffers := pickBufferList()
	if len(buffers) == 0 {
		view.MBWrite("[no buffers]")
		return false
	}

	if model.State.IsPlaying() {
		AskStringCap("buffer.Buffer: ", "", model.BufferNameCapacity, func(label string, pr model.PromptResult) {
			if pr != model.PromptResultYes || label == "" {
				return
			}
			bp := findBufferByLabel(label)
			if bp == nil {
				view.MBWrite("[no such buffer]")
				return
			}
			switchToBuffer(bp)
		})
		return true
	}

	defaultIdx := uint8(0)
	if len(buffers) > 1 {
		defaultIdx = 1
	}
	AskChoose("buffer.Buffer: ", buffers, bufferChoiceLabel, uint8(len(buffers)), defaultIdx, func(sel int16) {
		if sel == -2 {
			CmdAbort(false, 1)
			return
		}
		if sel < 0 {
			return
		}
		switchToBuffer(buffers[sel])
	})
	return true
}

package runtime

import (
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
	"strings"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
)

// commands.go — command palette and buffer switching

// commandsProvider returns the command name label for the given index. ctx is a []string.
func commandsProvider(ctx any, idx int) []byte {
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
		display.MBWrite("[no commands]")
		return false
	}
	AskFuzzy("M-x: ", commandsProvider, names, len(names), func(label string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes || label == "" {
			return
		}
		cmdName := strings.ToLower(label)
		if cmdFn, ok := commandNameMap[cmdName]; ok {
			cmdFn(false, 1)
			return
		}
		display.MBWrite("[unknown command: %s]", label)
	})
	return true
}

// CmdDescribeCommand shows the name and description of a selected command.
func CmdDescribeCommand(f bool, n int) bool {
	_ = f
	_ = n
	names := buildCommandList()
	if len(names) == 0 {
		display.MBWrite("[no commands]")
		return false
	}
	AskFuzzy("Describe: ", commandsProvider, names, len(names), func(label string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes || label == "" {
			return
		}
		if cmd := commandByName(label); cmd != nil && cmd.Doc != "" {
			display.MBWrite("%s: %s", cmd.Name, cmd.Doc)
			return
		}
		display.MBWrite("Command: %s", label)
	})
	return true
}

// CmdKillBuffer kills/releases the current buffer (with yes/no confirmation).
func CmdKillBuffer(f bool, n int) bool {
	_ = f
	// If numeric argument provided, kill that buffer (1-based index)
	if n > 0 {
		if n <= len(buffer.All.Buffers) {
			buf := buffer.All.Buffers[n-1]
			if buf == nil {
				display.MBWrite("[no such buffer]")
				return false
			}
			AskYesNo("Kill buffer?", func() {
				buffer.Release(buf)
				display.MBWrite("[buffer killed]")
			}, func() {
				display.MBWrite("[aborted]")
			})
			return true
		}
		display.MBWrite("[no such buffer]")
		return false
	}

	buf := buffer.All.Current
	if buf == nil {
		display.MBWrite("[no buffer to kill]")
		return false
	}
	AskYesNo("Kill current buffer?", func() {
		buffer.Release(buf)
		display.MBWrite("[buffer killed]")
	}, func() {
		display.MBWrite("[aborted]")
	})
	return true
}

// CmdKillBufferFuzzy prompts with a fuzzy list of buffers and kills after confirmation.
func CmdKillBufferFuzzy(f bool, n int) bool {
	_ = f
	_ = n
	names := make([]string, 0, len(buffer.All.Buffers))
	for i := 0; i < len(buffer.All.Buffers); i++ {
		buf := buffer.All.Buffers[i]
		if buf == nil {
			continue
		}
		names = append(names, buf.Name)
	}
	if len(names) == 0 {
		display.MBWrite("[no buffers]")
		return false
	}
	AskFuzzy("Kill buffer: ", commandsProvider, names, len(names), func(label string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes || label == "" {
			return
		}
		for i := 0; i < len(buffer.All.Buffers); i++ {
			buf := buffer.All.Buffers[i]
			if buf == nil {
				continue
			}
			if strings.EqualFold(buf.Name, label) {
				AskYesNo("Kill buffer?", func() {
					buffer.Release(buf)
					display.MBWrite("[buffer killed]")
				}, func() {
					display.MBWrite("[aborted]")
				})
				return
			}
		}
		display.MBWrite("[buffer not found: %s]", label)
	})
	return true
}

// pickBufferList returns the active buffers in editor order.
func pickBufferList() []*buffer.Buffer {
	list := make([]*buffer.Buffer, 0, len(buffer.All.Buffers))
	for i := 0; i < len(buffer.All.Buffers); i++ {
		if buf := buffer.All.Buffers[i]; buf != nil {
			list = append(list, buf)
		}
	}
	return list
}

func bufferChoiceLabel(ctx any, idx int) []byte {
	list, _ := ctx.([]*buffer.Buffer)
	if int(idx) >= len(list) || list[idx] == nil {
		return nil
	}
	return []byte(display.FitBufferName(list[idx].Name, display.BufferNameMaxCols))
}

func findBufferByLabel(label string) *buffer.Buffer {
	for i := 0; i < len(buffer.All.Buffers); i++ {
		buf := buffer.All.Buffers[i]
		if buf == nil {
			continue
		}
		if strings.EqualFold(buf.Name, label) {
			return buf
		}
	}
	return nil
}

func switchToBuffer(buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	macroRecordBufferName(buf)
	window.SwitchBuffer(buf)
	display.DisplayUpdate()
}

// CmdUseBuffer switches to a buffer. With a universal argument (f true, n > 0),
// select the nth buffer (1-based) directly. Otherwise show a horizontal picker (C-x b).
func CmdUseBuffer(f bool, n int) bool {
	if f && n > 0 {
		if n <= len(buffer.All.Buffers) {
			buf := buffer.All.Buffers[n-1]
			if buf != nil {
				window.SwitchBuffer(buf)
				return true
			}
		}
		return false
	}

	buffers := pickBufferList()
	if len(buffers) == 0 {
		display.MBWrite("[no buffers]")
		return false
	}

	if State.IsPlaying() {
		AskString("buffer.Buffer: ", "", func(label string, pr minibuffer.PromptResult) {
			if pr != minibuffer.PromptResultYes || label == "" {
				return
			}
			buf := findBufferByLabel(label)
			if buf == nil {
				display.MBWrite("[no such buffer]")
				return
			}
			switchToBuffer(buf)
		})
		return true
	}

	defaultIdx := 0
	if len(buffers) > 1 {
		defaultIdx = 1
	}
	AskChoose("buffer.Buffer: ", buffers, bufferChoiceLabel, len(buffers), defaultIdx, func(sel int16) {
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

package editor

import (
	"strings"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/ui"
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
// CmdCommandPalette opens the command palette (M-x) and executes the chosen command.
func CmdCommandPalette(f bool, n int) bool {
	_ = f
	_ = n
	// Build provider list
	names := buildCommandList()
	if len(names) == 0 {
		ui.MBWrite("[no commands]")
		return false
	}
	label, pr := ui.MBReadFuzzyListString("M-x: ", commandsProvider, names, uint(len(names)))
	if pr != app.PromptResultYes {
		return false
	}
	if label == "" {
		return false
	}
	cmdName := strings.ToLower(label)
	if cmdFn, ok := commandNameMap[cmdName]; ok {
		cmdFn(false, 1)
		return true
	}
	ui.MBWrite("[unknown command: %s]", label)
	return false
}

// CmdDescribeCommand shows the name and description of a selected command.
// CmdDescribeCommand shows the name and description of a selected command.
func CmdDescribeCommand(f bool, n int) bool {
	_ = f
	_ = n
	names := buildCommandList()
	if len(names) == 0 {
		ui.MBWrite("[no commands]")
		return false
	}
	label, pr := ui.MBReadFuzzyListString("Describe: ", commandsProvider, names, uint(len(names)))
	if pr != app.PromptResultYes {
		return false
	}
	if label == "" {
		return false
	}
	if cmd := commandByName(label); cmd != nil && cmd.Doc != "" {
		ui.MBWrite("%s: %s", cmd.Name, cmd.Doc)
		return true
	}
	ui.MBWrite("Command: %s", label)
	return true
}

// CmdKillBuffer kills/releases the current buffer.
// CmdKillBuffer kills/releases the current buffer.
func CmdKillBuffer(f bool, n int) bool {
	_ = f
	// If numeric argument provided, kill that buffer (1-based index)
	if n > 0 {
		if n <= len(app.State.Buffers) {
			bp := app.State.Buffers[n-1]
			if bp == nil {
				ui.MBWrite("[no such buffer]")
				return false
			}
			// confirm
			if ui.MBYesNo("Kill buffer?") != app.PromptResultYes {
				ui.MBWrite("[aborted]")
				return false
			}
			app.BufferRelease(bp)
			ui.MBWrite("[buffer killed]")
			return true
		}
		ui.MBWrite("[no such buffer]")
		return false
	}

	// default: kill current buffer with confirmation
	bp := app.State.CurrentBuffer
	if bp == nil {
		ui.MBWrite("[no buffer to kill]")
		return false
	}
	if ui.MBYesNo("Kill current buffer?") != app.PromptResultYes {
		ui.MBWrite("[aborted]")
		return false
	}
	app.BufferRelease(bp)
	ui.MBWrite("[buffer killed]")
	return true
}

// CmdKillBufferFuzzy prompts the user with a fuzzy list of buffers and kills the
// chosen buffer after confirmation.
// CmdKillBufferFuzzy prompts the user with a fuzzy list of buffers and kills the
// chosen buffer after confirmation.
func CmdKillBufferFuzzy(f bool, n int) bool {
	_ = f
	_ = n
	names := make([]string, 0, len(app.State.Buffers))
	for i := 0; i < len(app.State.Buffers); i++ {
		bp := app.State.Buffers[i]
		if bp == nil {
			continue
		}
		names = append(names, bp.Name)
	}
	if len(names) == 0 {
		ui.MBWrite("[no buffers]")
		return false
	}
	label, pr := ui.MBReadFuzzyListString("Kill buffer: ", commandsProvider, names, uint(len(names)))
	if pr != app.PromptResultYes {
		return false
	}
	if label == "" {
		return false
	}
	// find buffer by name
	for i := 0; i < len(app.State.Buffers); i++ {
		bp := app.State.Buffers[i]
		if bp == nil {
			continue
		}
		if strings.EqualFold(bp.Name, label) {
			if ui.MBYesNo("Kill buffer?") != app.PromptResultYes {
				ui.MBWrite("[aborted]")
				return false
			}
			app.BufferRelease(bp)
			ui.MBWrite("[buffer killed]")
			return true
		}
	}
	ui.MBWrite("[buffer not found: %s]", label)
	return false
}

// pickBufferList returns the active buffers in editor order.
// pickBufferList returns the active buffers in editor order.
func pickBufferList() []*buffer.Buffer {
	list := make([]*buffer.Buffer, 0, len(app.State.Buffers))
	for i := 0; i < len(app.State.Buffers); i++ {
		if bp := app.State.Buffers[i]; bp != nil {
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
	for i := 0; i < len(app.State.Buffers); i++ {
		bp := app.State.Buffers[i]
		if bp == nil {
			continue
		}
		if strings.EqualFold(bp.Name, label) {
			return bp
		}
	}
	return nil
}

// CmdUseBuffer switches to a buffer. With a universal argument (f true, n > 0),
// select the nth buffer (1-based) directly. Otherwise show a horizontal picker (C-x b).
// CmdUseBuffer switches to a buffer. With a universal argument (f true, n > 0),
// select the nth buffer (1-based) directly. Otherwise show a horizontal picker (C-x b).
func CmdUseBuffer(f bool, n int) bool {
	if f && n > 0 {
		if n <= len(app.State.Buffers) {
			bp := app.State.Buffers[n-1]
			if bp != nil {
				editorSwitchBuffer(bp)
				return true
			}
		}
		return false
	}

	buffers := pickBufferList()
	if len(buffers) == 0 {
		ui.MBWrite("[no buffers]")
		return false
	}

	var bp *buffer.Buffer
	if app.State.IsPlaying() {
		label, pr := ui.MBReadStringCap("buffer.Buffer: ", "", app.BufferNameCapacity)
		if pr != app.PromptResultYes {
			return false
		}
		if label == "" {
			return false
		}
		bp = findBufferByLabel(label)
		if bp == nil {
			ui.MBWrite("[no such buffer]")
			return false
		}
	} else {
		defaultIdx := uint8(0)
		if len(buffers) > 1 {
			defaultIdx = 1
		}
		sel := ui.MBChoose("buffer.Buffer: ", buffers, bufferChoiceLabel, uint8(len(buffers)), defaultIdx)
		if sel == -2 {
			CmdAbort(false, 1)
			return false
		}
		if sel < 0 {
			return false
		}
		bp = buffers[sel]
	}

	macroRecordBufferName(bp)
	editorSwitchBuffer(bp)
	ui.DisplayUpdate()
	return true
}

// CmdBackToIndentation moves point to the first non-blank character on the line.

package runtime

import (
	"encoding/json"
	"fmt"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
	"os"
	"strconv"
	"strings"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/search"
)

// Editor variables (fill-column, indent settings, etc.).

type variable struct {
	name     string
	doc      string
	min      int
	max      int
	local    bool
	read     func(buf *buffer.Buffer) int
	write    func(buf *buffer.Buffer, value int)
	onChange func()
}

var varTable = []variable{
	{
		name: "fill-column",
		doc:  "Wrap/fill column used by paragraph filling.",
		min:  0, max: 1000, local: true,
		read:  func(buf *buffer.Buffer) int { return buf.FillCol },
		write: func(buf *buffer.Buffer, v int) { buf.FillCol = v },
	},
	{
		name: "theme-mode",
		doc:  "Editor palette mode: 0 dark, 1 light.",
		min:  0, max: int(display.ThemeLight), local: false,
		read: func(buf *buffer.Buffer) int {
			_ = buf
			return int(display.Active.Theme.Mode)
		},
		write: func(buf *buffer.Buffer, v int) {
			_ = buf
			display.Active.Theme.Mode = display.ThemeMode(v)
		},
		onChange: configThemeChanged,
	},
	{
		name: "search-scope",
		doc:  "Search scope: 0 current buffer, 1 all buffers.",
		min:  0, max: int(search.SearchScopeAllBuffers), local: false,
		read: func(buf *buffer.Buffer) int {
			_ = buf
			return int(search.DefaultState.SearchScopeSetting)
		},
		write: func(buf *buffer.Buffer, v int) {
			_ = buf
			search.DefaultState.SearchScopeSetting = search.SearchScopeMode(v)
		},
		onChange: configSearchScopeChanged,
	},
	{
		name: "whitespace-cleanup",
		doc:  "Trim trailing whitespace from every line before saving: 0 off, 1 on.",
		min:  0, max: 1, local: true,
		read:  func(buf *buffer.Buffer) int { return boolToInt(buf.WhitespaceCleanup) },
		write: func(buf *buffer.Buffer, v int) { buf.WhitespaceCleanup = v != 0 },
	},
	{
		name: "auto-revert-mode",
		doc:  "Reload buffers from disk when the file changes externally: 0 prompt if modified, 1 always reload.",
		min:  0, max: 1, local: false,
		read: func(buf *buffer.Buffer) int {
			_ = buf
			return boolToInt(State.AutoRevertMode)
		},
		write: func(buf *buffer.Buffer, v int) {
			_ = buf
			State.AutoRevertMode = v != 0
		},
	},
	{
		name: "c-indent",
		doc:  "C-family block indent width in spaces.",
		min:  0, max: 32, local: true,
		read:  func(buf *buffer.Buffer) int { return buf.CIndent },
		write: func(buf *buffer.Buffer, v int) { buf.CIndent = v },
	},
	{
		name: "c-brace",
		doc:  "Extra indent for a standalone opening brace line in C-like modes.",
		min:  0, max: 32, local: true,
		read:  func(buf *buffer.Buffer) int { return buf.CBrace },
		write: func(buf *buffer.Buffer, v int) { buf.CBrace = v },
	},
	{
		name: "c-colon-offset",
		doc:  "Extra offset applied to C-family case/default labels.",
		min:  0, max: 32, local: true,
		read:  func(buf *buffer.Buffer) int { return buf.CColonOffset },
		write: func(buf *buffer.Buffer, v int) { buf.CColonOffset = v },
	},
	{
		name: "py-indent",
		doc:  "Python block indent width in spaces.",
		min:  0, max: 32, local: true,
		read:  func(buf *buffer.Buffer) int { return buf.PyIndent },
		write: func(buf *buffer.Buffer, v int) { buf.PyIndent = v },
	},
	{
		name: "py-continued-offset",
		doc:  "Extra indent for explicit Python continuation lines.",
		min:  0, max: 32, local: true,
		read:  func(buf *buffer.Buffer) int { return buf.PyContinuedOffset },
		write: func(buf *buffer.Buffer, v int) { buf.PyContinuedOffset = v },
	},
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func configThemeChanged() {
	display.ThemeUpdate()
	syncSyntaxPalette()
	for i := 0; i < len(window.Active.Windows); i++ {
		win := window.Active.Windows[i]
		if win != nil {
			win.ShouldRedraw = true
			win.ShouldUpdateModeLine = true
		}
	}
}

func configSearchScopeChanged() {
	for i := 0; i < len(window.Active.Windows); i++ {
		win := window.Active.Windows[i]
		if win != nil {
			win.ShouldUpdateModeLine = true
		}
	}
}

// VarsInit resets all editor variables to their defaults.
func VarsInit() {
	display.Active.FillCol = 80
	display.Active.Theme.Mode = display.ThemeDark
	search.DefaultState.SearchScopeSetting = search.SearchScopeBuffer
	State.WhitespaceCleanup = true
	State.AutoRevertMode = false
	State.CIndent = 2
	State.CBrace = 0
	State.CColonOffset = 0
	State.PyIndent = 4
	State.PyContinuedOffset = 4
	configThemeChanged()
	configSearchScopeChanged()
}

func bufferApplyVarDefaults(buf *buffer.Buffer) {
	if buf == nil {
		return
	}
	buf.FillCol = display.Active.FillCol
	buf.CIndent = State.CIndent
	buf.CBrace = State.CBrace
	buf.CColonOffset = State.CColonOffset
	buf.PyIndent = State.PyIndent
	buf.PyContinuedOffset = State.PyContinuedOffset
	buf.WhitespaceCleanup = State.WhitespaceCleanup
}

func varGlobalWrite(v *variable, value int) {
	switch v.name {
	case "fill-column":
		display.Active.FillCol = value
	case "c-indent":
		State.CIndent = value
	case "c-brace":
		State.CBrace = value
	case "c-colon-offset":
		State.CColonOffset = value
	case "py-indent":
		State.PyIndent = value
	case "py-continued-offset":
		State.PyContinuedOffset = value
	case "whitespace-cleanup":
		State.WhitespaceCleanup = value != 0
	default:
		v.write(nil, value)
	}
}

func varGlobalRead(v *variable) int {
	switch v.name {
	case "fill-column":
		return display.Active.FillCol
	case "c-indent":
		return State.CIndent
	case "c-brace":
		return State.CBrace
	case "c-colon-offset":
		return State.CColonOffset
	case "py-indent":
		return State.PyIndent
	case "py-continued-offset":
		return State.PyContinuedOffset
	case "whitespace-cleanup":
		return boolToInt(State.WhitespaceCleanup)
	default:
		return v.read(nil)
	}
}

func varStorageRead(v *variable, buf *buffer.Buffer) int {
	if v.local && buf != nil {
		return v.read(buf)
	}
	return varGlobalRead(v)
}

func varStorageWrite(v *variable, buf *buffer.Buffer, value int, runOnChange bool) bool {
	if value < v.min || value > v.max {
		return false
	}
	if v.local && buf != nil {
		v.write(buf, value)
	} else {
		varGlobalWrite(v, value)
	}
	if runOnChange && v.onChange != nil {
		v.onChange()
	}
	return true
}

func parseNumericText(text string) (int, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		n, err := strconv.ParseInt(text[2:], 16, 32)
		if err != nil || n < 0 {
			return 0, false
		}
		return int(n), true
	}
	n, err := strconv.ParseInt(text, 10, 32)
	if err != nil || n < 0 {
		return 0, false
	}
	return int(n), true
}

func varSetFromText(v *variable, text string) bool {
	parsed, ok := parseNumericText(text)
	if !ok {
		return false
	}
	return varStorageWrite(v, nil, parsed, true)
}

func varSetFromJSON(v *variable, raw json.RawMessage) bool {
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		if num < 0 || num != float64(int(num)) {
			return false
		}
		return varSetFromText(v, strconv.FormatInt(int64(num), 10))
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return false
	}
	return varSetFromText(v, s)
}

// VarsFromJSON applies variable settings from a JSON object (hyphenated keys).
func VarsFromJSON(config map[string]json.RawMessage) {
	VarsInit()
	if config == nil {
		return
	}
	for i := range varTable {
		v := &varTable[i]
		raw, ok := config[v.name]
		if !ok {
			continue
		}
		if !varSetFromJSON(v, raw) {
			fmt.Fprintf(os.Stderr, "jem: ignoring invalid config value for %s\n", v.name)
		}
	}
	configThemeChanged()
	configSearchScopeChanged()
}

func varFindByName(name string) *variable {
	for i := range varTable {
		if varTable[i].name == name {
			return &varTable[i]
		}
	}
	return nil
}

func varTableProvider(ctx any, idx int) []byte {
	_ = ctx
	if int(idx) >= len(varTable) {
		return nil
	}
	return []byte(varTable[idx].name)
}

func varFormat(v *variable, buf *buffer.Buffer) string {
	return strconv.Itoa(varStorageRead(v, buf))
}

// CmdSetVariable interactively sets a named editor variable.
func CmdSetVariable(f bool, n int) bool {
	_ = f
	_ = n
	buf := buffer.All.Current
	AskFuzzy("Set variable: ", varTableProvider, nil, len(varTable), func(name string, pr minibuffer.PromptResult) {
		if pr == minibuffer.PromptResultAbort {
			CmdAbort(false, 1)
			return
		}
		if pr != minibuffer.PromptResultYes {
			return
		}
		v := varFindByName(name)
		if v == nil {
			return
		}
		current := varFormat(v, buf)
		prompt := fmt.Sprintf("Set %s (current %s): ", v.name, current)
		AskStringCap(prompt, "", 64, func(response string, pr minibuffer.PromptResult) {
			if pr != minibuffer.PromptResultYes {
				return
			}
			parsed, ok := parseNumericText(response)
			if !ok || !varStorageWrite(v, buf, parsed, !v.local) {
				display.MBWrite("[invalid value for %s]", v.name)
				return
			}
			display.MBWrite("[%s = %s]", v.name, varFormat(v, buf))
		})
	})
	return true
}

// CmdDescribeVariable shows a variable's value and documentation.
func CmdDescribeVariable(f bool, n int) bool {
	_ = f
	_ = n
	AskFuzzy("Describe variable: ", varTableProvider, nil, len(varTable), func(name string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		v := varFindByName(name)
		if v == nil {
			display.MBWrite("[unknown variable: %s]", name)
			return
		}
		value := varFormat(v, buffer.All.Current)
		display.MBWrite("[%s = %s] %s", v.name, value, v.doc)
	})
	return true
}

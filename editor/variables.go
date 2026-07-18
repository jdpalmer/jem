package editor

import (
	"encoding/json"
	"fmt"
	"github.com/jdpalmer/jem/app"
	"os"
	"strconv"
	"strings"
)

// variables.go — editor variables (translation of src/variables.c)

type variable struct {
	name     string
	doc      string
	min      uint32
	max      uint32
	local    bool
	read     func(bp *Buffer) uint32
	write    func(bp *Buffer, value uint32)
	onChange func()
}

var varTable = []variable{
	{
		name: "fill-column",
		doc:  "Wrap/fill column used by paragraph filling.",
		min:  0, max: 1000, local: true,
		read:  func(bp *Buffer) uint32 { return bp.FillCol },
		write: func(bp *Buffer, v uint32) { bp.FillCol = v },
	},
	{
		name: "theme-mode",
		doc:  "Editor palette mode: 0 dark, 1 light.",
		min:  0, max: uint32(ThemeLight), local: false,
		read: func(bp *Buffer) uint32 {
			_ = bp
			return uint32(app.State.Theme.Mode)
		},
		write: func(bp *Buffer, v uint32) {
			_ = bp
			app.State.Theme.Mode = ThemeMode(v)
		},
		onChange: configThemeChanged,
	},
	{
		name: "search-scope",
		doc:  "Search scope: 0 current buffer, 1 all buffers.",
		min:  0, max: uint32(SearchScopeAllBuffers), local: false,
		read: func(bp *Buffer) uint32 {
			_ = bp
			return uint32(app.State.SearchScopeSetting)
		},
		write: func(bp *Buffer, v uint32) {
			_ = bp
			app.State.SearchScopeSetting = SearchScopeMode(v)
		},
		onChange: configSearchScopeChanged,
	},
	{
		name: "whitespace-cleanup",
		doc:  "Trim trailing whitespace from every line before saving: 0 off, 1 on.",
		min:  0, max: 1, local: true,
		read:  func(bp *Buffer) uint32 { return boolToU32(bp.WhitespaceCleanup) },
		write: func(bp *Buffer, v uint32) { bp.WhitespaceCleanup = v != 0 },
	},
	{
		name: "startup-quote",
		doc:  "Show a startup quote in the message line on launch: 0 off, 1 on.",
		min:  0, max: 1, local: false,
		read: func(bp *Buffer) uint32 {
			_ = bp
			return boolToU32(app.State.StartupQuote)
		},
		write: func(bp *Buffer, v uint32) {
			_ = bp
			app.State.StartupQuote = v != 0
		},
	},
	{
		name: "auto-revert-mode",
		doc:  "Reload buffers from disk when the file changes externally: 0 prompt if modified, 1 always reload.",
		min:  0, max: 1, local: false,
		read: func(bp *Buffer) uint32 {
			_ = bp
			return boolToU32(app.State.AutoRevertMode)
		},
		write: func(bp *Buffer, v uint32) {
			_ = bp
			app.State.AutoRevertMode = v != 0
		},
	},
	{
		name: "c-indent",
		doc:  "C-family block indent width in spaces.",
		min:  0, max: 32, local: true,
		read:  func(bp *Buffer) uint32 { return bp.CIndent },
		write: func(bp *Buffer, v uint32) { bp.CIndent = v },
	},
	{
		name: "c-brace",
		doc:  "Extra indent for a standalone opening brace line in C-like modes.",
		min:  0, max: 32, local: true,
		read:  func(bp *Buffer) uint32 { return bp.CBrace },
		write: func(bp *Buffer, v uint32) { bp.CBrace = v },
	},
	{
		name: "c-colon-offset",
		doc:  "Extra offset applied to C-family case/default labels.",
		min:  0, max: 32, local: true,
		read:  func(bp *Buffer) uint32 { return bp.CColonOffset },
		write: func(bp *Buffer, v uint32) { bp.CColonOffset = v },
	},
	{
		name: "py-indent",
		doc:  "Python block indent width in spaces.",
		min:  0, max: 32, local: true,
		read:  func(bp *Buffer) uint32 { return bp.PyIndent },
		write: func(bp *Buffer, v uint32) { bp.PyIndent = v },
	},
	{
		name: "py-continued-offset",
		doc:  "Extra indent for explicit Python continuation lines.",
		min:  0, max: 32, local: true,
		read:  func(bp *Buffer) uint32 { return bp.PyContinuedOffset },
		write: func(bp *Buffer, v uint32) { bp.PyContinuedOffset = v },
	},
}

func boolToU32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

func configThemeChanged() {
	themeUpdate()
	syncSyntaxPalette()
	for i := 0; i < int(app.State.WindowCount); i++ {
		wp := app.State.WINDOWS[i]
		if wp != nil {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
	}
}

func configSearchScopeChanged() {
	for i := 0; i < int(app.State.WindowCount); i++ {
		wp := app.State.WINDOWS[i]
		if wp != nil {
			wp.ShouldUpdateModeLine = true
		}
	}
}

// VarsInit resets all editor variables to their defaults.
func VarsInit() {
	app.State.FillCol = 80
	app.State.Theme.Mode = ThemeDark
	app.State.SearchScopeSetting = SearchScopeBuffer
	app.State.WhitespaceCleanup = true
	app.State.StartupQuote = true
	app.State.AutoRevertMode = false
	app.State.CIndent = 2
	app.State.CBrace = 0
	app.State.CColonOffset = 0
	app.State.PyIndent = 4
	app.State.PyContinuedOffset = 4
	configThemeChanged()
	configSearchScopeChanged()
}

func bufferApplyVarDefaults(bp *Buffer) {
	if bp == nil {
		return
	}
	bp.FillCol = app.State.FillCol
	bp.CIndent = app.State.CIndent
	bp.CBrace = app.State.CBrace
	bp.CColonOffset = app.State.CColonOffset
	bp.PyIndent = app.State.PyIndent
	bp.PyContinuedOffset = app.State.PyContinuedOffset
	bp.WhitespaceCleanup = app.State.WhitespaceCleanup
}

func varGlobalWrite(v *variable, value uint32) {
	switch v.name {
	case "fill-column":
		app.State.FillCol = value
	case "c-indent":
		app.State.CIndent = value
	case "c-brace":
		app.State.CBrace = value
	case "c-colon-offset":
		app.State.CColonOffset = value
	case "py-indent":
		app.State.PyIndent = value
	case "py-continued-offset":
		app.State.PyContinuedOffset = value
	case "whitespace-cleanup":
		app.State.WhitespaceCleanup = value != 0
	default:
		v.write(nil, value)
	}
}

func varGlobalRead(v *variable) uint32 {
	switch v.name {
	case "fill-column":
		return app.State.FillCol
	case "c-indent":
		return app.State.CIndent
	case "c-brace":
		return app.State.CBrace
	case "c-colon-offset":
		return app.State.CColonOffset
	case "py-indent":
		return app.State.PyIndent
	case "py-continued-offset":
		return app.State.PyContinuedOffset
	case "whitespace-cleanup":
		return boolToU32(app.State.WhitespaceCleanup)
	default:
		return v.read(nil)
	}
}

func varStorageRead(v *variable, bp *Buffer) uint32 {
	if v.local && bp != nil {
		return v.read(bp)
	}
	return varGlobalRead(v)
}

func varStorageWrite(v *variable, bp *Buffer, value uint32, runOnChange bool) bool {
	if value < v.min || value > v.max {
		return false
	}
	if v.local && bp != nil {
		v.write(bp, value)
	} else {
		varGlobalWrite(v, value)
	}
	if runOnChange && v.onChange != nil {
		v.onChange()
	}
	return true
}

func parseNumericText(text string) (uint32, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		n, err := strconv.ParseUint(text[2:], 16, 32)
		if err != nil {
			return 0, false
		}
		return uint32(n), true
	}
	n, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
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
		if num < 0 || num != float64(uint32(num)) {
			return false
		}
		return varSetFromText(v, strconv.FormatUint(uint64(num), 10))
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

func varTableProvider(ctx any, idx uint) []byte {
	_ = ctx
	if int(idx) >= len(varTable) {
		return nil
	}
	return []byte(varTable[idx].name)
}

func varFormat(v *variable, bp *Buffer) string {
	return strconv.FormatUint(uint64(varStorageRead(v, bp)), 10)
}

// CmdSetVariable interactively sets a named editor variable.
func CmdSetVariable(f bool, n int) bool {
	_ = f
	_ = n
	bp := app.State.CurrentBuffer
	name, pr := mbReadFuzzyListString("Set variable: ", varTableProvider, nil, uint(len(varTable)))
	if pr == PromptResultAbort {
		CmdAbort(false, 1)
		return false
	}
	if pr != PromptResultYes {
		return false
	}
	v := varFindByName(name)
	if v == nil {
		return false
	}
	current := varFormat(v, bp)
	prompt := fmt.Sprintf("Set %s (current %s): ", v.name, current)
	response, pr := mbReadStringCap(prompt, "", 64)
	if pr != PromptResultYes {
		return true
	}
	parsed, ok := parseNumericText(response)
	if !ok || !varStorageWrite(v, bp, parsed, !v.local) {
		mbWrite("[invalid value for %s]", v.name)
		return false
	}
	mbWrite("[%s = %s]", v.name, varFormat(v, bp))
	return true
}

// CmdDescribeVariable shows a variable's value and documentation.
func CmdDescribeVariable(f bool, n int) bool {
	_ = f
	_ = n
	name, pr := mbReadFuzzyListString("Describe variable: ", varTableProvider, nil, uint(len(varTable)))
	if pr != PromptResultYes {
		return false
	}
	v := varFindByName(name)
	if v == nil {
		mbWrite("[unknown variable: %s]", name)
		return false
	}
	value := varFormat(v, app.State.CurrentBuffer)
	mbWrite("[%s = %s] %s", v.name, value, v.doc)
	return true
}

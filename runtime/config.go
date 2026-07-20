package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/term"
)

const keyCodeSpace uint32 = 0x40000000

// ConfigLoad reads ~/.jem.json (if present) and applies settings.
func ConfigLoad() {
	VarsInit()

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := filepath.Join(home, ".jem.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "jem: invalid %s: %v\n", path, err)
		return
	}

	VarsFromJSON(raw)
	display.ThemeUpdate()
	keybindingsFromJSON(raw)
}

func keybindingsFromJSON(raw map[string]json.RawMessage) {
	kbRaw, ok := raw["keybindings"]
	if !ok {
		return
	}
	var kb map[string]*json.RawMessage
	if err := json.Unmarshal(kbRaw, &kb); err != nil {
		display.MBWrite("invalid JSON keybindings object")
		return
	}

	for key, valPtr := range kb {
		code, ok := parseKeySequence(strings.TrimSpace(key))
		if !ok {
			display.MBWrite("invalid JSON keybinding code: %s", key)
			continue
		}

		if valPtr == nil {
			delete(keybindingsMap, code)
			continue
		}
		var v string
		if err := json.Unmarshal(*valPtr, &v); err != nil {
			display.MBWrite("invalid JSON keybinding value for %s", key)
			continue
		}
		cmdName := strings.ToLower(v)
		if cmdFn, ok := commandNameMap[cmdName]; ok {
			keybindingsMap[code] = cmdFn
		} else {
			display.MBWrite("invalid JSON command name: %s", v)
		}
	}
}

// parseKeySequence implements the same parsing rules as the original C parser.
func parseKeySequence(s string) (uint32, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	idx := strings.Index(s, " ")
	if idx == -1 {
		prefix, ok := parseKeyChord(s)
		if !ok {
			return 0, false
		}
		return prefix, true
	}
	prefixStr := strings.TrimSpace(s[:idx])
	suffixStr := strings.TrimSpace(s[idx+1:])
	if prefixStr == "" || suffixStr == "" {
		return 0, false
	}
	prefix, ok := parseKeyChord(prefixStr)
	if !ok {
		return 0, false
	}
	if prefix != (term.CTL | uint32('X')) {
		return 0, false
	}
	suffix, ok := parseKeyChord(suffixStr)
	if !ok {
		return 0, false
	}
	return term.CTLX | suffix, true
}

func parseKeyChord(s string) (uint32, bool) {
	pos := 0
	var code uint32 = 0
	s = strings.TrimSpace(s)
	for pos < len(s) {
		if strings.HasPrefix(strings.ToUpper(s[pos:]), "M-") {
			if (code & term.META) != 0 {
				return 0, false
			}
			code |= term.META
			pos += 2
			continue
		}
		if strings.HasPrefix(strings.ToUpper(s[pos:]), "C-") {
			if (code & term.CTL) != 0 {
				return 0, false
			}
			code |= term.CTL
			pos += 2
			continue
		}
		if strings.HasPrefix(strings.ToUpper(s[pos:]), "S-") {
			if (code & term.SHIFT) != 0 {
				return 0, false
			}
			code |= term.SHIFT
			pos += 2
			continue
		}
		break
	}
	token := strings.ToUpper(strings.TrimSpace(s[pos:]))
	if token == "TAB" {
		code |= term.KeyTab
		return code, true
	}
	if token == "ENTER" {
		code |= term.KeyEnter
		return code, true
	}
	if token == "SPACE" {
		code |= ' '
		return code, true
	}
	if token == "BACKSPACE" {
		code |= 0x7F
		return code, true
	}
	if token == "UP" {
		code |= term.KeyUp
		return code, true
	}
	if token == "DOWN" {
		code |= term.KeyDown
		return code, true
	}
	if token == "LEFT" {
		code |= term.KeyLeft
		return code, true
	}
	if token == "RIGHT" {
		code |= term.KeyRight
		return code, true
	}
	if token == "HOME" {
		code |= term.KeyHome
		return code, true
	}
	if token == "END" {
		code |= term.KeyEnd
		return code, true
	}
	if token == "PAGEUP" || token == "PGUP" {
		code |= term.KeyPageUp
		return code, true
	}
	if token == "PAGEDOWN" || token == "PGDOWN" {
		code |= term.KeyPageDown
		return code, true
	}
	if token == "DELETE" || token == "DEL" {
		code |= term.KeyDelete
		return code, true
	}
	if len(token) > 0 {
		b := s[pos]
		if (code&(term.CTL|term.META)) != 0 && b >= 'a' && b <= 'z' {
			b = b - ('a' - 'A')
		}
		code |= uint32(b)
		return code, true
	}
	return 0, false
}

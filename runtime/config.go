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
// formatKeySequence renders a key code as an Emacs-style chord (e.g. "C-x C-f", "M-x").
func formatKeySequence(code uint32) string {
	if code&term.CTLX != 0 {
		return "C-x " + formatKeyChord(code&^term.CTLX)
	}
	return formatKeyChord(code)
}

func formatKeyChord(code uint32) string {
	var b strings.Builder
	if code&term.META != 0 {
		b.WriteString("M-")
	}
	if code&term.CTL != 0 {
		b.WriteString("C-")
	}
	if code&term.SHIFT != 0 {
		b.WriteString("S-")
	}
	base := code &^ term.KeyMask
	switch base {
	case term.KeyTab:
		b.WriteString("TAB")
	case term.KeyEnter:
		b.WriteString("ENTER")
	case ' ':
		b.WriteString("SPC")
	case 0x7F:
		b.WriteString("BACKSPACE")
	case term.KeyUp:
		b.WriteString("UP")
	case term.KeyDown:
		b.WriteString("DOWN")
	case term.KeyLeft:
		b.WriteString("LEFT")
	case term.KeyRight:
		b.WriteString("RIGHT")
	case term.KeyHome:
		b.WriteString("HOME")
	case term.KeyEnd:
		b.WriteString("END")
	case term.KeyPageUp:
		b.WriteString("PGUP")
	case term.KeyPageDown:
		b.WriteString("PGDOWN")
	case term.KeyDelete:
		b.WriteString("DEL")
	default:
		if base < 0x80 {
			ch := byte(base)
			if ch >= 'A' && ch <= 'Z' {
				ch = ch - 'A' + 'a'
			}
			b.WriteByte(ch)
		} else {
			b.WriteRune(rune(base))
		}
	}
	return b.String()
}

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

var namedKeys = map[string]uint32{
	"TAB":       term.KeyTab,
	"ENTER":     term.KeyEnter,
	"SPACE":     ' ',
	"BACKSPACE": 0x7F,
	"UP":        term.KeyUp,
	"DOWN":      term.KeyDown,
	"LEFT":      term.KeyLeft,
	"RIGHT":     term.KeyRight,
	"HOME":      term.KeyHome,
	"END":       term.KeyEnd,
	"PAGEUP":    term.KeyPageUp,
	"PGUP":      term.KeyPageUp,
	"PAGEDOWN":  term.KeyPageDown,
	"PGDOWN":    term.KeyPageDown,
	"DELETE":    term.KeyDelete,
	"DEL":       term.KeyDelete,
}

func parseKeyChord(s string) (uint32, bool) {
	pos := 0
	var code uint32 = 0
	s = strings.TrimSpace(s)
	for pos < len(s) {
		rest := strings.ToUpper(s[pos:])
		if strings.HasPrefix(rest, "M-") {
			if (code & term.META) != 0 {
				return 0, false
			}
			code |= term.META
			pos += 2
			continue
		}
		if strings.HasPrefix(rest, "C-") {
			if (code & term.CTL) != 0 {
				return 0, false
			}
			code |= term.CTL
			pos += 2
			continue
		}
		if strings.HasPrefix(rest, "S-") {
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
	if key, ok := namedKeys[token]; ok {
		return code | key, true
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

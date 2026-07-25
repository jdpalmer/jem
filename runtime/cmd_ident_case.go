package runtime

import (
	"strings"
	"unicode"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/window"
)

// Identifier case conversion (camelCase, PascalCase, snake_case, CONSTANT_CASE).

type identCaseStyle int

const (
	identCamel identCaseStyle = iota
	identPascal
	identSnake
	identConstant
)

// identBoundsAtPoint returns the [A-Za-z0-9_]+ span under point.
// If point sits just past an identifier, the preceding identifier is used.
func identBoundsAtPoint(buf *buffer.Buffer, loc buffer.Location) (start, end buffer.Location, ok bool) {
	line := buf.Line(loc.Line)
	if line == nil {
		return loc, loc, false
	}
	off := loc.Offset
	if off > len(line.Data) {
		off = len(line.Data)
	}
	if off > 0 && (off >= len(line.Data) || !isWordChar(line.Data[off])) && isWordChar(line.Data[off-1]) {
		off--
	}
	if off >= len(line.Data) || !isWordChar(line.Data[off]) {
		return loc, loc, false
	}
	startOff := off
	for startOff > 0 && isWordChar(line.Data[startOff-1]) {
		startOff--
	}
	endOff := off + 1
	for endOff < len(line.Data) && isWordChar(line.Data[endOff]) {
		endOff++
	}
	return buffer.Location{Line: loc.Line, Offset: startOff},
		buffer.Location{Line: loc.Line, Offset: endOff}, true
}

// splitIdentParts breaks an identifier into lowercased word parts.
// Splits on '_' / '-' and on simple camelCase / acronym boundaries.
func splitIdentParts(s string) []string {
	if s == "" {
		return nil
	}
	runes := []rune(s)
	var parts []string
	var cur []rune
	flush := func() {
		if len(cur) == 0 {
			return
		}
		parts = append(parts, strings.ToLower(string(cur)))
		cur = cur[:0]
	}
	for i, r := range runes {
		if r == '_' || r == '-' {
			flush()
			continue
		}
		if i > 0 && unicode.IsLetter(r) && unicode.IsUpper(r) {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
	return parts
}

func titlePart(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}

func joinIdentParts(parts []string, style identCaseStyle) string {
	if len(parts) == 0 {
		return ""
	}
	switch style {
	case identCamel:
		var b strings.Builder
		b.WriteString(strings.ToLower(parts[0]))
		for _, p := range parts[1:] {
			b.WriteString(titlePart(p))
		}
		return b.String()
	case identPascal:
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(titlePart(p))
		}
		return b.String()
	case identSnake:
		return strings.Join(parts, "_")
	case identConstant:
		upper := make([]string, len(parts))
		for i, p := range parts {
			upper[i] = strings.ToUpper(p)
		}
		return strings.Join(upper, "_")
	default:
		return strings.Join(parts, "_")
	}
}

func convertIdent(s string, style identCaseStyle) string {
	parts := splitIdentParts(s)
	if len(parts) == 0 {
		return s
	}
	return joinIdentParts(parts, style)
}

func cmdConvertIdentAtPoint(style identCaseStyle) bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}
	start, end, ok := identBoundsAtPoint(buf, win.Cursor)
	if !ok {
		return false
	}
	text := buf.GetText(start, end)
	if len(text) == 0 {
		return false
	}
	converted := convertIdent(string(text), style)
	if converted == string(text) {
		return true
	}
	BeginCommand()
	defer EndCommand()
	var newEnd buffer.Location
	if !bufferSetText(buf, start, end, []byte(converted), &newEnd, false) {
		return false
	}
	win.Cursor = newEnd
	win.DidEdit = true
	return true
}

// CmdCamelCase converts the identifier at point to camelCase.
func CmdCamelCase(f bool, n int) bool {
	_ = f
	_ = n
	return cmdConvertIdentAtPoint(identCamel)
}

// CmdPascalCase converts the identifier at point to PascalCase.
func CmdPascalCase(f bool, n int) bool {
	_ = f
	_ = n
	return cmdConvertIdentAtPoint(identPascal)
}

// CmdSnakeCase converts the identifier at point to snake_case.
func CmdSnakeCase(f bool, n int) bool {
	_ = f
	_ = n
	return cmdConvertIdentAtPoint(identSnake)
}

// CmdConstantCase converts the identifier at point to CONSTANT_CASE.
func CmdConstantCase(f bool, n int) bool {
	_ = f
	_ = n
	return cmdConvertIdentAtPoint(identConstant)
}

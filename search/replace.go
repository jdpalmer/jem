package search

import (
	"bytes"
	"unicode"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/window"
)

type matchCase int

const (
	matchCaseLower matchCase = iota
	matchCaseUpper
	matchCaseCapitalized
)

func markMatchStart(win *window.Window, patLen int) {
	win.Mark = win.Cursor.RewindBytes(win.Buffer, patLen)
}

func checkMatchCase(win *window.Window, patLen int) matchCase {
	line := win.Buffer.Line(win.Cursor.Line)
	if line == nil || win.Cursor.Offset < patLen {
		return matchCaseLower
	}
	start := win.Cursor.Offset - patLen
	text := line.Data[start : start+patLen]
	if len(text) == 0 || !unicode.IsUpper(rune(text[0])) {
		return matchCaseLower
	}
	for i := 1; i < len(text); i++ {
		if unicode.IsLower(rune(text[i])) {
			return matchCaseCapitalized
		}
	}
	return matchCaseUpper
}

func applyMatchCase(mc matchCase, repl []byte, out []byte) int {
	n := len(repl)
	if n > len(out) {
		n = len(out)
	}
	copy(out, repl[:n])
	switch mc {
	case matchCaseUpper:
		for i := 0; i < n; i++ {
			out[i] = byte(unicode.ToUpper(rune(out[i])))
		}
	case matchCaseCapitalized:
		if n > 0 {
			out[0] = byte(unicode.ToUpper(rune(out[0])))
		}
	}
	return n
}

func doReplace(win *window.Window, patLen int, repl []byte) bool {
	end := win.Cursor
	begin := end.RewindBytes(win.Buffer, patLen)
	return window.SetText(win.Buffer, begin, end, repl, nil) == nil
}

func doReplacePreservingCase(win *window.Window, patLen int, repl []byte, preserve bool) bool {
	if preserve {
		mc := checkMatchCase(win, patLen)
		if mc != matchCaseLower {
			var caseRepl [display.PatternCapacity]byte
			n := applyMatchCase(mc, repl, caseRepl[:])
			return doReplace(win, patLen, caseRepl[:n])
		}
	}
	return doReplace(win, patLen, repl)
}

func doReplaceRange(win *window.Window, start, end buffer.Location, repl []byte) bool {
	return window.SetText(win.Buffer, start, end, repl, nil) == nil
}

func writeReplacePrompt(buf *buffer.Buffer, from, to string) {
	prompt := ""
	if searchScopeIsAllBuffers() && buf != nil {
		prompt = "[" + buf.Name + "] "
	}
	prompt += "replace '" + from + "' with '" + to + "' (y/n/!/+/q): "
	display.MBWrite("%s", prompt)
}

func expandRegexReplacement(repl string, match RegexMatch) ([]byte, error) {
	var out bytes.Buffer
	text := match.Text
	indices := match.Index
	for i := 0; i < len(repl); i++ {
		if repl[i] == '\\' && i+1 < len(repl) {
			esc := repl[i+1]
			i++
			if esc >= '0' && esc <= '9' {
				group := int(esc - '0')
				start, end := -1, -1
				if group*2+1 < len(indices) {
					start = indices[group*2]
					end = indices[group*2+1]
				}
				if start >= 0 && end >= start && end <= len(text) {
					out.Write(text[start:end])
				}
				continue
			}
			if esc == 'n' {
				out.WriteByte('\n')
				continue
			}
			out.WriteByte(esc)
			continue
		}
		out.WriteByte(repl[i])
	}
	return out.Bytes(), nil
}

// SearchForward runs a forward search using DefaultState.SearchPattern.
func SearchForward() bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil {
		return false
	}
	pat := searchPatternBytes()
	if len(pat) == 0 {
		return true
	}
	scope := searchScopeInit(buf)
	if !findNextInScope(win, &scope, pat) {
		display.MBWrite("[not found]")
	}
	return true
}

// SearchBackward runs a backward search using DefaultState.SearchPattern.
func SearchBackward() bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil {
		return false
	}
	pat := searchPatternBytes()
	if len(pat) == 0 {
		return true
	}
	scope := searchScopeInit(buf)
	if !findPrevInScope(win, &scope, pat) {
		display.MBWrite("[not found]")
	}
	return true
}

// ToggleSearchScope switches the search scope between current buffer and all buffers.
func ToggleSearchScope() bool {
	if DefaultState.SearchScopeSetting == SearchScopeBuffer {
		DefaultState.SearchScopeSetting = SearchScopeAllBuffers
	} else {
		DefaultState.SearchScopeSetting = SearchScopeBuffer
	}
	if searchScopeIsAllBuffers() {
		display.MBWrite("[search scope: all buffers]")
	} else {
		display.MBWrite("[search scope: current buffer]")
	}
	return true
}

// StartQueryReplace begins interactive query-replace for the current search pattern.
func StartQueryReplace(repl string) KeySession {
	buf := buffer.All.Current
	if buf == nil {
		return nil
	}
	pat := searchPatternBytes()
	patLen := len(pat)
	if patLen == 0 {
		return nil
	}
	preserveCase := !DefaultState.SearchCaseSensitive
	return newQueryReplaceSession(buf, []byte(repl), pat, patLen, preserveCase)
}

// StartQueryReReplace begins interactive regex query-replace.
func StartQueryReReplace(pattern, replStr string) KeySession {
	buf := buffer.All.Current
	if buf == nil || pattern == "" {
		return nil
	}
	return newQueryReReplaceSession(buf, pattern, replStr)
}

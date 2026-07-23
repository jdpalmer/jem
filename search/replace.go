package search

import (
	"bytes"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
	"unicode"

	"github.com/jdpalmer/jem/buffer"
)

type matchCase int

const (
	matchCaseLower matchCase = iota
	matchCaseUpper
	matchCaseCapitalized
)

func markMatchStart(win *window.Window, patLen int) {
	lineNum := win.Cursor.Line
	off := win.Cursor.Offset
	for i := 0; i < patLen; i++ {
		if off > 0 {
			off--
		} else if lineNum > 1 {
			lineNum--
			line := win.Buffer.Line(lineNum)
			if line != nil {
				off = line.Len()
			}
		}
	}
	win.Mark = buffer.Location{Line: lineNum, Offset: off}
}

func markMatchLocation(win *window.Window, start buffer.Location) {
	win.Mark = start
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
	if n >= len(out) {
		n = len(out) - 1
	}
	copy(out, repl[:n])
	out[n] = 0
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
	return setText(win.Buffer, begin, end, repl, nil) == nil
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
	return setText(win.Buffer, start, end, repl, nil) == nil
}

func writeReplacePrompt(buf *buffer.Buffer, from, to string) {
	prompt := ""
	if searchScopeIsAllBuffers() && buf != nil {
		prompt = "[" + buf.Name + "] "
	}
	prompt += "replace '" + from + "' with '" + to + "' (y/n/!/+/q): "
	mbWrite("%s", prompt)
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

// SearchForward prompts for a pattern and searches forward in the buffer.
func SearchForward() bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil {
		return false
	}
	readPattern("Search", func(pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		pat := searchPatternBytes()
		if len(pat) == 0 {
			return
		}
		scope := searchScopeInit(buf)
		if !findNextInScope(win, &scope, pat) {
			mbWrite("[not found]")
		}
	})
	return true
}

// SearchBackward prompts for a pattern and searches backward in the buffer.
func SearchBackward() bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil {
		return false
	}
	readPattern("Reverse search", func(pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		pat := searchPatternBytes()
		if len(pat) == 0 {
			return
		}
		scope := searchScopeInit(buf)
		if !findPrevInScope(win, &scope, pat) {
			mbWrite("[not found]")
		}
	})
	return true
}

// ToggleSearchScope switches the search scope between current buffer and all buffers.
func ToggleSearchScope() bool {
	if currentState().SearchScopeSetting == SearchScopeBuffer {
		currentState().SearchScopeSetting = SearchScopeAllBuffers
	} else {
		currentState().SearchScopeSetting = SearchScopeBuffer
	}
	if searchScopeIsAllBuffers() {
		mbWrite("[search scope: all buffers]")
	} else {
		mbWrite("[search scope: current buffer]")
	}
	return true
}

// QueryReplace prompts for a pattern and replacement, then asks before each replacement.
func QueryReplace() bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil {
		return false
	}
	readPattern("replace", func(pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		pat := searchPatternBytes()
		patLen := len(pat)
		if patLen == 0 {
			return
		}
		preserveCase := !currentState().SearchCaseSensitive
		askString("Replace '"+string(pat)+"' with: ", "", func(repl string, pr minibuffer.PromptResult) {
			if pr == minibuffer.PromptResultAbort {
				return
			}
			startQueryReplace(newQueryReplaceSession(buf, []byte(repl), pat, patLen, preserveCase))
		})
	})
	return true
}

// QueryReReplace prompts for a regex pattern and replacement with backreference support, then asks before each replacement.
func QueryReReplace() bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil {
		return false
	}
	askString(buildSearchPrompt("Query re-replace"), currentState().SearchPattern, func(pattern string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes || pattern == "" {
			return
		}
		askString("Replace '"+pattern+"' with (\\0..\\9): ", "", func(replStr string, pr minibuffer.PromptResult) {
			if pr == minibuffer.PromptResultAbort {
				return
			}
			startQueryReplace(newQueryReReplaceSession(buf, pattern, replStr))
		})
	})
	return true
}

func startQueryReplace(s *queryReplaceSession) {
	pushKeySession(s)
}

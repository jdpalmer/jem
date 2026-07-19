package search

import (
	"bytes"
	"unicode"

	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
)

type matchCase int

const (
	matchCaseLower matchCase = iota
	matchCaseUpper
	matchCaseCapitalized
)

func markMatchStart(wp *model.Window, patLen int) {
	if wp == nil || wp.Buffer == nil || patLen == 0 {
		return
	}
	line := wp.Cursor.Line
	off := wp.Cursor.Offset
	for i := 0; i < patLen; i++ {
		if off > 0 {
			off--
		} else if line > 1 {
			line--
			lp := wp.Buffer.Line(line)
			if lp != nil {
				off = lp.Len()
			}
		}
	}
	wp.Mark = buffer.Location{Line: line, Offset: off}
}

func markMatchLocation(wp *model.Window, start buffer.Location) {
	if wp != nil {
		wp.Mark = start
	}
}

func checkMatchCase(wp *model.Window, patLen int) matchCase {
	if wp == nil || wp.Buffer == nil || patLen == 0 {
		return matchCaseLower
	}
	lp := wp.Buffer.Line(wp.Cursor.Line)
	if lp == nil || wp.Cursor.Offset < uint(patLen) {
		return matchCaseLower
	}
	start := int(wp.Cursor.Offset) - patLen
	text := lp.Data[start : start+patLen]
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

func doReplace(wp *model.Window, patLen int, repl []byte) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	end := wp.Cursor
	begin := end.RewindBytes(wp.Buffer, patLen)
	return setText(wp.Buffer, begin, end, repl, nil) == nil
}

func doReplacePreservingCase(wp *model.Window, patLen int, repl []byte, preserve bool) bool {
	if preserve {
		mc := checkMatchCase(wp, patLen)
		if mc != matchCaseLower {
			var caseRepl [model.PatternCapacity]byte
			n := applyMatchCase(mc, repl, caseRepl[:])
			return doReplace(wp, patLen, caseRepl[:n])
		}
	}
	return doReplace(wp, patLen, repl)
}

func doReplaceRange(wp *model.Window, start, end buffer.Location, repl []byte) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	return setText(wp.Buffer, start, end, repl, nil) == nil
}

func writeReplacePrompt(bp *buffer.Buffer, from, to string) {
	prompt := ""
	if searchScopeIsAllBuffers() && bp != nil {
		prompt = "[" + bp.Name + "] "
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

func SearchForward() bool {
	wp := model.State.CurrentWindow
	bp := model.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	readPattern("Search", func(pr model.PromptResult) {
		if pr != model.PromptResultYes {
			return
		}
		pat := searchPatternBytes()
		if len(pat) == 0 {
			return
		}
		scope := searchScopeInit(bp)
		if !findNextInScope(wp, &scope, pat) {
			mbWrite("[not found]")
		}
	})
	return true
}

func SearchBackward() bool {
	wp := model.State.CurrentWindow
	bp := model.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	readPattern("Reverse search", func(pr model.PromptResult) {
		if pr != model.PromptResultYes {
			return
		}
		pat := searchPatternBytes()
		if len(pat) == 0 {
			return
		}
		scope := searchScopeInit(bp)
		if !findPrevInScope(wp, &scope, pat) {
			mbWrite("[not found]")
		}
	})
	return true
}

func ToggleSearchScope() bool {
	if model.State.SearchScopeSetting == model.SearchScopeBuffer {
		model.State.SearchScopeSetting = model.SearchScopeAllBuffers
	} else {
		model.State.SearchScopeSetting = model.SearchScopeBuffer
	}
	if searchScopeIsAllBuffers() {
		mbWrite("[search scope: all buffers]")
	} else {
		mbWrite("[search scope: current buffer]")
	}
	return true
}

func QueryReplace() bool {
	wp := model.State.CurrentWindow
	bp := model.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	readPattern("replace", func(pr model.PromptResult) {
		if pr != model.PromptResultYes {
			return
		}
		pat := searchPatternBytes()
		patLen := len(pat)
		if patLen == 0 {
			return
		}
		preserveCase := !currentState().SearchCaseSensitive
		askString("Replace '"+string(pat)+"' with: ", "", func(repl string, pr model.PromptResult) {
			if pr == model.PromptResultAbort {
				return
			}
			startQueryReplace(newQueryReplaceSession(bp, []byte(repl), pat, patLen, preserveCase))
		})
	})
	return true
}

func QueryReReplace() bool {
	wp := model.State.CurrentWindow
	bp := model.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	askString(buildSearchPrompt("Query re-replace"), model.State.SearchPattern, func(pattern string, pr model.PromptResult) {
		if pr != model.PromptResultYes || pattern == "" {
			return
		}
		askString("Replace '"+pattern+"' with (\\0..\\9): ", "", func(replStr string, pr model.PromptResult) {
			if pr == model.PromptResultAbort {
				return
			}
			startQueryReplace(newQueryReReplaceSession(bp, pattern, replStr))
		})
	})
	return true
}

func startQueryReplace(s *queryReplaceSession) {
	if pushKeySession(s) {
		return
	}
	if s.Open() {
		s.Close()
		return
	}
	defer s.Close()
	for {
		k, ok := isearchReadKey()
		if !ok {
			return
		}
		if s.HandleKey(k) {
			return
		}
	}
}

package syntax

import (
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
)

// Delimiter navigation (translation of syntax_find_matching_delimiter in src/syntax.c).

// DelimiterPair maps a delimiter rune to its partner and scan direction.
func DelimiterPair(ch int) (open, close int, forward bool, ok bool) {
	di := delimiterIndex(ch)
	if di < 0 {
		return 0, 0, false, false
	}
	d := kDelims[di]
	return d.open, d.close, ch == d.open, true
}

// byteOffsetToRuneLimit returns how many leading runes end before byteOffset in
// lp.Data, matching syntax_process_line scan_maxn byte semantics in syntax.c.
func byteOffsetToRuneLimit(lp *buffer.Line, byteOffset uint) int {
	if lp == nil {
		return 0
	}
	data := lp.Data
	n := len(data)
	if byteOffset == 0 {
		return 0
	}
	if byteOffset >= uint(n) {
		buffer.EnsureLineCache(lp)
		return len(lp.RuneCache)
	}
	limit := 0
	i := 0
	for i < n {
		if i >= int(byteOffset) {
			break
		}
		_, size := utf8.DecodeRune(data[i:])
		if size == 0 {
			break
		}
		if i+size > int(byteOffset) {
			break
		}
		i += size
		limit++
	}
	return limit
}

func syntaxContextFromState(st *SynState) SyntaxContext {
	if st == nil {
		return buffer.SyntaxContextNone
	}
	switch st.DFA {
	case SS_STRING_D, SS_STRING_D_ESC, SS_STRING_S, SS_STRING_S_ESC:
		return buffer.SyntaxContextString
	case SS_CMT_LINE, SS_CMT_BLOCK, SS_CMT_STAR, SS_CMT_BRACE, SS_CMT_PAREN, SS_CMT_PAREN2,
		SS_LUA_BLOCK, SS_LUA_BLKEND, SS_HTML_CMT, SS_HTML_CMT_D1, SS_HTML_CMT_D2:
		return buffer.SyntaxContextComment
	case SS_PREPROC:
		return buffer.SyntaxContextPreproc
	default:
		return buffer.SyntaxContextCode
	}
}

func syntaxContextIsStructural(ctx SyntaxContext) bool {
	return ctx == buffer.SyntaxContextCode || ctx == buffer.SyntaxContextPreproc
}

func bufferSyntaxFindStart(bp *buffer.Buffer, lineNumber uint, st *SynState) {
	*st = SynState{DFA: SS_NORMAL}
	if bp == nil {
		return
	}
	info := langModeSpec(bp.LangMode)
	if info.SyntaxKind == ModeSyntaxNone ||
		info.SyntaxKind == ModeSyntaxMarkdown ||
		info.SyntaxKind == ModeSyntaxHashCommentOnly {
		return
	}
	syncLine := uint(1)
	if lineNumber > 1 {
		for q := lineNumber - 1; q > 0; q-- {
			lp := buffer.GetLine(bp, q)
			if lp != nil && lp.SyntaxValid {
				*st = lp.SyntaxEndState
				syncLine = q + 1
				break
			}
		}
	}
	for q := syncLine; q < lineNumber; q++ {
		lp := buffer.GetLine(bp, q)
		if lp == nil {
			continue
		}
		end, summary, styles := tokenizeLineFromState(lp, *st)
		lp.SyntaxEndState = end
		lp.SyntaxSummary = summary
		lp.SyntaxStyles = styles
		lp.SyntaxValid = true
	}
}

func syntaxStateAt(bp *buffer.Buffer, lineNumber, offset uint, st *SynState) bool {
	if bp == nil || lineNumber == 0 || lineNumber > buffer.EOF(bp) {
		*st = SynState{DFA: SS_NORMAL}
		return false
	}
	bufferSyntaxFindStart(bp, lineNumber, st)
	if lineNumber >= buffer.EOF(bp) {
		return true
	}
	lp := buffer.GetLine(bp, lineNumber)
	if lp == nil {
		return false
	}
	runeLimit := byteOffsetToRuneLimit(lp, offset)
	*st, _, _ = tokenizeLineFromStateLimit(lp, *st, runeLimit)
	return true
}

func syntaxCharIsStructural(bp *buffer.Buffer, lineNumber, offset uint) bool {
	var before, after SynState
	if !syntaxStateAt(bp, lineNumber, offset, &before) {
		return false
	}
	if !syntaxContextIsStructural(syntaxContextFromState(&before)) {
		return false
	}
	if !syntaxStateAt(bp, lineNumber, offset+1, &after) {
		return false
	}
	return syntaxContextIsStructural(syntaxContextFromState(&after))
}

func syntaxGetLineSummary(bp *buffer.Buffer, lineNumber uint, summaryOut *SyntaxLineSummary) bool {
	if bp == nil || lineNumber == 0 || lineNumber >= buffer.EOF(bp) {
		return false
	}
	lp := buffer.GetLine(bp, lineNumber)
	if lp == nil {
		return false
	}
	if !lp.SyntaxValid {
		var st SynState
		bufferSyntaxFindStart(bp, lineNumber, &st)
		end, summary, styles := tokenizeLineFromState(lp, st)
		lp.SyntaxEndState = end
		lp.SyntaxSummary = summary
		lp.SyntaxStyles = styles
		lp.SyntaxValid = true
	}
	*summaryOut = lp.SyntaxSummary
	return true
}

func syntaxFindMatchingDelimiter(bp *buffer.Buffer, start buffer.Location, matchOut *buffer.Location) bool {
	if bp == nil || start.Line == 0 || start.Line >= buffer.EOF(bp) {
		return false
	}
	lp := buffer.GetLine(bp, start.Line)
	if lp == nil || start.Offset >= buffer.LineLength(lp) {
		return false
	}
	ch := int(buffer.LineGetc(lp, start.Offset))
	open, close, forward, ok := DelimiterPair(ch)
	if !ok {
		return false
	}
	if !syntaxCharIsStructural(bp, start.Line, start.Offset) {
		return false
	}

	depth := 1
	if forward {
		lineNum := start.Line
		off := start.Offset + 1
		for lineNum <= bp.LineCount {
			line := buffer.GetLine(bp, lineNum)
			if line == nil {
				return false
			}
			for off < buffer.LineLength(line) {
				c := int(buffer.LineGetc(line, off))
				if (c == open || c == close) && syntaxCharIsStructural(bp, lineNum, off) {
					if c == open {
						depth++
					} else if depth--; depth == 0 {
						*matchOut = buffer.MakeLocation(lineNum, off)
						return true
					}
				}
				off++
			}
			lineNum++
			off = 0
		}
	} else {
		lineNum := start.Line
		off := int(start.Offset) - 1
		for lineNum >= 1 {
			line := buffer.GetLine(bp, lineNum)
			if line == nil {
				return false
			}
			for off >= 0 {
				c := int(buffer.LineGetc(line, uint(off)))
				if (c == open || c == close) && syntaxCharIsStructural(bp, lineNum, uint(off)) {
					if c == close {
						depth++
					} else if depth--; depth == 0 {
						*matchOut = buffer.MakeLocation(lineNum, uint(off))
						return true
					}
				}
				off--
			}
			if lineNum == 1 {
				break
			}
			lineNum--
			prev := buffer.GetLine(bp, lineNum)
			if prev == nil {
				return false
			}
			off = int(buffer.LineLength(prev)) - 1
		}
	}
	return false
}

func syntaxLocationHasDelimiter(bp *buffer.Buffer, loc buffer.Location) bool {
	if bp == nil || loc.Line == 0 || loc.Line >= buffer.EOF(bp) {
		return false
	}
	lp := buffer.GetLine(bp, loc.Line)
	if lp == nil || loc.Offset >= buffer.LineLength(lp) {
		return false
	}
	if _, _, _, ok := DelimiterPair(int(buffer.LineGetc(lp, loc.Offset))); !ok {
		return false
	}
	return syntaxCharIsStructural(bp, loc.Line, loc.Offset)
}

// FindMatchingDelimiter finds the partner delimiter for start.
func FindMatchingDelimiter(bp *buffer.Buffer, start buffer.Location, matchOut *buffer.Location) bool {
	return syntaxFindMatchingDelimiter(bp, start, matchOut)
}

// CharIsStructural reports whether offset on line is a structural delimiter char.
func CharIsStructural(bp *buffer.Buffer, lineNumber, offset uint) bool {
	return syntaxCharIsStructural(bp, lineNumber, offset)
}

// GetLineSummary ensures syntax summary for a line is computed.
func GetLineSummary(bp *buffer.Buffer, lineNumber uint, summaryOut *buffer.SyntaxLineSummary) bool {
	return syntaxGetLineSummary(bp, lineNumber, summaryOut)
}

package syntax

import (
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
)

// Delimiter matching and structural character navigation.

// DelimiterPair maps a delimiter rune to its partner and scan direction.
func DelimiterPair(ch int) (open, close int, forward bool, ok bool) {
	di := delimiterIndex(ch)
	if di < 0 {
		return 0, 0, false, false
	}
	d := kDelims[di]
	return d.open, d.close, ch == d.open, true
}

// byteOffsetToRuneLimit returns how many leading runes end before byteOffset in lp.Data.
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
		lp.EnsureCache()
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

func syntaxContextFromState(st *buffer.SynState) buffer.SyntaxContext {
	if st == nil {
		return buffer.SyntaxContextNone
	}
	switch st.DFA {
	case SynStateStringD, SynStateStringDEsc, SynStateStringS, SynStateStringSEsc:
		return buffer.SyntaxContextString
	case SynStateCmtLine, SynStateCmtBlock, SynStateCmtStar, SynStateCmtBrace, SynStateCmtParen, SynStateCmtParen2,
		SynStateLuaBlock, SynStateLuaBlkEnd, SynStateHTMLCmt, SynStateHTMLCmtD1, SynStateHTMLCmtD2:
		return buffer.SyntaxContextComment
	case SynStatePreproc:
		return buffer.SyntaxContextPreproc
	default:
		return buffer.SyntaxContextCode
	}
}

func syntaxContextIsStructural(ctx buffer.SyntaxContext) bool {
	return ctx == buffer.SyntaxContextCode || ctx == buffer.SyntaxContextPreproc
}

func bufferSyntaxFindStart(bp *buffer.Buffer, lineNumber uint, st *buffer.SynState) {
	*st = buffer.SynState{DFA: SynStateNormal}
	if bp == nil {
		return
	}
	info := For(bp.LangMode)
	if info.Kind == ModeSyntaxNone ||
		info.Kind == ModeSyntaxMarkdown ||
		info.Kind == ModeSyntaxHashCommentOnly {
		return
	}
	syncLine := uint(1)
	if lineNumber > 1 {
		for q := lineNumber - 1; q > 0; q-- {
			lp := bp.Line(q)
			if lp != nil && lp.SyntaxValid {
				*st = lp.SyntaxEndState
				syncLine = q + 1
				break
			}
		}
	}
	for q := syncLine; q < lineNumber; q++ {
		lp := bp.Line(q)
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

func syntaxStateAt(bp *buffer.Buffer, lineNumber, offset uint, st *buffer.SynState) bool {
	if bp == nil || lineNumber == 0 || lineNumber > bp.EOF() {
		*st = buffer.SynState{DFA: SynStateNormal}
		return false
	}
	bufferSyntaxFindStart(bp, lineNumber, st)
	if lineNumber >= bp.EOF() {
		return true
	}
	lp := bp.Line(lineNumber)
	if lp == nil {
		return false
	}
	runeLimit := byteOffsetToRuneLimit(lp, offset)
	*st, _, _ = tokenizeLineFromStateLimit(lp, *st, runeLimit)
	return true
}

func syntaxCharIsStructural(bp *buffer.Buffer, lineNumber, offset uint) bool {
	var before, after buffer.SynState
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

func syntaxFindMatchingDelimiter(bp *buffer.Buffer, start buffer.Location, matchOut *buffer.Location) bool {
	if bp == nil || start.Line == 0 || start.Line >= bp.EOF() {
		return false
	}
	lp := bp.Line(start.Line)
	if lp == nil || start.Offset >= lp.Len() {
		return false
	}
	ch := int(lp.Byte(start.Offset))
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
			line := bp.Line(lineNum)
			if line == nil {
				return false
			}
			for off < line.Len() {
				c := int(line.Byte(off))
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
			line := bp.Line(lineNum)
			if line == nil {
				return false
			}
			for off >= 0 {
				c := int(line.Byte(uint(off)))
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
			prev := bp.Line(lineNum)
			if prev == nil {
				return false
			}
			off = int(prev.Len()) - 1
		}
	}
	return false
}

// FindMatchingDelimiter finds the partner delimiter for start.
func FindMatchingDelimiter(bp *buffer.Buffer, start buffer.Location, matchOut *buffer.Location) bool {
	return syntaxFindMatchingDelimiter(bp, start, matchOut)
}

// CharIsStructural reports whether offset on line is a structural delimiter char.
func CharIsStructural(bp *buffer.Buffer, lineNumber, offset uint) bool {
	return syntaxCharIsStructural(bp, lineNumber, offset)
}

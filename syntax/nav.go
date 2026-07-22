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

// byteOffsetToRuneLimit returns how many leading runes end before byteOffset in line.Data.
func byteOffsetToRuneLimit(line *buffer.Line, byteOffset int) int {
	if line == nil {
		return 0
	}
	data := line.Data
	n := len(data)
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset >= n {
		line.EnsureCache()
		return len(line.RuneCache)
	}
	limit := 0
	i := 0
	for i < n {
		if i >= byteOffset {
			break
		}
		_, size := utf8.DecodeRune(data[i:])
		if size == 0 {
			break
		}
		if i+size > byteOffset {
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

func bufferSyntaxFindStart(buf *buffer.Buffer, lineNumber int, st *buffer.SynState) {
	*st = buffer.SynState{DFA: SynStateNormal}
	if buf == nil {
		return
	}
	info := For(buf.LangMode)
	if info.Kind == ModeSyntaxNone ||
		info.Kind == ModeSyntaxMarkdown ||
		info.Kind == ModeSyntaxHashCommentOnly {
		return
	}
	syncLine := 1
	if lineNumber > 1 {
		for q := lineNumber - 1; q > 0; q-- {
			line := buf.Line(q)
			if line != nil && line.SyntaxValid {
				*st = line.SyntaxEndState
				syncLine = q + 1
				break
			}
		}
	}
	for q := syncLine; q < lineNumber; q++ {
		line := buf.Line(q)
		if line == nil {
			continue
		}
		end, summary, styles := tokenizeLineFromState(line, *st)
		line.SyntaxEndState = end
		line.SyntaxSummary = summary
		line.SyntaxStyles = styles
		line.SyntaxValid = true
	}
}

func syntaxStateAt(buf *buffer.Buffer, lineNumber, offset int, st *buffer.SynState) bool {
	if buf == nil || lineNumber == 0 || lineNumber > buf.EOF() {
		*st = buffer.SynState{DFA: SynStateNormal}
		return false
	}
	bufferSyntaxFindStart(buf, lineNumber, st)
	if lineNumber >= buf.EOF() {
		return true
	}
	line := buf.Line(lineNumber)
	if line == nil {
		return false
	}
	runeLimit := byteOffsetToRuneLimit(line, offset)
	*st, _, _ = tokenizeLineFromStateLimit(line, *st, runeLimit)
	return true
}

func syntaxCharIsStructural(buf *buffer.Buffer, lineNumber, offset int) bool {
	var before, after buffer.SynState
	if !syntaxStateAt(buf, lineNumber, offset, &before) {
		return false
	}
	if !syntaxContextIsStructural(syntaxContextFromState(&before)) {
		return false
	}
	if !syntaxStateAt(buf, lineNumber, offset+1, &after) {
		return false
	}
	return syntaxContextIsStructural(syntaxContextFromState(&after))
}

func syntaxFindMatchingDelimiter(buf *buffer.Buffer, start buffer.Location, matchOut *buffer.Location) bool {
	if buf == nil || start.Line == 0 || start.Line >= buf.EOF() {
		return false
	}
	var dummy buffer.SynState
	bufferSyntaxFindStart(buf, len(buf.Lines), &dummy)

	line := buf.Line(start.Line)
	if line == nil || start.Offset >= line.Len() {
		return false
	}
	ch := int(line.Byte(start.Offset))
	open, close, forward, ok := DelimiterPair(ch)
	if !ok {
		return false
	}
	if !syntaxCharIsStructural(buf, start.Line, start.Offset) {
		return false
	}

	depth := 1
	if forward {
		lineNum := start.Line
		off := start.Offset + 1
		for lineNum <= len(buf.Lines) {
			line := buf.Line(lineNum)
			if line == nil {
				return false
			}
			for off < line.Len() {
				c := int(line.Byte(off))
				if (c == open || c == close) && syntaxCharIsStructural(buf, lineNum, off) {
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
		off := start.Offset - 1
		for lineNum >= 1 {
			line := buf.Line(lineNum)
			if line == nil {
				return false
			}
			for off >= 0 {
				c := int(line.Byte(off))
				if (c == open || c == close) && syntaxCharIsStructural(buf, lineNum, off) {
					if c == close {
						depth++
					} else if depth--; depth == 0 {
						*matchOut = buffer.MakeLocation(lineNum, off)
						return true
					}
				}
				off--
			}
			if lineNum == 1 {
				break
			}
			lineNum--
			prev := buf.Line(lineNum)
			if prev == nil {
				return false
			}
			off = prev.Len() - 1
		}
	}
	return false
}

// FindMatchingDelimiter finds the partner delimiter for start.
func FindMatchingDelimiter(buf *buffer.Buffer, start buffer.Location, matchOut *buffer.Location) bool {
	return syntaxFindMatchingDelimiter(buf, start, matchOut)
}

// CharIsStructural reports whether offset on line is a structural delimiter char.
func CharIsStructural(buf *buffer.Buffer, lineNumber, offset int) bool {
	return syntaxCharIsStructural(buf, lineNumber, offset)
}

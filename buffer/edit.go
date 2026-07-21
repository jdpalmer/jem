package buffer

import "math"

// ---- Buffer text operations ---------------------------------------------------

func splitInsertLines(insert []byte) [][]byte {
	if len(insert) == 0 {
		return [][]byte{{}}
	}
	parts := make([][]byte, 0, 4)
	start := 0
	for i := 0; i < len(insert); i++ {
		if insert[i] == '\n' {
			parts = append(parts, append([]byte(nil), insert[start:i]...))
			start = i + 1
		}
	}
	parts = append(parts, append([]byte(nil), insert[start:]...))
	return parts
}

func makeBufferLine(buf *Buffer, data []byte) Line {
	return Line{
		Data:        data,
		SyntaxValid: false,
		LangMode:    buf.LangMode,
		Buffer:      buf,
	}
}

func (buf *Buffer) ReplaceRaw(begin, end Location, newText []byte, newEndOut *Location) error {
	if buf == nil {
		return ErrNilBuffer
	}
	if buf.IsReadonly {
		return ErrReadonly
	}
	insert := newText
	if insert == nil {
		insert = []byte{}
	}

	if begin.Line > end.Line || (begin.Line == end.Line && begin.Offset > end.Offset) {
		return ErrBadRange
	}

	if begin == end && len(insert) == 0 {
		if newEndOut != nil {
			*newEndOut = begin
		}
		return nil
	}

	if len(buf.Lines) == 0 {
		_ = buf.AppendLineBytes(nil)
	}

	beginIsEOF := begin.Line == buf.EOF()
	endIsEOF := end.Line == buf.EOF()
	endReal := end.Line
	if endIsEOF {
		endReal = len(buf.Lines)
	}

	oldLineCount := len(buf.Lines)

	var prefix []byte
	var bline *Line
	if !beginIsEOF {
		bline = buf.Line(begin.Line)
		if bline == nil {
			return ErrBadRange
		}
		bOffset := begin.Offset
		if bOffset > len(bline.Data) {
			bOffset = len(bline.Data)
		}
		prefix = make([]byte, bOffset)
		copy(prefix, bline.Data[:bOffset])
	}

	var suffix []byte
	var eline *Line
	eOffset := 0
	if !endIsEOF {
		eline = buf.Line(end.Line)
		if eline == nil {
			return ErrBadRange
		}
		eOffset = end.Offset
		if eOffset > len(eline.Data) {
			eOffset = len(eline.Data)
		}
		suffix = append([]byte(nil), eline.Data[eOffset:]...)
	}

	var linesBefore []Line
	if beginIsEOF {
		linesBefore = append(linesBefore, buf.Lines...)
	} else if begin.Line > 1 {
		linesBefore = append(linesBefore, buf.Lines[:begin.Line-1]...)
	}

	tailIdx := len(buf.Lines)
	if !endIsEOF {
		tailIdx = end.Line
	}
	var linesAfter []Line
	if tailIdx < len(buf.Lines) {
		linesAfter = append(linesAfter, buf.Lines[tailIdx:]...)
	}

	parts := splitInsertLines(insert)

	var newLines []Line
	newLines = append(newLines, linesBefore...)

	prefixLen := len(prefix)
	suffixLen := len(suffix)

	if len(parts) == 1 {
		merged := make([]byte, 0, prefixLen+len(parts[0])+suffixLen)
		merged = append(merged, prefix...)
		merged = append(merged, parts[0]...)
		merged = append(merged, suffix...)
		newLines = append(newLines, makeBufferLine(buf, merged))
	} else {
		first := make([]byte, 0, prefixLen+len(parts[0]))
		first = append(first, prefix...)
		first = append(first, parts[0]...)
		newLines = append(newLines, makeBufferLine(buf, first))

		for i := 1; i < len(parts)-1; i++ {
			p := append([]byte(nil), parts[i]...)
			newLines = append(newLines, makeBufferLine(buf, p))
		}

		last := make([]byte, 0, len(parts[len(parts)-1])+suffixLen)
		last = append(last, parts[len(parts)-1]...)
		last = append(last, suffix...)
		newLines = append(newLines, makeBufferLine(buf, last))
	}

	newLines = append(newLines, linesAfter...)

	for i := range newLines {
		newLines[i].Buffer = buf
	}

	buf.Lines = newLines

	var newEnd Location
	lineNum := len(linesBefore) + len(parts)
	var offset int
	if len(parts) == 1 {
		offset = prefixLen + len(parts[0])
	} else if len(parts) > 0 {
		offset = len(parts[len(parts)-1])
	}
	newEnd = Location{Line: lineNum, Offset: offset}
	if newEndOut != nil {
		*newEndOut = newEnd
	}

	var normEnd Location
	if beginIsEOF {
		normEnd = begin
	} else if endIsEOF {
		normEnd = Location{Line: endReal, Offset: math.MaxInt}
	} else {
		normEnd = Location{Line: endReal, Offset: end.Offset}
	}

	var resultFirstLine int
	if beginIsEOF {
		resultFirstLine = oldLineCount + 1
	} else {
		resultFirstLine = begin.Line
	}

	callAdjustLocations(buf, begin, normEnd, newEnd)
	buf.InvalidateSyntaxFrom(resultFirstLine)
	callReparseFrom(buf, resultFirstLine)

	return nil
}

func callReparseFrom(buf *Buffer, lineNumber int) {
	if PackageHooks.ReparseFrom != nil {
		PackageHooks.ReparseFrom(buf, lineNumber)
	}
}

// InvalidateSyntaxFrom clears syntax validity from lineNumber through end of buffer.
func (buf *Buffer) InvalidateSyntaxFrom(lineNumber int) {
	if buf == nil || lineNumber <= 0 || lineNumber > len(buf.Lines) {
		return
	}
	for ln := lineNumber; ln <= len(buf.Lines); ln++ {
		line := buf.Line(ln)
		if line != nil {
			line.SyntaxValid = false
		}
	}
}

// NoteEdit marks the buffer changed and notifies PackageHooks when installed.
func (buf *Buffer) NoteEdit(isStructural bool) {
	callNoteEdit(buf, isStructural)
}

func callNoteEdit(buf *Buffer, isStructural bool) {
	if buf == nil {
		return
	}
	// Hooks run before IsChanged so NoteEdit can detect first-change.
	if PackageHooks.NoteEdit != nil {
		PackageHooks.NoteEdit(buf, isStructural)
	}
	buf.IsChanged = true
}

func callAdjustLocations(buf *Buffer, begin, end, newEnd Location) {
	if PackageHooks.AdjustLocationsAfterReplace != nil {
		PackageHooks.AdjustLocationsAfterReplace(buf, begin, end, newEnd)
	}
}

func (buf *Buffer) GetText(begin, end Location) []byte {
	if buf == nil {
		return nil
	}

	// EOF handling: EOF is virtual line len(Lines)+1 with offset 0
	endIsEOF := end.Line == buf.EOF()
	n := 0

	if begin.Line == end.Line {
		// Same line (may be EOF virtual line)
		if begin.Line == buf.EOF() {
			return nil
		}
		line := buf.Line(begin.Line)
		if line == nil {
			return nil
		}
		b := begin.Offset
		used := line.Len()
		if b > used {
			b = used
		}
		e := end.Offset
		if e > used {
			e = used
		}
		if e > b {
			n = e - b
			out := make([]byte, n)
			copy(out, line.Data[b:e])
			return out
		}
		return nil
	}

	// Different lines
	lastReal := end.Line
	if endIsEOF {
		lastReal = len(buf.Lines)
	}

	// compute required size
	// tail of start line
	if begin.Line <= len(buf.Lines) {
		sl := buf.Line(begin.Line)
		if sl != nil {
			slUsed := sl.Len()
			if slUsed > begin.Offset {
				n += slUsed - begin.Offset
			}
		}
	}
	// interior lines
	if begin.Line < lastReal {
		for ln := begin.Line + 1; ln < lastReal; ln++ {
			line := buf.Line(ln)
			if line != nil {
				n += line.Len() + 1 // plus '\n'
			}
		}
		// final segment
		if endIsEOF {
			if lastReal >= 1 && lastReal <= len(buf.Lines) {
				line := buf.Line(lastReal)
				if line != nil {
					n += line.Len()
				}
			}
		} else {
			if lastReal >= 1 && lastReal <= len(buf.Lines) {
				line := buf.Line(lastReal)
				if line != nil {
					lpUsed := line.Len()
					if end.Offset <= lpUsed {
						n += end.Offset
					} else {
						n += lpUsed
					}
				}
			}
		}
	}

	if n == 0 {
		return nil
	}

	out := make([]byte, 0, n)

	// copy start line tail
	if begin.Line <= len(buf.Lines) {
		sl := buf.Line(begin.Line)
		if sl != nil {
			b := begin.Offset
			slUsed := sl.Len()
			if b > slUsed {
				b = slUsed
			}
			if slUsed > b {
				out = append(out, sl.Data[b:slUsed]...)
			}
		}
	}

	// if there are interior lines, join with '\n'
	if begin.Line < lastReal {
		out = append(out, '\n')
		for ln := begin.Line + 1; ln < lastReal; ln++ {
			line := buf.Line(ln)
			if line != nil {
				out = append(out, line.Data...)
			}
			out = append(out, '\n')
		}
		// final segment
		if endIsEOF {
			line := buf.Line(lastReal)
			if line != nil {
				out = append(out, line.Data...)
			}
		} else {
			line := buf.Line(lastReal)
			if line != nil {
				e := end.Offset
				lpUsed := line.Len()
				if e > lpUsed {
					e = lpUsed
				}
				if e > 0 {
					out = append(out, line.Data[:e]...)
				}
			}
		}
	}

	return out
}

// SetText is the interactive edit entry: optional undo recording, NoteEdit
// (PackageHooks + IsChanged), then ReplaceRaw (location adjust, syntax
// invalidate, reparse via PackageHooks).
func (buf *Buffer) SetText(undo *UndoHistory, begin, end Location, newText []byte, newEndOut *Location) error {
	if buf == nil {
		return ErrNilBuffer
	}
	if buf.IsReadonly {
		return ErrReadonly
	}
	if undo != nil {
		oldText := buf.GetText(begin, end)
		undo.RecordEdit(buf, undo.Pending.Before, begin, oldText, newText)
	}
	hasNewline := false
	for i := 0; i < len(newText); i++ {
		if newText[i] == '\n' {
			hasNewline = true
			break
		}
	}
	isStructural := begin.Line != end.Line || hasNewline
	callNoteEdit(buf, isStructural)
	return buf.ReplaceRaw(begin, end, newText, newEndOut)
}

func (buf *Buffer) AppendLineBytes(text []byte) *Line {
	if buf == nil {
		return nil
	}
	var data []byte
	if text != nil {
		data = append([]byte(nil), text...)
	}
	newLine := Line{
		Data:        data,
		SyntaxValid: false,
		LangMode:    buf.LangMode,
		Buffer:      buf,
	}
	buf.Lines = append(buf.Lines, newLine)
	return &buf.Lines[len(buf.Lines)-1]
}

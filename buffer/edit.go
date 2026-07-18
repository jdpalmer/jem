package buffer

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

func makeBufferLine(bp *Buffer, data []byte) Line {
	return Line{
		Data:        data,
		SyntaxValid: false,
		LangMode:    bp.LangMode,
		Buffer:      bp,
	}
}

func (bp *Buffer) ReplaceRaw(begin, end Location, newText []byte, newEndOut *Location) error {
	if bp == nil {
		return ErrNilBuffer
	}
	if bp.IsReadonly {
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

	if bp.LineCount == 0 {
		_ = bp.AppendLineBytes(nil)
	}

	beginIsEOF := begin.Line == bp.EOF()
	endIsEOF := end.Line == bp.EOF()
	endReal := end.Line
	if endIsEOF {
		endReal = bp.LineCount
	}

	oldLineCount := bp.LineCount

	var prefix []byte
	var bline *Line
	if !beginIsEOF {
		bline = bp.Line(begin.Line)
		if bline == nil {
			return ErrBadRange
		}
		bOffset := int(begin.Offset)
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
		eline = bp.Line(end.Line)
		if eline == nil {
			return ErrBadRange
		}
		eOffset = int(end.Offset)
		if eOffset > len(eline.Data) {
			eOffset = len(eline.Data)
		}
		suffix = append([]byte(nil), eline.Data[eOffset:]...)
	}

	var linesBefore []Line
	if beginIsEOF {
		linesBefore = append(linesBefore, bp.Lines...)
	} else if begin.Line > 1 {
		linesBefore = append(linesBefore, bp.Lines[:begin.Line-1]...)
	}

	tailIdx := int(bp.LineCount)
	if !endIsEOF {
		tailIdx = int(end.Line)
	}
	var linesAfter []Line
	if tailIdx < len(bp.Lines) {
		linesAfter = append(linesAfter, bp.Lines[tailIdx:]...)
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
		newLines = append(newLines, makeBufferLine(bp, merged))
	} else {
		first := make([]byte, 0, prefixLen+len(parts[0]))
		first = append(first, prefix...)
		first = append(first, parts[0]...)
		newLines = append(newLines, makeBufferLine(bp, first))

		for i := 1; i < len(parts)-1; i++ {
			p := append([]byte(nil), parts[i]...)
			newLines = append(newLines, makeBufferLine(bp, p))
		}

		last := make([]byte, 0, len(parts[len(parts)-1])+suffixLen)
		last = append(last, parts[len(parts)-1]...)
		last = append(last, suffix...)
		newLines = append(newLines, makeBufferLine(bp, last))
	}

	newLines = append(newLines, linesAfter...)

	for i := range newLines {
		newLines[i].Buffer = bp
	}

	bp.Lines = newLines
	bp.LineCount = uint(len(bp.Lines))

	var newEnd Location
	lineNum := uint(len(linesBefore) + len(parts))
	var offset uint
	if len(parts) == 1 {
		offset = uint(prefixLen + len(parts[0]))
	} else if len(parts) > 0 {
		offset = uint(len(parts[len(parts)-1]))
	}
	newEnd = Location{Line: lineNum, Offset: offset}
	if newEndOut != nil {
		*newEndOut = newEnd
	}

	var normEnd Location
	if beginIsEOF {
		normEnd = begin
	} else if endIsEOF {
		normEnd = Location{Line: endReal, Offset: ^uint(0)}
	} else {
		normEnd = Location{Line: endReal, Offset: end.Offset}
	}

	var resultFirstLine uint
	if beginIsEOF {
		resultFirstLine = oldLineCount + 1
	} else {
		resultFirstLine = begin.Line
	}

	callAdjustLocations(bp, begin, normEnd, newEnd)
	bp.InvalidateSyntaxFrom(resultFirstLine)
	callReparseFrom(bp, resultFirstLine)

	return nil
}

func callReparseFrom(bp *Buffer, lineNumber uint) {
	if editSession.ReparseFrom != nil {
		editSession.ReparseFrom(bp, lineNumber)
	}
}

// InvalidateSyntaxFrom clears syntax validity from lineNumber through end of buffer.
func (bp *Buffer) InvalidateSyntaxFrom(lineNumber uint) {
	if bp == nil || lineNumber == 0 || lineNumber > bp.LineCount {
		return
	}
	for ln := lineNumber; ln <= bp.LineCount; ln++ {
		lp := bp.Line(ln)
		if lp != nil {
			lp.SyntaxValid = false
		}
	}
}

// NoteEdit marks the buffer changed and notifies the EditSession when installed.
func (bp *Buffer) NoteEdit(isStructural bool) {
	callNoteEdit(bp, isStructural)
}

func callNoteEdit(bp *Buffer, isStructural bool) {
	if bp == nil {
		return
	}
	// Session runs before IsChanged so NoteEdit can detect first-change.
	if editSession.NoteEdit != nil {
		editSession.NoteEdit(bp, isStructural)
	}
	bp.IsChanged = true
}

func callAdjustLocations(bp *Buffer, begin, end, newEnd Location) {
	if editSession.AdjustLocationsAfterReplace != nil {
		editSession.AdjustLocationsAfterReplace(bp, begin, end, newEnd)
	}
}

// LocationAdjustAfterReplace updates a single Location in place to account for
// a replacement of [begin,end) with newEnd, following the logic from src/edit.c.
func (loc *Location) AdjustAfterReplace(begin, end, newEnd Location) {
	if loc == nil {
		return
	}
	if loc.Line < begin.Line {
		return
	}
	if loc.Line == begin.Line && loc.Offset < begin.Offset {
		return
	}
	if loc.Line == end.Line && loc.Offset > end.Offset {
		loc.Line = newEnd.Line
		loc.Offset = newEnd.Offset + (loc.Offset - end.Offset)
		return
	}
	if loc.Line > end.Line {
		if newEnd.Line >= end.Line {
			loc.Line += newEnd.Line - end.Line
		} else {
			removed := end.Line - newEnd.Line
			if loc.Line >= removed {
				loc.Line -= removed
			} else {
				loc.Line = 1
			}
		}
		return
	}
	// Default: set to newEnd
	loc.Line = newEnd.Line
	loc.Offset = newEnd.Offset
}

func (bp *Buffer) GetText(begin, end Location) []byte {
	if bp == nil {
		return nil
	}

	// EOF handling: EOF is virtual line bp.LineCount+1 with offset 0
	endIsEOF := end.Line == bp.EOF()
	var n uint = 0

	if begin.Line == end.Line {
		// Same line (may be EOF virtual line)
		if begin.Line == bp.EOF() {
			return nil
		}
		lp := bp.Line(begin.Line)
		if lp == nil {
			return nil
		}
		b := begin.Offset
		used := lp.Len()
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
			copy(out, lp.Data[b:e])
			return out
		}
		return nil
	}

	// Different lines
	lastReal := end.Line
	if endIsEOF {
		lastReal = bp.LineCount
	}

	// compute required size
	// tail of start line
	if begin.Line <= bp.LineCount {
		sl := bp.Line(begin.Line)
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
			lp := bp.Line(ln)
			if lp != nil {
				n += lp.Len() + 1 // plus '\n'
			}
		}
		// final segment
		if endIsEOF {
			if lastReal >= 1 && lastReal <= bp.LineCount {
				lp := bp.Line(lastReal)
				if lp != nil {
					n += lp.Len()
				}
			}
		} else {
			if lastReal >= 1 && lastReal <= bp.LineCount {
				lp := bp.Line(lastReal)
				if lp != nil {
					lpUsed := lp.Len()
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
	if begin.Line <= bp.LineCount {
		sl := bp.Line(begin.Line)
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
			lp := bp.Line(ln)
			if lp != nil {
				out = append(out, lp.Data...)
			}
			out = append(out, '\n')
		}
		// final segment
		if endIsEOF {
			lp := bp.Line(lastReal)
			if lp != nil {
				out = append(out, lp.Data...)
			}
		} else {
			lp := bp.Line(lastReal)
			if lp != nil {
				e := end.Offset
				lpUsed := lp.Len()
				if e > lpUsed {
					e = lpUsed
				}
				if e > 0 {
					out = append(out, lp.Data[:e]...)
				}
			}
		}
	}

	return out
}

// SetText is the interactive edit-session entry point: optional undo recording,
// NoteEdit (session + IsChanged), then ReplaceRaw (location adjust, syntax
// invalidate, reparse via the active EditSession).
func (bp *Buffer) SetText(undo *UndoHistory, begin, end Location, newText []byte, newEndOut *Location) error {
	if bp == nil {
		return ErrNilBuffer
	}
	if bp.IsReadonly {
		return ErrReadonly
	}
	if undo != nil {
		oldText := bp.GetText(begin, end)
		undo.RecordEdit(bp, undo.Pending.Before, begin, oldText, newText)
	}
	hasNewline := false
	for i := 0; i < len(newText); i++ {
		if newText[i] == '\n' {
			hasNewline = true
			break
		}
	}
	isStructural := begin.Line != end.Line || hasNewline
	callNoteEdit(bp, isStructural)
	return bp.ReplaceRaw(begin, end, newText, newEndOut)
}

func (bp *Buffer) AppendLineBytes(text []byte) *Line {
	if bp == nil {
		return nil
	}
	var data []byte
	if text != nil {
		data = append([]byte(nil), text...)
	}
	newLine := Line{
		Data:        data,
		SyntaxValid: false,
		LangMode:    bp.LangMode,
		Buffer:      bp,
	}
	bp.Lines = append(bp.Lines, newLine)
	bp.LineCount = uint(len(bp.Lines))
	return &bp.Lines[bp.LineCount-1]
}

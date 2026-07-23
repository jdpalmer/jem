package buffer

type Location struct {
	Line   int
	Offset int
}

// MakeLocation creates a new Location with the given line and offset.
func MakeLocation(line, offset int) Location {
	return Location{Line: line, Offset: offset}
}

// AdvanceBytes returns a new Location advanced by the given number of bytes.
func (loc Location) AdvanceBytes(buf *Buffer, bytes int) Location {
	if loc.Line == buf.EOF() {
		return loc
	}
	curLine := loc.Line
	offset := loc.Offset
	for bytes > 0 {
		if curLine <= 0 || curLine > len(buf.Lines) {
			return Location{Line: buf.EOF(), Offset: 0}
		}
		line := buf.Line(curLine)
		if line == nil {
			return Location{Line: buf.EOF(), Offset: 0}
		}
		avail := len(line.Data) - offset
		if avail >= bytes {
			return Location{Line: curLine, Offset: offset + bytes}
		}
		bytes -= avail
		if curLine < len(buf.Lines) {
			if bytes == 0 {
				return Location{Line: curLine + 1, Offset: 0}
			}
			bytes--
			curLine++
			offset = 0
			continue
		}
		return Location{Line: buf.EOF(), Offset: 0}
	}
	return Location{Line: curLine, Offset: offset}
}

// RewindBytes returns a new Location moved back by the given number of bytes.
func (loc Location) RewindBytes(buf *Buffer, bytes int) Location {
	curLine := loc.Line
	offset := loc.Offset
	for bytes > 0 {
		if offset > 0 {
			step := bytes
			if step > offset {
				step = offset
			}
			offset -= step
			bytes -= step
			continue
		}
		if curLine <= 1 {
			break
		}
		curLine--
		line := buf.Line(curLine)
		if line == nil {
			break
		}
		offset = len(line.Data)
		bytes--
	}
	return Location{Line: curLine, Offset: offset}
}

// AdjustAfterReplace updates a Location in place for a replacement of
// [begin,end) ending at newEnd.
func (loc *Location) AdjustAfterReplace(begin, end, newEnd Location) {
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
	loc.Line = newEnd.Line
	loc.Offset = newEnd.Offset
}

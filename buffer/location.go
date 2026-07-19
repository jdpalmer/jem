package buffer

type Location struct {
	Line   uint
	Offset uint
}

func MakeLocation(line, offset uint) Location {
	return Location{Line: line, Offset: offset}
}

func (loc Location) AdvanceBytes(bp *Buffer, bytes int) Location {
	if bp == nil || bytes <= 0 {
		return loc
	}
	if loc.Line == bp.EOF() {
		return loc
	}
	curLine := loc.Line
	offset := int(loc.Offset)
	for bytes > 0 {
		if curLine == 0 || curLine > bp.LineCount {
			return Location{Line: bp.EOF(), Offset: 0}
		}
		lp := bp.Line(curLine)
		if lp == nil {
			return Location{Line: bp.EOF(), Offset: 0}
		}
		avail := len(lp.Data) - offset
		if avail >= bytes {
			return Location{Line: curLine, Offset: uint(offset + bytes)}
		}
		bytes -= avail
		if curLine < bp.LineCount {
			if bytes == 0 {
				return Location{Line: curLine + 1, Offset: 0}
			}
			bytes--
			curLine++
			offset = 0
			continue
		}
		return Location{Line: bp.EOF(), Offset: 0}
	}
	return Location{Line: curLine, Offset: uint(offset)}
}

func (loc Location) RewindBytes(bp *Buffer, bytes int) Location {
	if bp == nil || bytes <= 0 {
		return loc
	}
	curLine := loc.Line
	offset := int(loc.Offset)
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
		lp := bp.Line(curLine)
		if lp == nil {
			break
		}
		offset = len(lp.Data)
		bytes--
	}
	return Location{Line: curLine, Offset: uint(offset)}
}

// AdjustAfterReplace updates a Location in place for a replacement of
// [begin,end) ending at newEnd (src/edit.c).
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
	loc.Line = newEnd.Line
	loc.Offset = newEnd.Offset
}

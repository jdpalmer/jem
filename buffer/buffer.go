package buffer

import (
	"time"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

func New() *Buffer {
	return &Buffer{
		EolMode:  EModeLF,
		LangMode: LModeNone,
	}
}

func (bp *Buffer) Destroy() {
	if bp == nil {
		return
	}
	if bp.Lines != nil {
		for i := range bp.Lines {
			bp.Lines[i].Data = nil
		}
		bp.Lines = nil
	}
	bp.LineCount = 0
	bp.Name = ""
	bp.FileName = ""
	bp.Serial = 0
	bp.SavedUndoSerial = 0
	bp.FileMtime = time.Time{}
}

func (bp *Buffer) Clear() bool {
	if bp == nil {
		return false
	}
	bp.Lines = nil
	bp.LineCount = 0
	bp.IsChanged = false
	bp.Cursor = Location{Line: 1, Offset: 0}
	bp.Mark = Location{Line: 0, Offset: 0}
	return true
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

// EnsureLineCache decodes UTF-8 runes for syntax and display helpers.
func (lp *Line) EnsureCache() {
	if lp == nil || lp.CacheValid {
		return
	}
	bs := lp.Data
	capNeeded := len(bs)/2 + 1
	runes := make([]rune, 0, capNeeded)
	widths := make([]int8, 0, capNeeded)
	i := 0
	for i < len(bs) {
		r, size := utf8.DecodeRune(bs[i:])
		if r == utf8.RuneError && size == 1 {
			r = rune(bs[i])
			size = 1
		}
		w := runewidth.RuneWidth(r)
		if w <= 0 {
			w = 1
		}
		runes = append(runes, r)
		widths = append(widths, int8(w))
		i += size
	}
	lp.RuneCache = runes
	lp.WidthCache = widths
	lp.CacheValid = true
}

// Line helpers exported for language modes and search.

func (lp *Line) FirstNonblank() uint {
	if lp == nil {
		return 0
	}
	for i := uint(0); i < uint(len(lp.Data)); i++ {
		c := lp.Data[i]
		if c != ' ' && c != '\t' {
			return i
		}
	}
	return uint(len(lp.Data))
}

func (lp *Line) IndentColumn() uint {
	if lp == nil {
		return 0
	}
	col := uint(0)
	for i := uint(0); i < uint(len(lp.Data)); i++ {
		c := lp.Data[i]
		if c == ' ' {
			col++
		} else if c == '\t' {
			col += 8 - (col % 8)
		} else {
			break
		}
	}
	return col
}

func (lp *Line) FirstByte() byte {
	if lp == nil || len(lp.Data) == 0 {
		return 0
	}
	return lp.Data[0]
}

func (lp *Line) LastByte() byte {
	if lp == nil || len(lp.Data) == 0 {
		return 0
	}
	return lp.Data[len(lp.Data)-1]
}

func (lp *Line) IsBlank() bool {
	if lp == nil || len(lp.Data) == 0 {
		return true
	}
	for i := uint(0); i < uint(len(lp.Data)); i++ {
		c := lp.Data[i]
		if c != ' ' && c != '\t' {
			return false
		}
	}
	return true
}

func (bp *Buffer) TrimTrailingWhitespace(lineNumber uint) bool {
	if lineNumber == 0 || lineNumber > bp.LineCount {
		return false
	}
	line := &bp.Lines[lineNumber-1]
	newLen := uint(len(line.Data))
	for newLen > 0 {
		c := line.Data[newLen-1]
		if c != ' ' && c != '\t' {
			break
		}
		newLen--
	}
	if newLen == uint(len(line.Data)) {
		return false
	}
	begin := Location{Line: lineNumber, Offset: newLen}
	end := Location{Line: lineNumber, Offset: uint(len(line.Data))}
	return bp.SetText(nil, begin, end, nil, 0, nil)
}

package modes

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/session"
)

type (
	Buffer   = buffer.Buffer
	Line     = buffer.Line
	Location = buffer.Location
	Window   = session.Window
)

func MakeLocation(line, offset uint) Location { return buffer.MakeLocation(line, offset) }
func BufferEOF(bp *Buffer) uint               { return buffer.EOF(bp) }
func BufferGetLine(bp *Buffer, lineNumber uint) *Line {
	return buffer.GetLine(bp, lineNumber)
}
func LineGetc(lp *Line, n uint) byte { return buffer.LineGetc(lp, n) }
func LineLength(lp *Line) uint       { return buffer.LineLength(lp) }

func line_first_nonblank(lp *Line) uint { return buffer.LineFirstNonblank(lp) }
func line_indent_column(lp *Line) uint  { return buffer.LineIndentColumn(lp) }
func line_first_byte(lp *Line) byte     { return buffer.LineFirstByte(lp) }
func line_last_byte(lp *Line) byte      { return buffer.LineLastByte(lp) }
func line_is_blank(lp *Line) bool       { return buffer.LineIsBlank(lp) }

package display

import (
	"github.com/jdpalmer/jem/window"
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/term"
)

func selectionStyle(base buffer.TextStyle) buffer.TextStyle {
	flags := base & buffer.TextStyleBold
	return buffer.MakeTextStyle(buffer.TermColorBase03, Active.Theme.SelectionBg, flags)
}

// selInit initialises selection state from the window's mark and cursor.
func selInit(ss *SelState, win *window.Window) {
	ss.active = false
	ss.phase = selectionBefore
	if win.Mark.Line == 0 {
		return
	}
	markLine := win.Mark.Line
	dotLine := win.Cursor.Line
	marko := win.Mark.Offset
	doto := win.Cursor.Offset

	if markLine == dotLine {
		if marko == doto {
			return // zero-width region
		}
		ss.active = true
		ss.startLine = markLine
		ss.endLine = markLine
		if marko < doto {
			ss.startO = marko
			ss.endO = doto
		} else {
			ss.startO = doto
			ss.endO = marko
		}
		return
	}

	ss.active = true
	if markLine < dotLine {
		ss.startLine = markLine
		ss.startO = marko
		ss.endLine = dotLine
		ss.endO = doto
	} else {
		ss.startLine = dotLine
		ss.startO = doto
		ss.endLine = markLine
		ss.endO = marko
	}

	if win.TopLine > ss.endLine {
		ss.active = false
	} else if win.TopLine > ss.startLine {
		ss.phase = selectionInside
	}
}

// selLine returns the byte range [s, e) to highlight for lineNumber.
// Returns s==-1 if no selection on this line.
func selLine(ss *SelState, lineNumber int, line *buffer.Line) (s, e int) {
	s = -1
	e = 0
	if !ss.active {
		return
	}
	length := line.Len()

	if ss.phase == selectionBefore {
		if lineNumber != ss.startLine {
			return
		}
		if ss.startLine == ss.endLine {
			s = int(ss.startO)
			e = int(ss.endO)
			ss.phase = selectionAfter
		} else {
			s = int(ss.startO)
			e = length
			ss.phase = selectionInside
		}
	} else if ss.phase == selectionInside {
		if lineNumber == ss.endLine {
			s = 0
			e = int(ss.endO)
			ss.phase = selectionAfter
		} else {
			s = 0
			e = length
		}
	}
	return
}

// ---- Phantom cursor --------------------------------------------------------------

// restorePhantomCursor restores the back buffer cell overwritten by overlayPhantomCursor.
func restorePhantomCursor() {
	if !Active.PhantomCursorValid {
		return
	}
	row := int(Active.PhantomCursor.Row)
	col := int(Active.PhantomCursor.Col)
	Active.PhantomCursorValid = false
	if row < 0 || row >= len(backScreen.Rows) || col < 0 || col >= term.Cols() {
		return
	}
	backRow := &backScreen.Rows[row]
	backRow.Text[col] = phantomTextRune
	backRow.Style[col] = Active.PhantomStyle
	backRow.Dirty = true
}

// overlayPhantomCursor paints a block cursor at the editor cursor position.
func overlayPhantomCursor() {
	if !Active.ShowPhantomCursor {
		return
	}
	row := int(Active.Cursor.Row)
	col := int(Active.Cursor.Col)
	if row < 0 || row >= len(backScreen.Rows) || col < 0 || col >= term.Cols() {
		return
	}
	backRow := &backScreen.Rows[row]
	Active.PhantomCursor = Active.Cursor
	phantomTextRune = backRow.Text[col]
	Active.PhantomText = byte(backRow.Text[col])
	Active.PhantomStyle = backRow.Style[col]
	Active.PhantomCursorValid = true
	backRow.Style[col] = selectionStyle(backRow.Style[col]) | buffer.TextStyleBold
	backRow.Dirty = true
}

// ---- Gutter rendering -----------------------------------------------------------

// screenPutLineno renders a line-number gutter of width columns.
// Format: [git-marker][right-justified line number][left-clipped indicator]
func screenPutLineno(width, lineno int, marker int, leftClipped bool) {
	// Save draw and active styles
	savedActiveStyle := Active.ActiveStyle
	savedDrawStyle := drawStyle

	gutterStyle := Active.Theme.GutterStyle
	displaySetStyle(gutterStyle)
	drawStyle = gutterStyle

	// Git marker glyph (first column)
	if marker != 0 {
		var glyph rune = ' '
		var glyphStyle buffer.TextStyle
		bg := gutterStyle.Bg()
		switch marker {
		case 1:
			glyph = '+'
			glyphStyle = buffer.MakeTextStyle(buffer.TermColorGreen, bg, buffer.TextStyleBold)
		case 2:
			glyph = '~'
			glyphStyle = buffer.MakeTextStyle(buffer.TermColorYellow, bg, buffer.TextStyleBold)
		case 3:
			glyph = '-'
			glyphStyle = buffer.MakeTextStyle(buffer.TermColorRed, bg, buffer.TextStyleBold)
		default:
			glyphStyle = gutterStyle
		}
		displayPutGlyphStyle(glyph, glyphStyle)
	} else {
		screenPutc(' ')
	}

	// Right-justify line number in (width-2) columns, then left-clipped indicator
	numWidth := width - 2
	if numWidth < 0 {
		numWidth = 0
	}

	// Build the number text right-justified
	numBuf := make([]byte, 0, numWidth+1)
	if lineno > 0 {
		// Format number
		var tmp [12]byte
		n := 0
		v := lineno
		for v > 0 {
			tmp[n] = byte('0' + v%10)
			n++
			v /= 10
		}
		// Pad with spaces
		padLen := numWidth - n
		for i := 0; i < padLen; i++ {
			numBuf = append(numBuf, ' ')
		}
		// Append digits in reverse
		for i := n - 1; i >= 0; i-- {
			numBuf = append(numBuf, tmp[i])
		}
	} else {
		for i := 0; i < numWidth; i++ {
			numBuf = append(numBuf, ' ')
		}
	}
	screenPutBytes(numBuf)

	// Left-clipped indicator
	if leftClipped {
		screenPutc('<')
	} else {
		screenPutc(' ')
	}

	// Restore styles
	drawStyle = savedDrawStyle
	displaySetStyle(savedActiveStyle)
}

// renderBlankRow renders an empty row past the end of the buffer.
func renderBlankRow(row, gutter int) {
	screenMove(row, 0)
	screenPutLineno(gutter, 0, 0, false)
	screenEraseEol()
}

// ---- Content rendering ----------------------------------------------------------

// WindowCursorScreenCol returns the screen column of the cursor in win.
func WindowCursorScreenCol(win *window.Window) int {
	if win == nil || win.Buffer == nil {
		return 0
	}
	line := win.Buffer.Line(win.Cursor.Line)
	if line == nil {
		return 0
	}
	return lineColAtOffset(line, win.Cursor.Offset)
}

// screenPutPickerLine renders a full match-window row using the picker style.
func screenPutPickerLine(line *buffer.Line) {
	style := Active.Theme.PickerSelectionStyle
	savedDrawStyle := drawStyle
	drawStyle = style
	n := len(line.Data)
	i := 0
	for i < n {
		c := rune(line.Data[i])
		size := 1
		if line.Data[i] >= 0x80 {
			r, sz := utf8.DecodeRune(line.Data[i:n])
			if !(r == utf8.RuneError && sz == 1) {
				c = r
				size = sz
			}
		}
		screenPutGlyph(c)
		i += size
	}
	drawStyle = savedDrawStyle
}

// screenPutLine renders line content with syntax highlight and selection overlay.
func screenPutLine(line *buffer.Line, _ buffer.LangMode, _ *buffer.SynState, selStart, selEnd int) {
	syntax.SyntaxEnsureLine(line)
	n := len(line.Data)

	if n == 0 {
		if selStart >= 0 {
			displayPutGlyphStyle(' ', selectionStyle(Active.Theme.NormalStyle))
		}
		return
	}

	savedDrawStyle := drawStyle
	i := 0  // byte offset
	ri := 0 // rune index
	for i < n {
		// Get syntax style for this rune
		style := Active.Theme.NormalStyle
		if ri < len(line.SyntaxStyles) {
			style = line.SyntaxStyles[ri]
		}
		// Apply selection overlay
		if selStart >= 0 && i >= selStart && i < selEnd {
			style = selectionStyle(style)
		}
		drawStyle = style

		// Decode next rune
		c := rune(line.Data[i])
		size := 1
		if line.Data[i] >= 0x80 {
			r, sz := utf8.DecodeRune(line.Data[i:n])
			if !(r == utf8.RuneError && sz == 1) {
				c = r
				size = sz
			}
		}
		screenPutGlyph(c)
		i += size
		ri++
	}
	drawStyle = savedDrawStyle
}

// renderLine renders one buffer line (gutter + content + horizontal scroll) into row.
func renderLine(win *window.Window, lineNumber int, row int, synSt *buffer.SynState, selSt *SelState) {
	line := win.Buffer.Line(lineNumber)
	if line == nil {
		return
	}
	gutter := win.GutterWidth()
	marker := gitLineDiff(win.Buffer, lineNumber)

	var ss, se int
	if selSt != nil {
		ss, se = selLine(selSt, lineNumber, line)
	} else {
		ss = -1
	}

	screenMove(row, 0)
	// Content starts at column gutter; horizontal scroll shifts content left
	swCursorCol = gutter - win.HScroll
	tabOriginCol = swCursorCol
	oldClipLeft := clipLeftCol
	clipLeftCol = gutter

	if win.Buffer != nil && win.Buffer.Name == "*match*" && line != nil && line.Len() >= 2 && line.Data[0] == '>' && line.Data[1] == ' ' {
		screenPutPickerLine(line)
	} else {
		screenPutLine(line, win.Buffer.LangMode, synSt, ss, se)
	}

	clipLeftCol = oldClipLeft
	// Clamp sw cursor to gutter (handles case where hscroll > line width)
	if swCursorCol < gutter {
		swCursorCol = gutter
	}
	screenEraseEol()

	// Render gutter last so it paints over any content that bled into gutter area
	screenMove(row, 0)
	screenPutLineno(gutter, int(lineNumber), marker, win.HScroll > 0)
}

// ---- Modeline rendering ---------------------------------------------------------

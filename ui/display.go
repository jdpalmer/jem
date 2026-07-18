package ui

import (
	"fmt"
	"os"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/modes"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/term"
	"github.com/mattn/go-runewidth"
)

// display.go - Double-buffered terminal display rendering (translation of display.c)

// ---- Types -----------------------------------------------------------------------

type RenderSpan struct {
	Style TextStyle
	Bytes []byte
}

type ScreenRow struct {
	Dirty bool
	Text  []rune
	Style []TextStyle
	Spans []RenderSpan // kept for ScreenSync span-based emission; filled by renderRow
}

type Screen struct {
	Rows     []ScreenRow
	RowCount int
}

// SelectionPhase tracks where we are relative to the selected region.
type SelectionPhase int

const (
	selectionBefore SelectionPhase = 0
	selectionInside SelectionPhase = 1
	selectionAfter  SelectionPhase = 2
)

// SelState holds pre-computed selection bounds for a single rendering pass.
type SelState struct {
	active    bool
	startLine uint
	startO    uint
	endLine   uint
	endO      uint
	phase     SelectionPhase
}

// ---- Globals ---------------------------------------------------------------------

var (
	frontScreen Screen
	backScreen  Screen
)

// Virtual screen cursor and style
var swCursorRow, swCursorCol int
var drawStyle TextStyle
var tabOriginCol int
var clipLeftCol int

// phantomTextRune stores the full rune for phantom cursor restore (types.go only has byte)
var phantomTextRune rune

// Pools shared with edit.go
var runeSlicePool sync.Pool
var widthSlicePool sync.Pool
var rowBufPool sync.Pool
var renderBufPool sync.Pool

// ---- Init ------------------------------------------------------------------------

func DisplayInit() {
	term.RefreshSize()
	DisplayInitHeadless(term.Rows(), term.Cols())
}

// DisplayInitHeadless sets up display buffers without a real terminal (tests, tools).
func DisplayInitHeadless(rows, cols int) {
	if rows < 1 {
		rows = 24
	}
	if cols < 1 {
		cols = 80
	}
	term.SetSize(rows, cols)
	frontScreen = allocScreen(term.Rows())
	backScreen = allocScreen(term.Rows())
	app.State.Theme.Mode = ThemeDark
	themeUpdate()
}

// themeUpdate recomputes cached palette colors from the active theme mode (src/theme.c).
func themeUpdate() {
	theme := &app.State.Theme
	if theme.Mode == ThemeLight {
		theme.NormalStyle = MakeTextStyle(TermColorBase00, TermColorBase3, 0)
		theme.CommentStyle = MakeTextStyle(TermColorBase1, TermColorBase3, 0)
		theme.GutterStyle = MakeTextStyle(TermColorBase1, TermColorBase2, 0)
		theme.SelectionBg = TermColorYellow
		theme.ModelineNameColor = TermColorBase01
		theme.PickerSelectionStyle = MakeTextStyle(TermColorBase03, TermColorBase2, 0)
	} else {
		theme.NormalStyle = MakeTextStyle(TermColorBase0, TermColorBase03, 0)
		theme.CommentStyle = MakeTextStyle(TermColorBase01, TermColorBase03, 0)
		theme.GutterStyle = TextStyleGutter
		theme.SelectionBg = TermColorBlue
		theme.ModelineNameColor = TermColorBase1
		theme.PickerSelectionStyle = MakeTextStyle(TermColorBase3, TermColorBase02, 0)
	}
	term.ClearStyleCache()
	for _, s := range []TextStyle{
		theme.NormalStyle,
		theme.CommentStyle,
		theme.GutterStyle,
		theme.PickerSelectionStyle,
		MakeTextStyle(theme.ModelineNameColor, TextStyleBg(theme.GutterStyle), TextStyleBold),
		MakeTextStyle(TermColorRed, TextStyleBg(theme.GutterStyle), TextStyleBold),
	} {
		_ = term.StyleBytes(s)
	}
}

func allocScreen(rows int) Screen {
	cols := term.Cols()
	s := Screen{
		Rows:     make([]ScreenRow, rows+1),
		RowCount: rows + 1,
	}
	for i := range s.Rows {
		s.Rows[i].Text = make([]rune, cols)
		s.Rows[i].Style = make([]TextStyle, cols)
		s.Rows[i].Spans = nil
		for j := 0; j < cols; j++ {
			s.Rows[i].Text[j] = ' '
			s.Rows[i].Style[j] = app.State.Theme.NormalStyle
		}
	}
	return s
}

// ---- Virtual screen style --------------------------------------------------------

// displaySetStyle applies style to the terminal only when it differs from ActiveStyle.
func displaySetStyle(style TextStyle) {
	if app.State.ActiveStyle != style {
		term.SetStyle(style)
		app.State.ActiveStyle = style
	}
}

// ---- Virtual screen writes -------------------------------------------------------

// screenMove positions the software cursor and resets drawStyle to NormalStyle.
func screenMove(row, col int) {
	swCursorRow = row
	swCursorCol = col
	drawStyle = app.State.Theme.NormalStyle
}

// screenSetStyle sets drawStyle without moving the cursor.
func screenSetStyle(style TextStyle) {
	drawStyle = style
}

// screenPutRaw writes one codepoint into the back buffer at the sw cursor.
func screenPutRaw(c rune) {
	if swCursorRow < 0 || swCursorRow >= len(backScreen.Rows) {
		return
	}
	backRow := &backScreen.Rows[swCursorRow]

	if swCursorCol >= term.Cols() {
		if backRow.Text[term.Cols()-1] != '$' {
			backRow.Dirty = true
		}
		backRow.Text[term.Cols()-1] = '$'
		return
	}

	var w int
	if c >= 0x80 {
		w = runewidth.RuneWidth(c)
	} else {
		w = 1
	}

	if swCursorCol < clipLeftCol {
		// Before visible content area: advance counter but don't write
		if w == 2 {
			swCursorCol += 2
		} else {
			swCursorCol++
		}
		return
	}

	if w == 2 {
		if swCursorCol+1 < term.Cols() {
			if backRow.Text[swCursorCol] != c || backRow.Style[swCursorCol] != drawStyle {
				backRow.Dirty = true
			}
			backRow.Text[swCursorCol] = c
			backRow.Style[swCursorCol] = drawStyle
			swCursorCol++
			// Zero sentinel in second cell of wide char
			if backRow.Text[swCursorCol] != 0 || backRow.Style[swCursorCol] != drawStyle {
				backRow.Dirty = true
			}
			backRow.Text[swCursorCol] = 0
			backRow.Style[swCursorCol] = drawStyle
			swCursorCol++
		} else {
			// Wide char doesn't fit in last cell
			if backRow.Text[term.Cols()-1] != '$' {
				backRow.Dirty = true
			}
			backRow.Text[term.Cols()-1] = '$'
		}
	} else {
		if backRow.Text[swCursorCol] != c || backRow.Style[swCursorCol] != drawStyle {
			backRow.Dirty = true
		}
		backRow.Text[swCursorCol] = c
		backRow.Style[swCursorCol] = drawStyle
		swCursorCol++
	}
}

// screenPutGlyph handles tabs, control chars, then delegates to screenPutRaw.
func screenPutGlyph(c rune) {
	if c == '\t' {
		if swCursorCol < tabOriginCol {
			tabOriginCol = swCursorCol
		}
		// C display.c uses do-while: always emit at least one space, then
		// continue until aligned to an 8-column boundary from tabOriginCol.
		for {
			screenPutGlyph(' ')
			if (swCursorCol-tabOriginCol)&0x07 == 0 {
				break
			}
		}
	} else if c < 0x20 || c == 0x7F {
		screenPutGlyph('^')
		screenPutGlyph(c ^ 0x40)
	} else {
		screenPutRaw(c)
	}
}

// screenPutBytes writes UTF-8 bytes into the back buffer using screenPutGlyph.
// Fast path for ASCII 0x20-0x7E uses screenPutRaw directly.
func screenPutBytes(s []byte) {
	i := 0
	for i < len(s) {
		// Fast path: contiguous ASCII printable chars
		for i < len(s) {
			b := s[i]
			if b < 0x20 || b > 0x7E {
				break
			}
			screenPutRaw(rune(b))
			i++
		}
		if i >= len(s) {
			break
		}
		b := s[i]
		if b < 0x80 {
			// Control char or non-printable ASCII
			screenPutGlyph(rune(b))
			i++
		} else {
			r, size := utf8.DecodeRune(s[i:])
			if r == utf8.RuneError && size == 1 {
				screenPutGlyph(rune(b))
				i++
			} else {
				screenPutGlyph(r)
				i += size
			}
		}
	}
}

// screenPutc is an alias for screenPutRaw.
func screenPutc(c rune) { screenPutRaw(c) }

// screenEraseEol fills from sw cursor to end of row with spaces at drawStyle.
func screenEraseEol() {
	if swCursorRow < 0 || swCursorRow >= len(backScreen.Rows) {
		return
	}
	backRow := &backScreen.Rows[swCursorRow]
	for swCursorCol < term.Cols() {
		if backRow.Text[swCursorCol] != ' ' || backRow.Style[swCursorCol] != drawStyle {
			backRow.Dirty = true
		}
		backRow.Text[swCursorCol] = ' '
		backRow.Style[swCursorCol] = drawStyle
		swCursorCol++
	}
}

// screenFlushRow syncs one row to the terminal and positions the hardware cursor.
// Used by the minibuffer for immediate prompt rendering.
func screenFlushRow(row, cursorCol int) {
	if row < 0 || row >= len(backScreen.Rows) {
		return
	}
	if cursorCol >= term.Cols() {
		cursorCol = term.Cols() - 1
	}
	rowSync(row)
	if row == term.Rows() {
		app.State.Cursor.Row = uint32(term.Rows())
		app.State.Cursor.Col = uint32(cursorCol)
	}
	term.Move(row, cursorCol)
	term.Flush()
}

// displayPutBytesStyle writes bytes with a temporary drawStyle.
func displayPutBytesStyle(s []byte, style TextStyle) {
	saved := drawStyle
	drawStyle = style
	screenPutBytes(s)
	drawStyle = saved
}

// displayPutGlyphStyle writes one codepoint with a temporary drawStyle.
func displayPutGlyphStyle(c rune, style TextStyle) {
	saved := drawStyle
	drawStyle = style
	screenPutc(c)
	drawStyle = saved
}

// ---- Diff-based sync -------------------------------------------------------------

// screenWriteSpan encodes a rune slice to UTF-8 and writes it to the terminal.
// Zero values are skipped (wide-char second-cell sentinels).
func screenWriteSpan(text []rune, n int) {
	var out [4096]byte
	outLen := 0
	for i := 0; i < n; i++ {
		c := text[i]
		if c == 0 {
			continue // sentinel for wide char second cell
		}
		var tmp [4]byte
		sz := utf8.EncodeRune(tmp[:], c)
		if outLen+sz > len(out) {
			term.Write(out[:outLen])
			outLen = 0
		}
		copy(out[outLen:], tmp[:sz])
		outLen += sz
	}
	if outLen > 0 {
		term.Write(out[:outLen])
	}
}

// rowEmitDiffs emits only changed cells in [from, to) for the given row.
func rowEmitDiffs(row, from, to int) {
	backRow := &backScreen.Rows[row]
	frontRow := &frontScreen.Rows[row]

	k := from
	for k < to {
		// Skip matching cells
		for k < to && backRow.Text[k] == frontRow.Text[k] && backRow.Style[k] == frontRow.Style[k] {
			k++
		}
		if k >= to {
			break
		}
		seg := k
		term.Move(row, seg)
		for k < to && (backRow.Text[k] != frontRow.Text[k] || backRow.Style[k] != frontRow.Style[k]) {
			style := backRow.Style[k]
			displaySetStyle(style)
			run := k
			for run < to && backRow.Style[run] == style &&
				(backRow.Text[run] != frontRow.Text[run] || backRow.Style[run] != frontRow.Style[run]) {
				run++
			}
			screenWriteSpan(backRow.Text[k:run], run-k)
			// Update frontScreen
			for j := k; j < run; j++ {
				frontRow.Text[j] = backRow.Text[j]
				frontRow.Style[j] = backRow.Style[j]
			}
			k = run
		}
		_ = seg
	}
}

// rowSync synchronizes one dirty row to the terminal using diff logic.
func rowSync(row int) {
	backRow := &backScreen.Rows[row]
	frontRow := &frontScreen.Rows[row]

	// Find first differing column
	first := 0
	for first < term.Cols() && backRow.Text[first] == frontRow.Text[first] && backRow.Style[first] == frontRow.Style[first] {
		first++
	}
	if first == term.Cols() {
		return // No differences
	}

	// Find last differing column
	last := term.Cols() - 1
	for last > first && backRow.Text[last] == frontRow.Text[last] && backRow.Style[last] == frontRow.Style[last] {
		last--
	}

	// Scan from right to find where trailing normal-style spaces begin
	normalStyle := app.State.Theme.NormalStyle
	lastsp := term.Cols()
	for lastsp > first && backRow.Text[lastsp-1] == ' ' && backRow.Style[lastsp-1] == normalStyle {
		lastsp--
	}

	if lastsp <= last {
		// Trailing region is normal-style spaces; emit up to lastsp then erase-to-eol
		rowEmitDiffs(row, first, lastsp)
		term.Move(row, lastsp)
		displaySetStyle(normalStyle)
		term.EraseEol()
		for i := lastsp; i < term.Cols(); i++ {
			frontRow.Text[i] = ' '
			frontRow.Style[i] = normalStyle
		}
	} else {
		// Just emit the changed range
		rowEmitDiffs(row, first, last+1)
	}
}

// ScreenSync synchronizes all dirty rows from backScreen to the terminal.
func ScreenSync() {
	// If screen is dirty (e.g. after resize): reset front screen and clear terminal
	if app.State.ScreenDirty {
		normalStyle := app.State.Theme.NormalStyle
		for i := range backScreen.Rows {
			backScreen.Rows[i].Dirty = true
			frontRow := &frontScreen.Rows[i]
			for j := 0; j < term.Cols(); j++ {
				frontRow.Text[j] = ' '
				frontRow.Style[j] = normalStyle
			}
		}
		term.Move(0, 0)
		displaySetStyle(normalStyle)
		term.EraseEos()
		app.State.ScreenDirty = false
		app.State.MessagePresent = false
	}

	// Sync dirty body rows and the message line when active.
	syncRows := term.Rows()
	if app.State.MessagePresent && term.Rows() < len(backScreen.Rows) {
		syncRows = term.Rows() + 1
	}
	for i := 0; i < syncRows; i++ {
		if i >= len(backScreen.Rows) {
			break
		}
		if backScreen.Rows[i].Dirty {
			backScreen.Rows[i].Dirty = false
			rowSync(i)
		}
	}

	displaySetStyle(app.State.Theme.NormalStyle)
	term.Move(int(app.State.Cursor.Row), int(app.State.Cursor.Col))
	term.Flush()

	if pasteRepaintPending {
		pasteRepaintPending = false
		term.PingRepaint()
	}
}

// ---- Selection -------------------------------------------------------------------

// selectionStyle computes the highlight style for selected text.
func selectionStyle(base TextStyle) TextStyle {
	flags := base & TextStyleBold
	return MakeTextStyle(TermColorBase03, app.State.Theme.SelectionBg, flags)
}

// selInit initialises selection state from the window's mark and cursor.
func selInit(ss *SelState, wp *Window) {
	ss.active = false
	ss.phase = selectionBefore
	if wp.Mark.Line == 0 {
		return
	}
	markLine := wp.Mark.Line
	dotLine := wp.Cursor.Line
	marko := wp.Mark.Offset
	doto := wp.Cursor.Offset

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

	if wp.TopLine > ss.endLine {
		ss.active = false
	} else if wp.TopLine > ss.startLine {
		ss.phase = selectionInside
	}
}

// selLine returns the byte range [s, e) to highlight for lineNumber.
// Returns s==-1 if no selection on this line.
func selLine(ss *SelState, lineNumber uint, lp *Line) (s, e int) {
	s = -1
	e = 0
	if !ss.active {
		return
	}
	length := int(lp.Len())

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
	if !app.State.PhantomCursorValid {
		return
	}
	row := int(app.State.PhantomCursor.Row)
	col := int(app.State.PhantomCursor.Col)
	app.State.PhantomCursorValid = false
	if row < 0 || row >= len(backScreen.Rows) || col < 0 || col >= term.Cols() {
		return
	}
	backRow := &backScreen.Rows[row]
	backRow.Text[col] = phantomTextRune
	backRow.Style[col] = app.State.PhantomStyle
	backRow.Dirty = true
}

// overlayPhantomCursor paints a block cursor at the editor cursor position.
func overlayPhantomCursor() {
	if !app.State.ShowPhantomCursor {
		return
	}
	row := int(app.State.Cursor.Row)
	col := int(app.State.Cursor.Col)
	if row < 0 || row >= len(backScreen.Rows) || col < 0 || col >= term.Cols() {
		return
	}
	backRow := &backScreen.Rows[row]
	app.State.PhantomCursor = app.State.Cursor
	phantomTextRune = backRow.Text[col]
	app.State.PhantomText = byte(backRow.Text[col])
	app.State.PhantomStyle = backRow.Style[col]
	app.State.PhantomCursorValid = true
	backRow.Style[col] = selectionStyle(backRow.Style[col]) | TextStyleBold
	backRow.Dirty = true
}

// ---- Gutter rendering -----------------------------------------------------------

// screenPutLineno renders a line-number gutter of width columns.
// Format: [git-marker][right-justified line number][left-clipped indicator]
func screenPutLineno(width, lineno int, marker GitLineDiff, leftClipped bool) {
	// Save draw and active styles
	savedActiveStyle := app.State.ActiveStyle
	savedDrawStyle := drawStyle

	gutterStyle := app.State.Theme.GutterStyle
	displaySetStyle(gutterStyle)
	drawStyle = gutterStyle

	// Git marker glyph (first column)
	if marker != GitLineDiffNone {
		var glyph rune = ' '
		var glyphStyle TextStyle
		bg := TextStyleBg(gutterStyle)
		switch marker {
		case GitLineDiffAdded:
			glyph = '+'
			glyphStyle = MakeTextStyle(TermColorGreen, bg, TextStyleBold)
		case GitLineDiffModified:
			glyph = '~'
			glyphStyle = MakeTextStyle(TermColorYellow, bg, TextStyleBold)
		case GitLineDiffDeleted:
			glyph = '-'
			glyphStyle = MakeTextStyle(TermColorRed, bg, TextStyleBold)
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
	screenPutLineno(gutter, 0, GitLineDiffNone, false)
	screenEraseEol()
}

// ---- Content rendering ----------------------------------------------------------

// windowCursorScreenCol returns the screen column of the cursor in wp.
func windowCursorScreenCol(wp *Window) int {
	if wp == nil || wp.Buffer == nil {
		return 0
	}
	lp := wp.Buffer.Line(wp.Cursor.Line)
	if lp == nil {
		return 0
	}
	return lineColAtOffset(lp, wp.Cursor.Offset)
}

// screenPutPickerLine renders a full match-window row using the picker style.
func screenPutPickerLine(lp *Line) {
	style := app.State.Theme.PickerSelectionStyle
	savedDrawStyle := drawStyle
	drawStyle = style
	n := len(lp.Data)
	i := 0
	for i < n {
		c := rune(lp.Data[i])
		size := 1
		if lp.Data[i] >= 0x80 {
			r, sz := utf8.DecodeRune(lp.Data[i:n])
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
func screenPutLine(lp *Line, _ LangMode, _ *SynState, selStart, selEnd int) {
	syntax.SyntaxEnsureLine(lp)
	n := len(lp.Data)

	if n == 0 {
		if selStart >= 0 {
			displayPutGlyphStyle(' ', selectionStyle(app.State.Theme.NormalStyle))
		}
		return
	}

	savedDrawStyle := drawStyle
	i := 0  // byte offset
	ri := 0 // rune index
	for i < n {
		// Get syntax style for this rune
		style := app.State.Theme.NormalStyle
		if ri < len(lp.SyntaxStyles) {
			style = lp.SyntaxStyles[ri]
		}
		// Apply selection overlay
		if selStart >= 0 && i >= selStart && i < selEnd {
			style = selectionStyle(style)
		}
		drawStyle = style

		// Decode next rune
		c := rune(lp.Data[i])
		size := 1
		if lp.Data[i] >= 0x80 {
			r, sz := utf8.DecodeRune(lp.Data[i:n])
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
func renderLine(wp *Window, lineNumber uint, row int, synSt *SynState, selSt *SelState) {
	lp := wp.Buffer.Line(lineNumber)
	if lp == nil {
		return
	}
	gutter := int(wp.GutterWidth())
	marker := gitLineDiff(wp.Buffer, lineNumber)

	var ss, se int
	if selSt != nil {
		ss, se = selLine(selSt, lineNumber, lp)
	} else {
		ss = -1
	}

	screenMove(row, 0)
	// Content starts at column gutter; horizontal scroll shifts content left
	swCursorCol = gutter - int(wp.HScroll)
	tabOriginCol = swCursorCol
	oldClipLeft := clipLeftCol
	clipLeftCol = gutter

	if wp.Buffer != nil && wp.Buffer.Name == "*match*" && lp != nil && lp.Len() >= 2 && lp.Data[0] == '>' && lp.Data[1] == ' ' {
		screenPutPickerLine(lp)
	} else {
		screenPutLine(lp, wp.Buffer.LangMode, synSt, ss, se)
	}

	clipLeftCol = oldClipLeft
	// Clamp sw cursor to gutter (handles case where hscroll > line width)
	if swCursorCol < gutter {
		swCursorCol = gutter
	}
	screenEraseEol()

	// Render gutter last so it paints over any content that bled into gutter area
	screenMove(row, 0)
	screenPutLineno(gutter, int(lineNumber), marker, wp.HScroll > 0)
}

// ---- Modeline rendering ---------------------------------------------------------

// renderModeline renders the modeline for window wp.
func renderModeline(wp *Window) {
	if wp == nil || wp.Buffer == nil {
		return
	}
	bp := wp.Buffer
	row := int(wp.ScreenTopRow + wp.Height)
	if row >= term.Rows() {
		row = term.Rows() - 1
	}
	if row < 0 {
		return
	}

	const modelineHint = "Ctrl+/ = Menu"

	// EOL label
	eolLabel := "LF"
	switch bp.EolMode {
	case EModeCRLF:
		eolLabel = "CRLF"
	case EModeCR:
		eolLabel = "CR"
	}

	// Language label
	langLabel := modes.LangModeInfo(bp.LangMode).DisplayName

	// Position: percentage, line, column
	lineno := wp.Cursor.Line
	totallines := bp.LineCount
	var pct uint32
	if lineno >= totallines {
		pct = 100
	} else if totallines > 0 {
		pct = uint32((lineno * 100) / totallines)
	}
	colno := uint32(windowCursorScreenCol(wp)) + 1

	// Format position text: "  1% L1 C1" (3-char pct field)
	digits := 1
	if pct >= 100 {
		digits = 3
	} else if pct >= 10 {
		digits = 2
	}
	pad := 3 - digits
	positionText := ""
	for i := 0; i < pad; i++ {
		positionText += " "
	}
	positionText += fmt.Sprintf("%d%% L%d C%d", pct, lineno, colno)

	// Styles
	gutterStyle := app.State.Theme.GutterStyle
	gutterBg := TextStyleBg(gutterStyle)
	nameStyle := MakeTextStyle(app.State.Theme.ModelineNameColor, gutterBg, TextStyleBold)
	dirtyStyle := MakeTextStyle(TermColorRed, gutterBg, TextStyleBold)

	gutterW := int(wp.GutterWidth())
	markerCol := gutterW + 79 - int(wp.HScroll)
	hintCol := term.Cols() - len(modelineHint)

	screenMove(row, 0)
	backScreen.Rows[row].Dirty = true
	drawStyle = gutterStyle

	// Dirty indicator
	if bp.IsChanged {
		displayPutBytesStyle([]byte("*"), dirtyStyle)
	} else {
		screenPutc(' ')
	}

	// Disk-changed indicator (modified buffer, user kept local edits)
	if !bp.DiskChangeNotifiedMtime.IsZero() {
		displayPutGlyphStyle('D', dirtyStyle)
	} else {
		screenPutc(' ')
	}

	// Macro recording indicator
	if app.State.IsRecording() {
		displayPutGlyphStyle('m', MakeTextStyle(TermColorRed, gutterBg, TextStyleBold))
	} else if int32(CTLX|')') != app.State.Keys[0] {
		displayPutGlyphStyle('m', nameStyle)
	} else {
		screenPutc(' ')
	}

	// Spot/mark indicator
	if marksState.Count > 0 {
		displayPutGlyphStyle('s', nameStyle)
	} else {
		screenPutc(' ')
	}

	// Search scope indicator
	displayPutGlyphStyle('b', nameStyle)

	// Editor version
	screenPutBytes([]byte("  jem " + Version + " | "))

	// File name
	if bp.Name != "" {
		displayPutBytesStyle([]byte(bp.Name), nameStyle)
	} else {
		displayPutBytesStyle([]byte("[No File]"), nameStyle)
	}

	screenPutBytes([]byte(" | "))
	screenPutBytes([]byte(eolLabel))
	screenPutBytes([]byte(" | "))
	screenPutBytes([]byte(langLabel))
	if gitText := gitModelineText(bp); gitText != "" {
		gitStyle := MakeTextStyle(app.State.Theme.ModelineNameColor, gutterBg, 0)
		screenPutBytes([]byte(" | "))
		displayPutBytesStyle([]byte(gitText), gitStyle)
	}
	screenPutBytes([]byte(" | "))
	screenPutBytes([]byte(positionText))

	// Padding with optional column-80 marker and hint
	showHint := swCursorCol < hintCol
	padEnd := term.Cols()
	if showHint {
		padEnd = hintCol
	}
	if showHint && markerCol >= hintCol {
		markerCol = -1
	}
	if markerCol < padEnd && (markerCol < swCursorCol || markerCol < gutterW) {
		markerCol = -1
	}
	for swCursorCol < padEnd {
		if swCursorCol == markerCol {
			screenPutc('^')
		} else {
			screenPutc(' ')
		}
	}
	if showHint {
		screenPutBytes([]byte(modelineHint))
	}
	for swCursorCol < term.Cols() {
		if swCursorCol == markerCol {
			screenPutc('^')
		} else {
			screenPutc(' ')
		}
	}
}

// ---- Main display update ---------------------------------------------------------

// DisplayUpdate refreshes the display from all dirty windows.
func DisplayUpdate() {
	applyPendingPaste()

	// Check for terminal resize
	if term.RefreshSize() {
		frontScreen = allocScreen(term.Rows())
		backScreen = allocScreen(term.Rows())
		app.WindowRetile()
		swCursorRow = 0
		swCursorCol = 0
		app.State.ScreenDirty = true
		app.State.MessagePresent = false
	}

	if len(backScreen.Rows) < term.Rows() {
		frontScreen = allocScreen(term.Rows())
		backScreen = allocScreen(term.Rows())
	}

	if app.State.ScreenDirty {
		for i := 0; i < int(app.State.WindowCount); i++ {
			wp := app.State.WINDOWS[i]
			if wp != nil {
				wp.ShouldRedraw = true
				wp.ShouldUpdateModeLine = true
			}
		}
		app.State.PhantomCursorValid = false
	}

	restorePhantomCursor()

	// Keep current window's modeline live (updates L/C on cursor motion)
	if app.State.CurrentWindow != nil {
		app.State.CurrentWindow.ShouldUpdateModeLine = true
	}

	for wi := 0; wi < int(app.State.WindowCount); wi++ {
		wp := app.State.WINDOWS[wi]
		if wp == nil || wp.Buffer == nil {
			continue
		}

		if !wp.ShouldReframe && !wp.DidMove && !wp.DidEdit && !wp.ShouldRedraw && !wp.ShouldUpdateModeLine {
			continue
		}

		oldTopLine := wp.TopLine

		// Reframe: check whether cursor is visible in the current viewport
		if !wp.ShouldReframe {
			cursorVisible := false
			visLine := wp.TopLine
			for i := uint32(0); i < wp.Height; i++ {
				if visLine == wp.Cursor.Line {
					cursorVisible = true
					break
				}
				if visLine > wp.Buffer.LineCount {
					break
				}
				visLine++
			}
			if !cursorVisible {
				wp.ShouldReframe = true
			}
		}

		if wp.ShouldReframe {
			wp.CenterCursor()
			wp.ShouldRedraw = true
		}

		// Adjust horizontal scroll for the current window to keep cursor visible
		if wp == app.State.CurrentWindow {
			gutterW := int(wp.GutterWidth())
			cc := windowCursorScreenCol(wp)
			visible := term.Cols() - gutterW
			margin := visible / 4
			newHScroll := int(wp.HScroll)
			if cc < newHScroll {
				if cc > margin {
					newHScroll = cc - margin
				} else {
					newHScroll = 0
				}
			} else if cc >= newHScroll+visible {
				newHScroll = cc - visible + margin + 1
			}
			if newHScroll < 0 {
				newHScroll = 0
			}
			if uint32(newHScroll) != wp.HScroll {
				wp.HScroll = uint32(newHScroll)
				wp.ShouldRedraw = true
			}
		}

		gutterW := int(wp.GutterWidth())

		// When region is active, force full redraw so selection highlights all affected lines
		if wp.Mark.Line != 0 && !wp.ShouldRedraw {
			wp.DidEdit = false
			wp.DidMove = false
			wp.ShouldRedraw = true
		}

		if wp.DidEdit && !wp.DidMove && !wp.ShouldRedraw {
			// Fast path: only re-render the cursor line
			cursorRow := int(wp.ScreenTopRow) + int(wp.Cursor.Line-wp.TopLine)
			var synSt SynState
			var selSt SelState
			selInit(&selSt, wp)
			renderLine(wp, wp.Cursor.Line, cursorRow, &synSt, &selSt)
		} else if wp.DidEdit || wp.ShouldRedraw {
			// Compute scroll delta
			var scrollN int
			if oldTopLine != wp.TopLine {
				delta := int(oldTopLine) - int(wp.TopLine)
				height := int(wp.Height)
				if delta < 0 {
					delta = -delta
				}
				if delta >= height {
					scrollN = height + 1 // force full redraw
				} else {
					scrollN = int(wp.TopLine) - int(oldTopLine)
				}
			}

			// Apply terminal scroll if partial scroll
			if scrollN != 0 && scrollN > -int(wp.Height) && scrollN < int(wp.Height) {
				top := int(wp.ScreenTopRow)
				height := int(wp.Height)
				absN := scrollN
				if absN < 0 {
					absN = -absN
				}
				if absN > 0 && absN < height {
					clearFrom := top + height - absN
					clearTo := top + height
					if scrollN < 0 {
						clearFrom = top
						clearTo = top + absN
					}
					// Rotate back/front screen row slices
					if scrollN > 0 {
						screenRowsScrollUp(&backScreen.Rows, top, height, absN)
						screenRowsScrollUp(&frontScreen.Rows, top, height, absN)
					} else {
						screenRowsScrollDown(&backScreen.Rows, top, height, absN)
						screenRowsScrollDown(&frontScreen.Rows, top, height, absN)
					}
					// Clear the exposed rows on both screens
					normalStyle := app.State.Theme.NormalStyle
					for clearRow := clearFrom; clearRow < clearTo; clearRow++ {
						if clearRow < len(frontScreen.Rows) {
							for j := 0; j < term.Cols(); j++ {
								frontScreen.Rows[clearRow].Text[j] = ' '
								frontScreen.Rows[clearRow].Style[j] = normalStyle
							}
						}
					}
					// Scroll the terminal
					displaySetStyle(app.State.Theme.NormalStyle)
					term.ScrollRows(top, top+height-1, scrollN)
					// Repaint exposed rows explicitly
					for clearRow := clearFrom; clearRow < clearTo; clearRow++ {
						term.Move(clearRow, 0)
						term.EraseEol()
					}
				}
			}

			// Full render of all rows in this window
			var synSt SynState
			var selSt SelState
			selInit(&selSt, wp)
			lineNumber := wp.TopLine
			for r := uint32(0); r < wp.Height; r++ {
				row := int(wp.ScreenTopRow + r)
				if row >= term.Rows() {
					break
				}
				if lineNumber <= wp.Buffer.LineCount {
					renderLine(wp, lineNumber, row, &synSt, &selSt)
					lineNumber++
				} else {
					renderBlankRow(row, gutterW)
				}
			}
		}

		if wp.ShouldUpdateModeLine {
			renderModeline(wp)
		}

		wp.ShouldReframe = false
		wp.ForceReframe = false
		wp.DidMove = false
		wp.DidEdit = false
		wp.ShouldRedraw = false
		wp.ShouldUpdateModeLine = false
	}

	// Compute hardware cursor position (interactive minibuffer owns the cursor).
	if app.State.ActiveMinibuffer == nil {
		if app.State.CurrentWindow != nil {
			cw := app.State.CurrentWindow
			cursorLineDelta := cw.Cursor.Line - cw.TopLine
			app.State.Cursor.Row = cw.ScreenTopRow + uint32(cursorLineDelta)

			gutterW := int(cw.GutterWidth())
			contentCol := windowCursorScreenCol(cw)
			hscroll := int(cw.HScroll)
			cursorCol := gutterW + contentCol - hscroll
			if gutterW >= term.Cols() {
				cursorCol = term.Cols() - 1
			} else {
				if cursorCol < gutterW {
					cursorCol = gutterW
				}
				if cursorCol >= term.Cols() {
					cursorCol = term.Cols() - 1
				}
			}
			app.State.Cursor.Col = uint32(cursorCol)
		}
		overlayPhantomCursor()
	}
	ScreenSync()
}

// CmdRefresh forces a full screen refresh on the next DisplayUpdate.
func CmdRefresh(f bool, n int) bool {
	app.State.ScreenDirty = true
	return true
}

// CmdThemeToggle switches between dark and light theme palettes.
func CmdThemeToggle(f bool, n int) bool {
	_ = f
	_ = n
	theme := &app.State.Theme
	if theme.Mode == ThemeDark {
		theme.Mode = ThemeLight
	} else {
		theme.Mode = ThemeDark
	}
	themeUpdate()
	app.State.ScreenDirty = true
	for i := 0; i < int(app.State.WindowCount); i++ {
		wp := app.State.WINDOWS[i]
		if wp != nil {
			wp.ShouldRedraw = true
			wp.ShouldUpdateModeLine = true
		}
	}
	if theme.Mode == ThemeLight {
		mbWrite("[light mode]")
	} else {
		mbWrite("[dark mode]")
	}
	return true
}

// ---- Screen row scroll helpers ---------------------------------------------------

func screenRowsScrollUp(rows *[]ScreenRow, start, length, n int) {
	reverseScreenRows(rows, start, start+n-1)
	reverseScreenRows(rows, start+n, start+length-1)
	reverseScreenRows(rows, start, start+length-1)
}

func screenRowsScrollDown(rows *[]ScreenRow, start, length, n int) {
	screenRowsScrollUp(rows, start, length, length-n)
}

func reverseScreenRows(rows *[]ScreenRow, lo, hi int) {
	for lo < hi {
		(*rows)[lo], (*rows)[hi] = (*rows)[hi], (*rows)[lo]
		lo++
		hi--
	}
}

// ---- Line cache -----------------------------------------------------------------

func ensureLineCache(lp *Line) {
	if lp == nil || lp.CacheValid {
		return
	}
	bs := lp.Data
	capNeeded := len(bs)/2 + 1
	var runes []rune
	if v := runeSlicePool.Get(); v != nil {
		r := v.([]rune)
		if cap(r) >= capNeeded {
			runes = r[:0]
		} else {
			runes = make([]rune, 0, capNeeded)
		}
	} else {
		runes = make([]rune, 0, capNeeded)
	}
	var widths []int8
	if v := widthSlicePool.Get(); v != nil {
		w := v.([]int8)
		if cap(w) >= capNeeded {
			widths = w[:0]
		} else {
			widths = make([]int8, 0, capNeeded)
		}
	} else {
		widths = make([]int8, 0, capNeeded)
	}

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

// lineMeasureAdvance returns the screen column after rendering one character.
// Mirrors src/display.c line_measure_advance.
func lineMeasureAdvance(col int, c rune) int {
	if c == '\t' {
		col |= 0x07
		return col + 1
	}
	if c < 0x20 || c == 0x7F {
		return col + 2
	}
	if c < 0x80 {
		return col + 1
	}
	w := runewidth.RuneWidth(c)
	if w > 0 {
		return col + w
	}
	return col + 1
}

// lineColAtOffset returns the screen column corresponding to byte offset in lp.
func lineColAtOffset(lp *Line, offset uint) int {
	if lp == nil {
		return 0
	}
	col := 0
	i := uint(0)
	for i < offset && i < lp.Len() {
		b := lp.Data[i]
		if b < 0x80 {
			col = lineMeasureAdvance(col, rune(b))
			i++
			continue
		}
		r, size := utf8.DecodeRune(lp.Data[i:])
		if r == utf8.RuneError && size == 1 {
			col = lineMeasureAdvance(col, rune(b))
			i++
			continue
		}
		col = lineMeasureAdvance(col, r)
		i += uint(size)
	}
	return col
}

// ---- Debug/compatibility stubs --------------------------------------------------

// RenderModeline is a public wrapper for renderModeline (called from minibuf.go etc.)
func RenderModeline(wp *Window) {
	renderModeline(wp)
}

// debugDisplayLog writes a diagnostic message when debug logging is enabled.
var debugLogs bool

func debugDisplayLog(format string, args ...any) {
	if !debugLogs {
		return
	}
	f, err := os.OpenFile("/tmp/jem-display.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, time.Now().Format(time.RFC3339Nano)+" "+format+"\n", args...)
}

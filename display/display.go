package display

import (
	"unicode/utf8"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/term"
	"github.com/mattn/go-runewidth"
)

type ScreenRow struct {
	Dirty bool
	Text  []rune
	Style []buffer.TextStyle
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
	startLine int
	startO    int
	endLine   int
	endO      int
	phase     SelectionPhase
}

// ---- Globals ---------------------------------------------------------------------

var (
	frontScreen Screen
	backScreen  Screen
)

// Virtual screen cursor and style
var swCursorRow, swCursorCol int
var drawStyle buffer.TextStyle
var tabOriginCol int
var clipLeftCol int

// phantomTextRune stores the full rune for phantom cursor restore (types.go only has byte)
var phantomTextRune rune

// ---- Init ------------------------------------------------------------------------

// DisplayInit initializes the display and synchronizes the terminal size.
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
	Active.Theme.Mode = ThemeDark
	ThemeUpdate()
}

// ThemeUpdate recomputes cached palette colors from the active theme mode.
func ThemeUpdate() {
	theme := &Active.Theme
	if theme.Mode == ThemeLight {
		theme.NormalStyle = buffer.MakeTextStyle(buffer.TermColorBase00, buffer.TermColorBase3, 0)
		theme.CommentStyle = buffer.MakeTextStyle(buffer.TermColorBase1, buffer.TermColorBase3, 0)
		theme.GutterStyle = buffer.MakeTextStyle(buffer.TermColorBase1, buffer.TermColorBase2, 0)
		theme.SelectionBg = buffer.TermColorYellow
		theme.ModelineNameColor = buffer.TermColorBase01
		theme.PickerSelectionStyle = buffer.MakeTextStyle(buffer.TermColorBase03, theme.SelectionBg, 0)
	} else {
		theme.NormalStyle = buffer.MakeTextStyle(buffer.TermColorBase0, buffer.TermColorBase03, 0)
		theme.CommentStyle = buffer.MakeTextStyle(buffer.TermColorBase01, buffer.TermColorBase03, 0)
		theme.GutterStyle = buffer.TextStyleGutter
		theme.SelectionBg = buffer.TermColorBlue
		theme.ModelineNameColor = buffer.TermColorBase1
		theme.PickerSelectionStyle = buffer.MakeTextStyle(buffer.TermColorBase3, theme.SelectionBg, 0)
	}
	term.ClearStyleCache()
	for _, s := range []buffer.TextStyle{
		theme.NormalStyle,
		theme.CommentStyle,
		theme.GutterStyle,
		theme.PickerSelectionStyle,
		buffer.MakeTextStyle(theme.ModelineNameColor, theme.GutterStyle.Bg(), buffer.TextStyleBold),
		buffer.MakeTextStyle(buffer.TermColorRed, theme.GutterStyle.Bg(), buffer.TextStyleBold),
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
		s.Rows[i].Style = make([]buffer.TextStyle, cols)
		for j := 0; j < cols; j++ {
			s.Rows[i].Text[j] = ' '
			s.Rows[i].Style[j] = Active.Theme.NormalStyle
		}
	}
	return s
}

// ---- Virtual screen style --------------------------------------------------------

// displaySetStyle applies style to the terminal only when it differs from ActiveStyle.
func displaySetStyle(style buffer.TextStyle) {
	if Active.ActiveStyle != style {
		term.SetStyle(style)
		Active.ActiveStyle = style
	}
}

// ---- Virtual screen writes -------------------------------------------------------

// screenMove positions the software cursor and resets drawStyle to NormalStyle.
func screenMove(row, col int) {
	swCursorRow = row
	swCursorCol = col
	drawStyle = Active.Theme.NormalStyle
}

// screenSetStyle sets drawStyle without moving the cursor.
func screenSetStyle(style buffer.TextStyle) {
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

	w := runewidth.RuneWidth(c)

	if swCursorCol < clipLeftCol {
		// Before visible content area: advance counter but don't write
		swCursorCol += w
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
		// Always emit at least one space, then continue until aligned to an
		// 8-column boundary from tabOriginCol.
		for {
			screenPutRaw(' ')
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
		Active.Cursor.Row = term.Rows()
		Active.Cursor.Col = cursorCol
	}
	term.Move(row, cursorCol)
	term.Flush()
}

// displayPutBytesStyle writes bytes with a temporary drawStyle.
func displayPutBytesStyle(s []byte, style buffer.TextStyle) {
	saved := drawStyle
	drawStyle = style
	screenPutBytes(s)
	drawStyle = saved
}

// displayPutGlyphStyle writes one codepoint with a temporary drawStyle.
func displayPutGlyphStyle(c rune, style buffer.TextStyle) {
	saved := drawStyle
	drawStyle = style
	screenPutRaw(c)
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
	normalStyle := Active.Theme.NormalStyle
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
	if Active.ScreenDirty {
		normalStyle := Active.Theme.NormalStyle
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
		Active.ScreenDirty = false
		Active.MessagePresent = false
	}

	// Sync dirty body rows and the message line when active.
	syncRows := term.Rows()
	if Active.MessagePresent && term.Rows() < len(backScreen.Rows) {
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

	displaySetStyle(Active.Theme.NormalStyle)
	term.Move(Active.Cursor.Row, Active.Cursor.Col)
	term.Flush()

	if pasteRepaintPending {
		pasteRepaintPending = false
		term.PingRepaint()
	}
}

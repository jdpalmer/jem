package display

import (
	"fmt"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/mode"
	"github.com/jdpalmer/jem/term"
)

func renderModeline(win *window.Window) {
	if win == nil || win.Buffer == nil {
		return
	}
	buf := win.Buffer
	row := int(win.ScreenTopRow + win.Height)
	if row >= term.Rows() {
		row = term.Rows() - 1
	}
	if row < 0 {
		return
	}

	const modelineHint = "Ctrl+/ = Menu"

	// EOL label
	eolLabel := "LF"
	switch buf.EolMode {
	case buffer.EModeCRLF:
		eolLabel = "CRLF"
	case buffer.EModeCR:
		eolLabel = "CR"
	}

	// Language label
	langLabel := mode.LangModeInfo(buf.LangMode).DisplayName

	// Position: percentage, line, column
	lineno := win.Cursor.Line
	totallines := buf.LineCount
	var pct uint32
	if lineno >= totallines {
		pct = 100
	} else if totallines > 0 {
		pct = uint32((lineno * 100) / totallines)
	}
	colno := uint32(WindowCursorScreenCol(win)) + 1

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
	gutterStyle := Active.Theme.GutterStyle
	gutterBg := gutterStyle.Bg()
	nameStyle := buffer.MakeTextStyle(Active.Theme.ModelineNameColor, gutterBg, buffer.TextStyleBold)
	dirtyStyle := buffer.MakeTextStyle(buffer.TermColorRed, gutterBg, buffer.TextStyleBold)

	gutterW := int(win.GutterWidth())
	markerCol := gutterW + 79 - int(win.HScroll)
	hintCol := term.Cols() - len(modelineHint)

	screenMove(row, 0)
	backScreen.Rows[row].Dirty = true
	drawStyle = gutterStyle

	// Dirty indicator
	if buf.IsChanged {
		displayPutBytesStyle([]byte("*"), dirtyStyle)
	} else {
		screenPutc(' ')
	}

	// Disk-changed indicator (modified buffer, user kept local edits)
	if !buf.DiskChangeNotifiedMtime.IsZero() {
		displayPutGlyphStyle('D', dirtyStyle)
	} else {
		screenPutc(' ')
	}

	// Macro recording / stored-macro indicator
	if Active.MacroRecording {
		displayPutGlyphStyle('m', buffer.MakeTextStyle(buffer.TermColorRed, gutterBg, buffer.TextStyleBold))
	} else if Active.MacroPresent {
		displayPutGlyphStyle('m', nameStyle)
	} else {
		screenPutc(' ')
	}

	// Spot/mark indicator
	if len(markring.Active.Marks) > 0 {
		displayPutGlyphStyle('s', nameStyle)
	} else {
		screenPutc(' ')
	}

	// Search scope indicator
	displayPutGlyphStyle('b', nameStyle)

	// Editor version
	screenPutBytes([]byte("  jem " + Version + " | "))

	// File name (display-truncated; stored Name is unbounded)
	if buf.Name != "" {
		displayPutBytesStyle([]byte(FitBufferName(buf.Name, BufferNameMaxCols)), nameStyle)
	} else {
		displayPutBytesStyle([]byte("[No File]"), nameStyle)
	}

	screenPutBytes([]byte(" | "))
	screenPutBytes([]byte(eolLabel))
	screenPutBytes([]byte(" | "))
	screenPutBytes([]byte(langLabel))
	if gitText := gitModelineText(buf); gitText != "" {
		gitStyle := buffer.MakeTextStyle(Active.Theme.ModelineNameColor, gutterBg, 0)
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
	// Check for terminal resize
	if term.RefreshSize() {
		frontScreen = allocScreen(term.Rows())
		backScreen = allocScreen(term.Rows())
		window.WindowRetile()
		swCursorRow = 0
		swCursorCol = 0
		Active.ScreenDirty = true
		Active.MessagePresent = false
	}

	if len(backScreen.Rows) < term.Rows() {
		frontScreen = allocScreen(term.Rows())
		backScreen = allocScreen(term.Rows())
	}

	if Active.ScreenDirty {
		for i := 0; i < int(len(window.Active.Windows)); i++ {
			win := window.Active.Windows[i]
			if win != nil {
				win.ShouldRedraw = true
				win.ShouldUpdateModeLine = true
			}
		}
		Active.PhantomCursorValid = false
	}

	restorePhantomCursor()

	// Keep current window's modeline live (updates L/C on cursor motion)
	if window.Active.CurrentWindow != nil {
		window.Active.CurrentWindow.ShouldUpdateModeLine = true
	}

	for wi := 0; wi < int(len(window.Active.Windows)); wi++ {
		win := window.Active.Windows[wi]
		if win == nil || win.Buffer == nil {
			continue
		}

		if !win.ShouldReframe && !win.DidMove && !win.DidEdit && !win.ShouldRedraw && !win.ShouldUpdateModeLine {
			continue
		}

		oldTopLine := win.TopLine

		// Reframe: check whether cursor is visible in the current viewport
		if !win.ShouldReframe {
			cursorVisible := false
			visLine := win.TopLine
			for i := uint32(0); i < win.Height; i++ {
				if visLine == win.Cursor.Line {
					cursorVisible = true
					break
				}
				if visLine > win.Buffer.LineCount {
					break
				}
				visLine++
			}
			if !cursorVisible {
				win.ShouldReframe = true
			}
		}

		if win.ShouldReframe {
			win.CenterCursor()
			win.ShouldRedraw = true
		}

		// Adjust horizontal scroll for the current window to keep cursor visible
		if win == window.Active.CurrentWindow {
			gutterW := int(win.GutterWidth())
			cc := WindowCursorScreenCol(win)
			visible := term.Cols() - gutterW
			margin := visible / 4
			newHScroll := int(win.HScroll)
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
			if uint32(newHScroll) != win.HScroll {
				win.HScroll = uint32(newHScroll)
				win.ShouldRedraw = true
			}
		}

		gutterW := int(win.GutterWidth())

		// When region is active, force full redraw so selection highlights all affected lines
		if win.Mark.Line != 0 && !win.ShouldRedraw {
			win.DidEdit = false
			win.DidMove = false
			win.ShouldRedraw = true
		}

		if win.DidEdit && !win.DidMove && !win.ShouldRedraw {
			// Fast path: only re-render the cursor line
			cursorRow := int(win.ScreenTopRow) + int(win.Cursor.Line-win.TopLine)
			var synSt buffer.SynState
			var selSt SelState
			selInit(&selSt, win)
			renderLine(win, win.Cursor.Line, cursorRow, &synSt, &selSt)
		} else if win.DidEdit || win.ShouldRedraw {
			// Compute scroll delta
			var scrollN int
			if oldTopLine != win.TopLine {
				delta := int(oldTopLine) - int(win.TopLine)
				height := int(win.Height)
				if delta < 0 {
					delta = -delta
				}
				if delta >= height {
					scrollN = height + 1 // force full redraw
				} else {
					scrollN = int(win.TopLine) - int(oldTopLine)
				}
			}

			// Apply terminal scroll if partial scroll
			if scrollN != 0 && scrollN > -int(win.Height) && scrollN < int(win.Height) {
				top := int(win.ScreenTopRow)
				height := int(win.Height)
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
					normalStyle := Active.Theme.NormalStyle
					for clearRow := clearFrom; clearRow < clearTo; clearRow++ {
						if clearRow < len(frontScreen.Rows) {
							for j := 0; j < term.Cols(); j++ {
								frontScreen.Rows[clearRow].Text[j] = ' '
								frontScreen.Rows[clearRow].Style[j] = normalStyle
							}
						}
					}
					// Scroll the terminal
					displaySetStyle(Active.Theme.NormalStyle)
					term.ScrollRows(top, top+height-1, scrollN)
					// Repaint exposed rows explicitly
					for clearRow := clearFrom; clearRow < clearTo; clearRow++ {
						term.Move(clearRow, 0)
						term.EraseEol()
					}
				}
			}

			// Full render of all rows in this window
			var synSt buffer.SynState
			var selSt SelState
			selInit(&selSt, win)
			lineNumber := win.TopLine
			for r := uint32(0); r < win.Height; r++ {
				row := int(win.ScreenTopRow + r)
				if row >= term.Rows() {
					break
				}
				if lineNumber <= win.Buffer.LineCount {
					renderLine(win, lineNumber, row, &synSt, &selSt)
					lineNumber++
				} else {
					renderBlankRow(row, gutterW)
				}
			}
		}

		if win.ShouldUpdateModeLine {
			renderModeline(win)
		}

		win.ShouldReframe = false
		win.ForceReframe = false
		win.DidMove = false
		win.DidEdit = false
		win.ShouldRedraw = false
		win.ShouldUpdateModeLine = false
	}

	// Compute hardware cursor position (interactive minibuffer owns the cursor).
	if minibuffer.Active == nil {
		if window.Active.CurrentWindow != nil {
			cw := window.Active.CurrentWindow
			cursorLineDelta := cw.Cursor.Line - cw.TopLine
			Active.Cursor.Row = cw.ScreenTopRow + uint32(cursorLineDelta)

			gutterW := int(cw.GutterWidth())
			contentCol := WindowCursorScreenCol(cw)
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
			Active.Cursor.Col = uint32(cursorCol)
		}
		overlayPhantomCursor()
	}
	ScreenSync()
}

// ---- Screen row scroll helpers ---------------------------------------------------

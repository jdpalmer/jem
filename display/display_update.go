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

func renderModeline(wp *window.Window) {
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
	case buffer.EModeCRLF:
		eolLabel = "CRLF"
	case buffer.EModeCR:
		eolLabel = "CR"
	}

	// Language label
	langLabel := mode.LangModeInfo(bp.LangMode).DisplayName

	// Position: percentage, line, column
	lineno := wp.Cursor.Line
	totallines := bp.LineCount
	var pct uint32
	if lineno >= totallines {
		pct = 100
	} else if totallines > 0 {
		pct = uint32((lineno * 100) / totallines)
	}
	colno := uint32(WindowCursorScreenCol(wp)) + 1

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
	if bp.Name != "" {
		displayPutBytesStyle([]byte(FitBufferName(bp.Name, BufferNameMaxCols)), nameStyle)
	} else {
		displayPutBytesStyle([]byte("[No File]"), nameStyle)
	}

	screenPutBytes([]byte(" | "))
	screenPutBytes([]byte(eolLabel))
	screenPutBytes([]byte(" | "))
	screenPutBytes([]byte(langLabel))
	if gitText := gitModelineText(bp); gitText != "" {
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
			wp := window.Active.Windows[i]
			if wp != nil {
				wp.ShouldRedraw = true
				wp.ShouldUpdateModeLine = true
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
		wp := window.Active.Windows[wi]
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
		if wp == window.Active.CurrentWindow {
			gutterW := int(wp.GutterWidth())
			cc := WindowCursorScreenCol(wp)
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
			var synSt buffer.SynState
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

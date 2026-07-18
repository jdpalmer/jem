package editor

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/term"
)

// Type aliases for buffer/syntax/term symbols used throughout the editor package.
type (
	Buffer              = buffer.Buffer
	Line                = buffer.Line
	Location            = buffer.Location
	EolMode             = buffer.EolMode
	LangMode            = buffer.LangMode
	SynState            = buffer.SynState
	SyntaxLineSummary   = buffer.SyntaxLineSummary
	SyntaxContext       = buffer.SyntaxContext
	SyntaxDelimiterMask = buffer.SyntaxDelimiterMask
	SyntaxBlock         = buffer.SyntaxBlock
	TextStyle           = buffer.TextStyle
	TermColor           = buffer.TermColor
	UndoHistory         = buffer.UndoHistory
	UndoKind            = buffer.UndoKind
)

const (
	EModeLF   = buffer.EModeLF
	EModeCRLF = buffer.EModeCRLF
	EModeCR   = buffer.EModeCR

	SyntaxContextNone    = buffer.SyntaxContextNone
	SyntaxContextCode    = buffer.SyntaxContextCode
	SyntaxContextString  = buffer.SyntaxContextString
	SyntaxContextComment = buffer.SyntaxContextComment
	SyntaxContextPreproc = buffer.SyntaxContextPreproc

	SyntaxDelimParen   = buffer.SyntaxDelimParen
	SyntaxDelimBracket = buffer.SyntaxDelimBracket
	SyntaxDelimCurly   = buffer.SyntaxDelimCurly
	SyntaxDelimAll     = buffer.SyntaxDelimAll

	UndoDelete = buffer.UndoDelete
	UndoInsert = buffer.UndoInsert

	TermColorBlack   = buffer.TermColorBlack
	TermColorRed     = buffer.TermColorRed
	TermColorGreen   = buffer.TermColorGreen
	TermColorYellow  = buffer.TermColorYellow
	TermColorBlue    = buffer.TermColorBlue
	TermColorMagenta = buffer.TermColorMagenta
	TermColorCyan    = buffer.TermColorCyan
	TermColorWhite   = buffer.TermColorWhite
	TermColorDefault = buffer.TermColorDefault
	TermColorBase03  = buffer.TermColorBase03
	TermColorBase02  = buffer.TermColorBase02
	TermColorBase01  = buffer.TermColorBase01
	TermColorBase00  = buffer.TermColorBase00
	TermColorBase0   = buffer.TermColorBase0
	TermColorBase1   = buffer.TermColorBase1
	TermColorBase2   = buffer.TermColorBase2
	TermColorBase3   = buffer.TermColorBase3

	TextStyleFgShift   = buffer.TextStyleFgShift
	TextStyleBgShift   = buffer.TextStyleBgShift
	TextStyleColorMask = buffer.TextStyleColorMask
	TextStyleBold      = buffer.TextStyleBold
	TextStyleUnderline = buffer.TextStyleUnderline
	TextStyleReverse   = buffer.TextStyleReverse

	CTL   = term.CTL
	META  = term.META
	CTLX  = term.CTLX
	SHIFT = term.SHIFT

	KeyMask = term.KeyMask

	KeyUp       = term.KeyUp
	KeyDown     = term.KeyDown
	KeyLeft     = term.KeyLeft
	KeyRight    = term.KeyRight
	KeyTab      = term.KeyTab
	KeyEnter    = term.KeyEnter
	KeyHome     = term.KeyHome
	KeyEnd      = term.KeyEnd
	KeyPageUp   = term.KeyPageUp
	KeyPageDown = term.KeyPageDown
	KeyDelete   = term.KeyDelete

	MouseLeft      = term.MouseLeft
	MouseWheelUp   = term.MouseWheelUp
	MouseWheelDown = term.MouseWheelDown
	MouseDrag      = term.MouseDrag

	UnicodeLimit = term.UnicodeLimit
)

var (
	TextStyleDefault = buffer.TextStyleDefault
	TextStyleGutter  = buffer.TextStyleGutter
	MakeTextStyle    = buffer.MakeTextStyle
	TextStyleFg      = buffer.TextStyle.Fg
	TextStyleBg      = buffer.TextStyle.Bg
)

func bufferSetText(bp *Buffer, begin, end Location, newText []byte, newEndOut *Location, kill bool) bool {
	if kill {
		oldText := bp.GetText(begin, end)
		if len(oldText) > 0 && !killAppend(oldText) {
			return false
		}
	}
	ok := bp.SetText(&editorUndo, begin, end, newText, newEndOut)
	if kill && ok {
		killWriteClipboard()
	}
	return ok
}

func bufferAdjustLocationsAfterReplace(bp *Buffer, begin, end, newEnd Location) {
	for i := 0; i < int(app.State.WindowCount); i++ {
		wp := app.State.WINDOWS[i]
		if wp == nil || wp.Buffer != bp {
			continue
		}
		wp.Cursor.AdjustAfterReplace(begin, end, newEnd)
		wp.Mark.AdjustAfterReplace(begin, end, newEnd)
		if wp.TopLine >= begin.Line {
			if wp.TopLine > end.Line {
				if newEnd.Line >= end.Line {
					wp.TopLine += newEnd.Line - end.Line
				} else {
					removed := end.Line - newEnd.Line
					if wp.TopLine >= removed {
						wp.TopLine -= removed
					} else {
						wp.TopLine = 1
					}
				}
			} else {
				wp.TopLine = begin.Line
			}
		}
	}
}

func bufferNoteEdit(bp *Buffer, isStructural bool) {
	firstChange := !bp.IsChanged
	shouldRedraw := isStructural
	count := 0
	for i := 0; i < int(app.State.WindowCount); i++ {
		wp := app.State.WINDOWS[i]
		if wp != nil && wp.Buffer == bp {
			count++
		}
	}
	if count != 1 {
		shouldRedraw = true
	}
	for i := 0; i < int(app.State.WindowCount); i++ {
		wp := app.State.WINDOWS[i]
		if wp == nil || wp.Buffer != bp {
			continue
		}
		if shouldRedraw {
			wp.ShouldRedraw = true
		} else {
			wp.DidEdit = true
		}
		if firstChange {
			wp.ShouldUpdateModeLine = true
		}
	}
}

func initTermHooks() {
	term.PackageHooks = term.Hooks{
		OnMouse: func(col, row int) {
			app.State.Mouse.Col = uint32(col)
			app.State.Mouse.Row = uint32(row)
		},
		OnPaste: func(paste []byte) {
			queuePaste(paste)
		},
		OnResume: func() {
			if term.RefreshSize() {
				DisplayInitHeadless(term.Rows(), term.Cols())
			}
		},
	}
}

func initBufferSyntaxHooks() {
	buffer.PackageHooks = buffer.Hooks{
		NoteEdit:                    bufferNoteEdit,
		AdjustLocationsAfterReplace: bufferAdjustLocationsAfterReplace,
		InvalidateSyntaxFrom:        (*buffer.Buffer).InvalidateSyntaxFrom,
		ReparseFrom:                 syntax.IncrementalReparse,
	}
	syncSyntaxPalette()
}

func syncSyntaxPalette() {
	syntax.PackagePalette = syntax.Palette{
		NormalStyle:  app.State.Theme.NormalStyle,
		CommentStyle: app.State.Theme.CommentStyle,
	}
}

// DFA state constants for tests and mode code.
const (
	SS_NORMAL       = syntax.SS_NORMAL
	SS_IDENT        = syntax.SS_IDENT
	SS_NUMBER       = syntax.SS_NUMBER
	SS_STRING_D     = syntax.SS_STRING_D
	SS_STRING_D_ESC = syntax.SS_STRING_D_ESC
	SS_STRING_S     = syntax.SS_STRING_S
	SS_STRING_S_ESC = syntax.SS_STRING_S_ESC
	SS_CMT_LINE     = syntax.SS_CMT_LINE
	SS_CMT_BLOCK    = syntax.SS_CMT_BLOCK
	SS_CMT_STAR     = syntax.SS_CMT_STAR
	SS_CMT_BRACE    = syntax.SS_CMT_BRACE
	SS_CMT_PAREN    = syntax.SS_CMT_PAREN
	SS_CMT_PAREN2   = syntax.SS_CMT_PAREN2
	SS_PREPROC      = syntax.SS_PREPROC
	SS_LUA_DASH     = syntax.SS_LUA_DASH
	SS_LUA_BLOCK    = syntax.SS_LUA_BLOCK
	SS_LUA_BLKEND   = syntax.SS_LUA_BLKEND
	SS_HTML_CMT     = syntax.SS_HTML_CMT
	SS_HTML_CMT_D1  = syntax.SS_HTML_CMT_D1
	SS_HTML_CMT_D2  = syntax.SS_HTML_CMT_D2
)

var SyntaxDebug = syntax.SyntaxDebug

const (
	LModeNone         LangMode = buffer.LModeNone
	LModeC            LangMode = buffer.LModeC
	LModeJava         LangMode = buffer.LModeJava
	LModePython       LangMode = buffer.LModePython
	LModeLua          LangMode = buffer.LModeLua
	LModeLisp         LangMode = buffer.LModeLisp
	LModeMarkdown     LangMode = buffer.LModeMarkdown
	LModePascal       LangMode = buffer.LModePascal
	LModeVerilog      LangMode = buffer.LModeVerilog
	LModeMake         LangMode = buffer.LModeMake
	LModeSwift        LangMode = buffer.LModeSwift
	LModeJavaScript   LangMode = buffer.LModeJavaScript
	LModeActionScript LangMode = buffer.LModeActionScript
	LModeTypeScript   LangMode = buffer.LModeTypeScript
	LModeDart         LangMode = buffer.LModeDart
	LModeGo           LangMode = buffer.LModeGo
	LModeCSharp       LangMode = buffer.LModeCSharp
	LModeRust         LangMode = buffer.LModeRust
	LModeR            LangMode = buffer.LModeR
	LModeKotlin       LangMode = buffer.LModeKotlin
	LModeHTML         LangMode = buffer.LModeHTML
	LModeCSS          LangMode = buffer.LModeCSS
)

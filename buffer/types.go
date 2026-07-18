// Package buffer provides the text buffer model for jem.
package buffer

import "time"

const BufferNameCapacity = 16

type EolMode int

const (
	EModeLF   EolMode = 0
	EModeCRLF EolMode = 1
	EModeCR   EolMode = 2
)

type LangMode int

const (
	LModeNone         LangMode = 0
	LModeC            LangMode = 1
	LModeJava         LangMode = 2
	LModePython       LangMode = 3
	LModeLua          LangMode = 4
	LModeLisp         LangMode = 5
	LModeMarkdown     LangMode = 6
	LModePascal       LangMode = 7
	LModeVerilog      LangMode = 8
	LModeMake         LangMode = 9
	LModeSwift        LangMode = 10
	LModeJavaScript   LangMode = 11
	LModeActionScript LangMode = 12
	LModeTypeScript   LangMode = 13
	LModeDart         LangMode = 14
	LModeGo           LangMode = 15
	LModeCSharp       LangMode = 16
	LModeRust         LangMode = 17
	LModeR            LangMode = 18
	LModeKotlin       LangMode = 19
	LModeHTML         LangMode = 20
	LModeCSS          LangMode = 21
)

type SyntaxContext int

const (
	SyntaxContextNone    SyntaxContext = 0
	SyntaxContextCode    SyntaxContext = 1
	SyntaxContextString  SyntaxContext = 2
	SyntaxContextComment SyntaxContext = 3
	SyntaxContextPreproc SyntaxContext = 4
)

type SyntaxDelimiterMask uint8

const (
	SyntaxDelimParen   SyntaxDelimiterMask = 1 << 0
	SyntaxDelimBracket SyntaxDelimiterMask = 1 << 1
	SyntaxDelimCurly   SyntaxDelimiterMask = 1 << 2
	SyntaxDelimAll     SyntaxDelimiterMask = SyntaxDelimParen | SyntaxDelimBracket | SyntaxDelimCurly
)

type TextStyle uint16

type TermColor uint8

const (
	TermColorBlack   TermColor = 0
	TermColorRed     TermColor = 1
	TermColorGreen   TermColor = 2
	TermColorYellow  TermColor = 3
	TermColorBlue    TermColor = 4
	TermColorMagenta TermColor = 5
	TermColorCyan    TermColor = 6
	TermColorWhite   TermColor = 7
	TermColorDefault TermColor = 16

	TermColorBase03 TermColor = 17
	TermColorBase02 TermColor = 18
	TermColorBase01 TermColor = 19
	TermColorBase00 TermColor = 20
	TermColorBase0  TermColor = 21
	TermColorBase1  TermColor = 22
	TermColorBase2  TermColor = 23
	TermColorBase3  TermColor = 24
)

const (
	TextStyleFgShift   uint16    = 0
	TextStyleBgShift   uint16    = 5
	TextStyleColorMask uint16    = 0x001F
	TextStyleBold      TextStyle = 0x0400
	TextStyleUnderline TextStyle = 0x0800
	TextStyleReverse   TextStyle = 0x1000
)

func MakeTextStyle(fg, bg TermColor, flags TextStyle) TextStyle {
	return TextStyle((uint16(fg)&TextStyleColorMask)<<TextStyleFgShift |
		(uint16(bg)&TextStyleColorMask)<<TextStyleBgShift |
		uint16(flags))
}

func (style TextStyle) Fg() TermColor {
	return TermColor((uint16(style) >> TextStyleFgShift) & TextStyleColorMask)
}

func (style TextStyle) Bg() TermColor {
	return TermColor((uint16(style) >> TextStyleBgShift) & TextStyleColorMask)
}

var (
	TextStyleDefault = TextStyle((uint16(TermColorDefault) << TextStyleFgShift) |
		(uint16(TermColorDefault) << TextStyleBgShift))
	TextStyleGutter = TextStyle((uint16(TermColorBase01) << TextStyleFgShift) |
		(uint16(TermColorBase02) << TextStyleBgShift))
)

type Location struct {
	Line   uint
	Offset uint
}

func MakeLocation(line, offset uint) Location {
	return Location{Line: line, Offset: offset}
}

type SynState struct {
	DFA     uint8
	Paren   uint8
	Bracket uint8
	Curly   uint8
}

type SyntaxLineSummary struct {
	FirstCodeOffset uint
	OpenOffsets     [3]uint
	CloseOffsets    [3]uint
	OpenMask        uint8
	CloseMask       uint8
}

type Line struct {
	Data           []byte
	SyntaxEndState SynState
	SyntaxSummary  SyntaxLineSummary
	SyntaxValid    bool
	RuneCache      []rune
	WidthCache     []int8
	SyntaxStyles   []TextStyle
	CacheValid     bool
	Metadata       any
	LangMode       LangMode
	Buffer         *Buffer
}

func (lp *Line) Byte(n uint) byte {
	if lp == nil || n >= uint(len(lp.Data)) {
		return 0
	}
	return lp.Data[n]
}

func (lp *Line) Len() uint {
	if lp == nil {
		return 0
	}
	return uint(len(lp.Data))
}

type Buffer struct {
	Lines                   []Line
	LineCount               uint
	Serial                  uint32
	SavedUndoSerial         uint32
	IsChanged               bool
	IsReadonly              bool
	EolMode                 EolMode
	LangMode                LangMode
	FillCol                 uint32
	CIndent                 uint32
	CBrace                  uint32
	CColonOffset            uint32
	PyIndent                uint32
	PyContinuedOffset       uint32
	WhitespaceCleanup       bool
	Name                    string
	FileName                string
	FileMtime               time.Time
	DiskChangeNotifiedMtime time.Time
	Cursor                  Location // last-known cursor; windows own live cursor state
	Mark                    Location // Line == 0 means unset; otherwise 1-based line index
}

// EOF returns the location just past the last line (1-based lines).
// For an empty buffer this is line 1; with N lines it is line N+1.
func (bp *Buffer) EOF() uint {
	if bp == nil {
		return 1
	}
	return bp.LineCount + 1
}

// Line returns line lineNumber (1-based). The pointer is invalidated if
// bp.Lines is reallocated; prefer line numbers across edits.
func (bp *Buffer) Line(lineNumber uint) *Line {
	if bp == nil || lineNumber == 0 || lineNumber > bp.LineCount {
		return nil
	}
	return &bp.Lines[lineNumber-1]
}

type SyntaxBlock struct {
	Open       Location
	Close      Location
	HeaderLine uint
	Delimiter  byte
}

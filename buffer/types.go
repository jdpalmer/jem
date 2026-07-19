// Package buffer provides the text buffer model for jem.
package buffer

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

type SyntaxBlock struct {
	Open       Location
	Close      Location
	HeaderLine uint
	Delimiter  byte
}

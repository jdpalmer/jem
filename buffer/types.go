// Package buffer provides the text buffer model for jem.
package buffer

type EolMode int

const (
	EModeLF EolMode = iota
	EModeCRLF
	EModeCR
)

type LangMode int

const (
	LModeNone LangMode = iota
	LModeC
	LModeJava
	LModePython
	LModeLua
	LModeLisp
	LModeMarkdown
	LModePascal
	LModeVerilog
	LModeMake
	LModeSwift
	LModeJavaScript
	LModeActionScript
	LModeTypeScript
	LModeDart
	LModeGo
	LModeCSharp
	LModeRust
	LModeR
	LModeKotlin
	LModeHTML
	LModeCSS
)

// IndentConfig is language-agnostic indent style for a buffer.
type IndentConfig struct {
	Width     int // primary indent step
	Brace     int // extra indent for a standalone opening brace (C-like)
	Label     int // extra offset for case/default labels (C-like)
	Continued int // extra indent for continuation lines (Python-like)
}

type SyntaxContext int

const (
	SyntaxContextNone SyntaxContext = iota
	SyntaxContextCode
	SyntaxContextString
	SyntaxContextComment
	SyntaxContextPreproc
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

// OffsetUnset marks an unknown/absent byte offset in SyntaxLineSummary.
const OffsetUnset = -1

type SyntaxLineSummary struct {
	FirstCodeOffset int
	OpenOffsets     [3]int
	CloseOffsets    [3]int
	OpenMask        uint8
	CloseMask       uint8
}

type SyntaxBlock struct {
	Open       Location
	Close      Location
	HeaderLine int
	Delimiter  byte
}

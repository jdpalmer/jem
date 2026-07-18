package syntax

import "github.com/jdpalmer/jem/buffer"

type (
	TextStyle         = buffer.TextStyle
	TermColor         = buffer.TermColor
	SynState          = buffer.SynState
	SyntaxLineSummary = buffer.SyntaxLineSummary
	SyntaxContext     = buffer.SyntaxContext
	LangMode          = buffer.LangMode
)

const (
	TermColorBlack   = buffer.TermColorBlack
	TermColorRed     = buffer.TermColorRed
	TermColorGreen   = buffer.TermColorGreen
	TermColorYellow  = buffer.TermColorYellow
	TermColorBlue    = buffer.TermColorBlue
	TermColorMagenta = buffer.TermColorMagenta
	TermColorCyan    = buffer.TermColorCyan
	TermColorWhite   = buffer.TermColorWhite
	TermColorDefault = buffer.TermColorDefault
	TextStyleBold    = buffer.TextStyleBold
)

var (
	MakeTextStyle    = buffer.MakeTextStyle
	TextStyleFg      = buffer.TextStyle.Fg
	TextStyleDefault = buffer.TextStyleDefault
)

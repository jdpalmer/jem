package buffer

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

// MakeTextStyle creates a TextStyle with the given foreground color, background color, and flags.
func MakeTextStyle(fg, bg TermColor, flags TextStyle) TextStyle {
	return TextStyle((uint16(fg)&TextStyleColorMask)<<TextStyleFgShift |
		(uint16(bg)&TextStyleColorMask)<<TextStyleBgShift |
		uint16(flags))
}

// Fg returns the foreground color of the style.
func (style TextStyle) Fg() TermColor {
	return TermColor((uint16(style) >> TextStyleFgShift) & TextStyleColorMask)
}

// Bg returns the background color of the style.
func (style TextStyle) Bg() TermColor {
	return TermColor((uint16(style) >> TextStyleBgShift) & TextStyleColorMask)
}

var (
	TextStyleDefault = TextStyle((uint16(TermColorDefault) << TextStyleFgShift) |
		(uint16(TermColorDefault) << TextStyleBgShift))
	TextStyleGutter = TextStyle((uint16(TermColorBase01) << TextStyleFgShift) |
		(uint16(TermColorBase02) << TextStyleBgShift))
)

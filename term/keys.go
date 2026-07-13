package term

// Key encoding (uint32):
//   - CTL, META, CTLX, SHIFT — modifier flags in bits 24–27
//   - 0x100000xx — special keys (arrows, tab, enter, …)
//   - 0x200000xx — mouse events
//   - otherwise — Unicode code point (possibly with modifier flags OR'd in)

const (
	CTL   uint32 = 0x01000000
	META  uint32 = 0x02000000 // Alt/Option in Kitty CSI modifiers
	CTLX  uint32 = 0x04000000
	SHIFT uint32 = 0x08000000
)

const KeyMask = CTL | META | CTLX | SHIFT

const (
	KeyUp       uint32 = 0x10000001
	KeyDown     uint32 = 0x10000002
	KeyLeft     uint32 = 0x10000003
	KeyRight    uint32 = 0x10000004
	KeyTab      uint32 = 0x10000005
	KeyEnter    uint32 = 0x10000006
	KeyHome     uint32 = 0x10000007
	KeyEnd      uint32 = 0x10000008
	KeyPageUp   uint32 = 0x10000009
	KeyPageDown uint32 = 0x1000000A
	KeyDelete   uint32 = 0x1000000B
)

const (
	MouseLeft      uint32 = 0x20000001
	MouseWheelUp   uint32 = 0x20000002
	MouseWheelDown uint32 = 0x20000003
	MouseDrag      uint32 = 0x20000004
)

// KeyPasteComplete is returned after a bracketed-paste payload is delivered via
// OnPaste. It is not a user key; the editor maps it to a main-thread redraw.
const KeyPasteComplete uint32 = 0x00FF0002

// UnicodeLimit is one past the highest valid Unicode code point for key payloads.
const UnicodeLimit = 0x110000

// Terminal protocol constants used during key decode.
const (
	finalByteMin           = 0x40
	finalByteMax           = 0x7E
	asciiByteLimit         = 0x80
	codepointTab           = 9
	codepointLinefeed      = 10
	codepointEnter         = 13
	codepointEscape        = 27
	codepointSpace         = 32
	codepointBackspace     = 127
	defaultModifierParam   = 1
	keyEventPress          = 1
	keyEventRelease        = 3
	mouseButtonLeft        = 0
	mouseButtonDrag        = 32
	mouseButtonWheelUp     = 64
	mouseButtonWheelDown   = 65
	kittyKeyLeft           = 57351
	kittyKeyUp             = 57352
	kittyKeyDown           = 57353
	kittyKeyRight          = 57354
	kittyKeyHome           = 57358
	kittyKeyEnd            = 57359
	kittyKeyPageUp         = 57361
	kittyKeyPageDown       = 57362
	kittyKeyDelete         = 57363
	escapeSequenceMaxBytes = 64
)

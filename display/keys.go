package display

import "github.com/jdpalmer/jem/term"

// DecodeKeyChar maps a raw terminal key to an editor-level key code.
// controlContext mirrors the C parameter: when true, normalize letters for
// control-key contexts; when false, normalize Enter/Tab/Escape to specials.
func DecodeKeyChar(key uint32, controlContext bool) uint32 {
	if controlContext && key >= 'a' && key <= 'z' {
		key -= 0x20
	}
	if !controlContext && key == '\t' {
		return term.KeyTab
	}
	if !controlContext && (key == '\r' || key == '\n') {
		return term.KeyEnter
	}
	if !controlContext && key == 0x1B {
		return 0x1B
	}
	if key == 0x00 {
		return term.CTL | ' '
	}
	if key >= 0x01 && key <= 0x1F {
		return term.CTL | (key + '@')
	}
	return key
}

// ApplyMetaPrefixToKey applies the ESC (meta) prefix to a decoded key.
func ApplyMetaPrefixToKey(k uint32) uint32 {
	if k&term.KeyMask != 0 {
		return k | term.META
	}
	if k >= 'a' && k <= 'z' {
		return term.META | (k - ('a' - 'A'))
	}
	if k < 0x100 {
		return term.META | k
	}
	return k | term.META
}

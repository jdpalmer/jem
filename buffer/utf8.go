package buffer

import "unicode/utf8"

// ToLowerASCII lowercases an ASCII letter byte; other bytes are unchanged.
func ToLowerASCII(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b - 'A' + 'a'
	}
	return b
}

// ToUpperASCII uppercases an ASCII letter byte; other bytes are unchanged.
func ToUpperASCII(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b - 'a' + 'A'
	}
	return b
}

// NextOffset returns the byte offset of the next rune start after offset in data.
// If offset is at or past len(data), it returns offset unchanged.
func NextOffset(data []byte, offset int) int {
	if offset < 0 || offset >= len(data) {
		return offset
	}
	_, size := utf8.DecodeRune(data[offset:])
	return offset + size
}

// PrevOffset returns the byte offset of the start of the rune immediately
// preceding offset. If offset is 0 it returns 0.
func PrevOffset(data []byte, offset int) int {
	if offset <= 0 {
		return 0
	}
	r, size := utf8.DecodeLastRune(data[:offset])
	if r == utf8.RuneError && size == 1 {
		return offset - 1
	}
	return offset - size
}

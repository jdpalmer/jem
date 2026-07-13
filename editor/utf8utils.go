package editor

import "unicode/utf8"

// utf8NextOffset returns the byte offset of the next rune start after offset in data.
// If offset is at or past len(data), it returns offset unchanged.
func utf8NextOffset(data []byte, offset uint) uint {
	if offset >= uint(len(data)) {
		return offset
	}
	_, size := utf8.DecodeRune(data[offset:])
	if size <= 0 {
		return offset + 1
	}
	return offset + uint(size)
}

// utf8PrevOffset returns the byte offset of the start of the rune immediately
// preceding "offset". If offset is 0 it returns 0.
func utf8PrevOffset(data []byte, offset uint) uint {
	if offset == 0 {
		return 0
	}
	// DecodeLastRune works on a slice up to offset
	r, size := utf8.DecodeLastRune(data[:offset])
	if r == utf8.RuneError && size == 1 {
		// invalid single byte; step back one
		return offset - 1
	}
	return offset - uint(size)
}

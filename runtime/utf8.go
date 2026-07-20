package runtime

import "unicode/utf8"

func u8lower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b - 'A' + 'a'
	}
	return b
}

func u8upper(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b - 'a' + 'A'
	}
	return b
}

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
	r, size := utf8.DecodeLastRune(data[:offset])
	if r == utf8.RuneError && size == 1 {
		return offset - 1
	}
	return offset - uint(size)
}

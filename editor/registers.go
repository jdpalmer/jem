package editor

// registers.go - Named clipboards / text registers (translation of registers.c)

var registerStore = make(map[string][]byte)

func RegisterSetText(name string, text []byte) bool {
	if name == "" {
		mbWrite("[register name required]")
		return false
	}
	if len(text) == 0 {
		delete(registerStore, name)
		return true
	}
	copyBuf := make([]byte, len(text))
	copy(copyBuf, text)
	registerStore[name] = copyBuf
	return true
}

func RegisterGetText(name string) ([]byte, bool) {
	val, ok := registerStore[name]
	return val, ok
}

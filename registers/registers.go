package registers

// Named text registers (clipboards keyed by name).

var defaultStore = make(map[string][]byte)

// Active is the process-wide register map. Bound by runtime.App.Activate.
var Active map[string][]byte = defaultStore

// Bind points Active at m. Pass nil to restore the package default empty map.
func Bind(m map[string][]byte) {
	if m == nil {
		Active = defaultStore
		return
	}
	Active = m
}

// Set stores text under name. Empty name fails; empty text deletes the entry.
func Set(name string, text []byte) bool {
	if name == "" {
		return false
	}
	if len(text) == 0 {
		delete(Active, name)
		return true
	}
	copyBuf := make([]byte, len(text))
	copy(copyBuf, text)
	Active[name] = copyBuf
	return true
}

// Get returns the text stored under name.
func Get(name string) ([]byte, bool) {
	val, ok := Active[name]
	return val, ok
}

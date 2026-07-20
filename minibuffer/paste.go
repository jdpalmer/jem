package minibuffer

// MinibufferInsertPaste inserts bracketed-paste text into the active minibuffer.
func InsertPaste(text []byte) bool {
	state := Active
	if state == nil || len(text) == 0 {
		return false
	}
	if state.Text == nil || state.Nbuf+uint(len(text)) >= uint(cap(state.Text)) {
		return false
	}

	paste := append([]byte(nil), text...)
	for i := range paste {
		if paste[i] == '\r' {
			paste[i] = '\n'
		}
	}

	insertLen := uint(len(paste))
	if state.Nbuf+insertLen >= uint(cap(state.Text)) {
		insertLen = uint(cap(state.Text)) - state.Nbuf
	}
	copy(state.Text[state.CursorPos:], paste[:insertLen])
	state.Nbuf += insertLen
	state.CursorPos += insertLen
	state.HaveSavedEdit = false
	return true
}

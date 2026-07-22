package minibuffer

// MinibufferInsertPaste inserts bracketed-paste text into the active minibuffer.
func InsertPaste(text []byte) bool {
	state := Active
	if state == nil || len(text) == 0 {
		return false
	}
	if state.Text == nil {
		return false
	}

	paste := append([]byte(nil), text...)
	for i := range paste {
		if paste[i] == '\r' {
			paste[i] = '\n'
		}
	}

	oldLen := len(state.Text)
	insertLen := len(paste)
	if oldLen+insertLen >= state.Nbuf {
		insertLen = state.Nbuf - oldLen - 1
	}
	if insertLen <= 0 {
		return false
	}

	cpos := state.CursorPos
	if cpos > oldLen {
		cpos = oldLen
	}

	state.Text = state.Text[:oldLen+insertLen]
	copy(state.Text[cpos+insertLen:], state.Text[cpos:oldLen])
	copy(state.Text[cpos:], paste[:insertLen])
	state.CursorPos = cpos + insertLen
	state.HaveSavedEdit = false
	return true
}

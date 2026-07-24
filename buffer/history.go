package buffer

// Process-wide undo history for interactive edits.
// Bound at AppInit via BindHistory (same backing store as runtime.History).
var (
	defaultHistory UndoHistory
	History        *UndoHistory = &defaultHistory
)

// BindHistory sets the process-wide undo history. Pass nil to restore the default.
func BindHistory(h *UndoHistory) {
	if h == nil {
		History = &defaultHistory
		return
	}
	History = h
}

// BeginCommand starts an undo group for All.Current at before.
func BeginCommand(before Location) {
	if History == nil || History.IsReplaying || All.Current == nil {
		return
	}
	History.BeginCommand(All.Current, before)
}

// EndCommand finishes the current undo group.
func EndCommand() {
	if History != nil {
		History.EndCommand()
	}
}

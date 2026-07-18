package app

// AppState holds mutable editor application state shared across packages.
type AppState struct {
	EditorRuntimeState
	EditorDisplayState
	EditorMacroState
	EditorSettingsState
}

var defaultState AppState

// State points at the active AppState. Bound by editor.Editor.Activate;
// leaf packages keep using app.State.Field without an Editor import.
var State *AppState = &defaultState

// Bind points State at s. Pass nil to restore the package default storage.
func Bind(s *AppState) {
	if s == nil {
		State = &defaultState
		return
	}
	State = s
}

// Reset clears the currently bound AppState to its zero value.
func Reset() {
	*State = AppState{}
}

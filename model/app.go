// Package model is the fat session model for jem: buffers, windows, marks,
// kill ring, and SetText. An Editor owns an AppState; Activate binds
// model.State so view/mode/search/tools/fileio can use it without importing
// the editor event loop (which would create an import cycle).
package model

// AppState holds mutable editor application state shared across packages.
type AppState struct {
	EditorRuntimeState
	EditorDisplayState
	EditorMacroState
	EditorSettingsState
}

var defaultState AppState

// State points at the active AppState. Bound by editor.Editor.Activate.
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

func (a *AppState) IsRecording() bool { return a.Recording }
func (a *AppState) IsPlaying() bool   { return a.PlayPos >= 0 }

// HasMacro reports whether a non-empty keyboard macro is stored.
func (a *AppState) HasMacro() bool { return len(a.Macro) > 0 }

package app

// AppState holds mutable editor application state shared across packages.
type AppState struct {
	EditorRuntimeState
	EditorDisplayState
	EditorMacroState
	EditorSettingsState
}

// State is the process-wide application state.
var State AppState

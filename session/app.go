package session

// App holds mutable editor application state shared across packages.
type AppState struct {
	EditorRuntimeState
	EditorDisplayState
	EditorMacroState
	EditorSettingsState
}

var App AppState

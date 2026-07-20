package window

// State holds the window list and current window.
type State struct {
	Windows       []*Window
	CurrentWindow *Window
}

var defaultState State

// Active points at the bound window state. Bound by runtime.App.Activate.
var Active *State = &defaultState

// Bind points Active at s. Pass nil to restore the package default storage.
func Bind(s *State) {
	if s == nil {
		Active = &defaultState
		return
	}
	Active = s
}

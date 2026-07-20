package minibuffer

// Active is the currently shown minibuffer edit state, if any.
var Active *State

// State is an alias for MinibufferState used as the active prompt.
type State = MinibufferState

package term

// Hooks connects terminal events to the editor shell.
//
// Set once during editor init via initTermHooks. Not safe for concurrent use.
type Hooks struct {
	OnMouse  func(col, row int)
	OnPaste  func(text []byte)
	OnResume func() // after Resume re-enters editor raw mode (e.g. shell spawn)
}

// PackageHooks is set by the editor during init.
var PackageHooks Hooks

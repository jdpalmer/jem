package search

// KeySession is a multi-key modal driven by the editor listener stack
// (isearch, query-replace confirm). Open returns true when the session
// finishes without waiting for keys.
type KeySession interface {
	Open() (done bool)
	HandleKey(k uint32) (done bool)
	Close()
}

// Hooks are editor-owned callbacks (import cycle avoidance).
type Hooks struct {
	PushKeySession func(s KeySession)
}

// PackageHooks is set once via editor.Services.
var PackageHooks Hooks

func pushKeySession(s KeySession) bool {
	if PackageHooks.PushKeySession == nil {
		return false
	}
	PackageHooks.PushKeySession(s)
	return true
}

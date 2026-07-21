package search

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/minibuffer"
)

// KeySession is a multi-key modal driven by the editor listener stack
// (isearch, query-replace confirm). Open returns true when the session
// finishes without waiting for keys.
type KeySession interface {
	Open() (done bool)
	HandleKey(k uint32) (done bool)
	Close()
}

// Hooks are runtime-owned callbacks (import cycle avoidance).
type Hooks struct {
	PushKeySession func(s KeySession)
	SetText        func(bp *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error
	AskString      func(prompt, initial string, onDone func(string, minibuffer.PromptResult))
	WaitKey        func() (uint32, bool)
	BeginMinibuf   func()
	EndMinibuf     func()
}

// PackageHooks is set once via runtime.Services.
var PackageHooks Hooks

func pushKeySession(s KeySession) bool {
	if PackageHooks.PushKeySession == nil {
		return false
	}
	PackageHooks.PushKeySession(s)
	return true
}

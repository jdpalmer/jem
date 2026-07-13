package editor

import sessionpkg "github.com/jdpalmer/jem/session"

type App = sessionpkg.AppState

var session = struct {
	App *sessionpkg.AppState
}{
	App: &sessionpkg.App,
}

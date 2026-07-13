package tools

import sessionpkg "github.com/jdpalmer/jem/session"

var session = struct {
	App *sessionpkg.AppState
}{
	App: &sessionpkg.App,
}

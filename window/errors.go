package window

import "errors"

var (
	ErrNilWindow  = errors.New("nil window")
	ErrNoEditHook = errors.New("edit hook not configured")
	ErrBadRune    = errors.New("invalid rune")
)

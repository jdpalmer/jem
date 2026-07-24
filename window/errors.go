package window

import "errors"

var (
	ErrNilWindow = errors.New("nil window")
	ErrBadRune   = errors.New("invalid rune")
)

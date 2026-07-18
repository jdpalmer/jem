package buffer

import "errors"

var (
	ErrNilBuffer = errors.New("nil buffer")
	ErrReadonly  = errors.New("read-only buffer")
	ErrBadRange  = errors.New("invalid edit range")
)

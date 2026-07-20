package window

import "github.com/jdpalmer/jem/buffer"

const MaxWindows = 255

// Window is a viewport onto a buffer.
type Window struct {
	Buffer               *buffer.Buffer
	TopLine              uint
	Cursor               buffer.Location
	Mark                 buffer.Location
	ScreenTopRow         uint32
	Height               uint32
	ForceReframe         bool
	ShouldReframe        bool
	DidMove              bool
	DidEdit              bool
	ShouldRedraw         bool
	ShouldUpdateModeLine bool
	HScroll              uint32
}

// Region is a half-open buffer range.
type Region struct {
	Start buffer.Location
	End   buffer.Location
}

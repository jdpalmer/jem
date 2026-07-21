package window

import "github.com/jdpalmer/jem/buffer"

const MaxWindows = 255

// Window is a viewport onto a buffer.
type Window struct {
	Buffer               *buffer.Buffer
	TopLine              int
	Cursor               buffer.Location
	Mark                 buffer.Location
	ScreenTopRow         int
	Height               int
	ForceReframe         bool
	ShouldReframe        bool
	DidMove              bool
	DidEdit              bool
	ShouldRedraw         bool
	ShouldUpdateModeLine bool
	HScroll              int
}

// Region is a half-open buffer range.
type Region struct {
	Start buffer.Location
	End   buffer.Location
}

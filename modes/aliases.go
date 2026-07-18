package modes

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
)

type (
	Buffer   = buffer.Buffer
	Line     = buffer.Line
	Location = buffer.Location
	Window   = app.Window
)

func MakeLocation(line, offset uint) Location { return buffer.MakeLocation(line, offset) }

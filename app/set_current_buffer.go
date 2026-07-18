package app

import (
	"strings"

	"github.com/jdpalmer/jem/buffer"
)

func SetCurrentBuffer(bp *buffer.Buffer) {
	if bp == nil {
		return
	}
	index := -1
	for i, b := range State.Buffers {
		if b == bp {
			index = i
			break
		}
	}
	if index != -1 {
		for i := index; i > 0; i-- {
			State.Buffers[i] = State.Buffers[i-1]
		}
		State.Buffers[0] = bp
	}
	State.CurrentBuffer = bp
}

func BufferFind(name string) *buffer.Buffer {
	for _, bp := range State.Buffers {
		if bp != nil && strings.EqualFold(bp.Name, name) {
			return bp
		}
	}
	return nil
}

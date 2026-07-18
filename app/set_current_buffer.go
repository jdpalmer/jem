package app

import "strings"

func SetCurrentBuffer(bp *Buffer) {
	if bp == nil {
		return
	}
	index := -1
	for i := 0; i < int(State.BufferCount); i++ {
		if State.Buffers[i] == bp {
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
	if PackageHooks.SetCurrentBuffer != nil {
		PackageHooks.SetCurrentBuffer(bp)
	}
}

func BufferFind(name string) *Buffer {
	for i := 0; i < int(State.BufferCount); i++ {
		bp := State.Buffers[i]
		if bp != nil && strings.EqualFold(bp.Name, name) {
			return bp
		}
	}
	return nil
}

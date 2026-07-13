package session

import "strings"

func SetCurrentBuffer(bp *Buffer) {
	if bp == nil {
		return
	}
	index := -1
	for i := 0; i < int(App.BufferCount); i++ {
		if App.Buffers[i] == bp {
			index = i
			break
		}
	}
	if index != -1 {
		for i := index; i > 0; i-- {
			App.Buffers[i] = App.Buffers[i-1]
		}
		App.Buffers[0] = bp
	}
	App.CurrentBuffer = bp
	if PackageHooks.SetCurrentBuffer != nil {
		PackageHooks.SetCurrentBuffer(bp)
	}
}

func BufferFind(name string) *Buffer {
	for i := 0; i < int(App.BufferCount); i++ {
		bp := App.Buffers[i]
		if bp != nil && strings.EqualFold(bp.Name, name) {
			return bp
		}
	}
	return nil
}

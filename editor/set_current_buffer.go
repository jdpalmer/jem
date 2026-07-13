package editor

import sess "github.com/jdpalmer/jem/session"

func SetCurrentBuffer(bp *Buffer) {
	sess.SetCurrentBuffer(bp)
}

func editorSetCurrentBuffer(bp *Buffer) {
	sess.SetCurrentBuffer(bp)
}

func BufferFind(name string) *Buffer {
	return sess.BufferFind(name)
}

func bufferFind(name string) *Buffer {
	return sess.BufferFind(name)
}

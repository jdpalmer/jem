package editor

import sess "github.com/jdpalmer/jem/session"

func truncateBufferName(name string) string {
	return sess.TruncateBufferName(name)
}

func bufferCreate(ed *EditorRuntimeState) *Buffer {
	return sess.BufferCreate(ed)
}

func bufferRelease(bp *Buffer) {
	sess.BufferRelease(bp)
}

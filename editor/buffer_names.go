package editor

import (
	"fmt"
	"path/filepath"
	"strings"

	sess "github.com/jdpalmer/jem/session"
)

func bufferNameFromPath(fname string) string {
	base := filepath.Base(fname)
	if i := strings.IndexByte(base, ';'); i >= 0 {
		base = base[:i]
	}
	if sess.BufferFind(base) == nil {
		return sess.TruncateBufferName(base)
	}
	for suffix := 2; ; suffix++ {
		name := fmt.Sprintf("%s:%d", base, suffix)
		if sess.BufferFind(name) == nil {
			return sess.TruncateBufferName(name)
		}
	}
}

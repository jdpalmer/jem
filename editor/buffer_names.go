package editor

import (
	"fmt"
	"path/filepath"
	"strings"
)

func bufferNameFromPath(fname string) string {
	base := filepath.Base(fname)
	if i := strings.IndexByte(base, ';'); i >= 0 {
		base = base[:i]
	}
	if bufferFind(base) == nil {
		return truncateBufferName(base)
	}
	for suffix := 2; ; suffix++ {
		name := fmt.Sprintf("%s:%d", base, suffix)
		if bufferFind(name) == nil {
			return truncateBufferName(name)
		}
	}
}

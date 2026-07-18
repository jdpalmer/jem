package editor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jdpalmer/jem/app"
)

func bufferNameFromPath(fname string) string {
	base := filepath.Base(fname)
	if i := strings.IndexByte(base, ';'); i >= 0 {
		base = base[:i]
	}
	if app.BufferFind(base) == nil {
		return app.TruncateBufferName(base)
	}
	for suffix := 2; ; suffix++ {
		name := fmt.Sprintf("%s:%d", base, suffix)
		if app.BufferFind(name) == nil {
			return app.TruncateBufferName(name)
		}
	}
}

package display

import (
	"github.com/jdpalmer/jem/buffer"
)

func gitLineDiff(buf *buffer.Buffer, lineNumber int) int {
	if PackageHooks.GitLineDiff == nil {
		return 0
	}
	return PackageHooks.GitLineDiff(buf, lineNumber)
}

func gitModelineText(buf *buffer.Buffer) string {
	if PackageHooks.GitModelineText == nil {
		return ""
	}
	return PackageHooks.GitModelineText(buf)
}

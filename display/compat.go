package display

import (
	"github.com/jdpalmer/jem/buffer"
)

func gitLineDiff(bp *buffer.Buffer, lineNumber uint) int {
	if PackageHooks.GitLineDiff == nil {
		return 0
	}
	return PackageHooks.GitLineDiff(bp, lineNumber)
}

func gitModelineText(bp *buffer.Buffer) string {
	if PackageHooks.GitModelineText == nil {
		return ""
	}
	return PackageHooks.GitModelineText(bp)
}

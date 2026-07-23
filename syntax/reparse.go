package syntax

import (
	"slices"

	"github.com/jdpalmer/jem/buffer"
)

// IncrementalReparse reparses lines starting at startLine until changes stop propagating.
func IncrementalReparse(buf *buffer.Buffer, startLine int) {
	if startLine == 0 || startLine > len(buf.Lines) {
		return
	}
	for ln := startLine; ln <= len(buf.Lines); ln++ {
		line := buf.Line(ln)
		if line == nil {
			continue
		}
		start := buffer.SynState{DFA: SynStateNormal}
		if ln > 1 {
			if prev := buf.Line(ln - 1); prev != nil {
				start = prev.SyntaxEndState
			}
		}
		oldEnd := line.SyntaxEndState
		oldStyles := line.SyntaxStyles
		newEnd, newSummary, newStyles := tokenizeLineFromState(line, start)
		line.SyntaxStyles = newStyles
		line.SyntaxSummary = newSummary
		line.SyntaxEndState = newEnd
		line.SyntaxValid = true
		if oldEnd == newEnd && slices.Equal(oldStyles, newStyles) {
			break
		}
	}
}

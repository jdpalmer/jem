package syntax

import (
	"fmt"
	"os"
	"time"

	"github.com/jdpalmer/jem/buffer"
)

// Toggle for verbose syntax debug logging (set to true during debugging).
var SyntaxDebug = false

// Incremental reparse utilities

// synStateEqual compares two SynState values
func synStateEqual(a, b buffer.SynState) bool {
	return a.DFA == b.DFA && a.Paren == b.Paren && a.Bracket == b.Bracket && a.Curly == b.Curly
}

// stylesEqual is a simple equality for slices of TextStyle
func stylesEqual(a, b []buffer.TextStyle) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

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
		// determine start state from previous line
		var start buffer.SynState
		if ln > 1 {
			prev := buf.Line(ln - 1)
			if prev != nil {
				start = prev.SyntaxEndState
			} else {
				start = buffer.SynState{DFA: SynStateNormal}
			}
		} else {
			start = buffer.SynState{DFA: SynStateNormal}
		}
		oldEnd := line.SyntaxEndState
		oldStyles := line.SyntaxStyles
		newEnd, newSummary, newStyles := tokenizeLineFromState(line, start)
		// update line
		line.SyntaxStyles = newStyles
		line.SyntaxSummary = newSummary
		line.SyntaxEndState = newEnd
		line.SyntaxValid = true
		// if end state matches previous stored end state and styles unchanged, stop
		if synStateEqual(oldEnd, newEnd) && stylesEqual(oldStyles, newStyles) {
			break
		}
		// else continue to next line because change may have propagated
	}
	if SyntaxDebug {
		// Dump a short snapshot to /tmp/jem-syntax.log for the buffer around startLine
		f, err := os.OpenFile("/tmp/jem-syntax.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			fmt.Fprintf(f, "--- IncrementalReparse snapshot %s startLine=%d bufferLines=%d\n", time.Now().Format(time.RFC3339), startLine, len(buf.Lines))
			end := startLine + 9
			if end > len(buf.Lines) {
				end = len(buf.Lines)
			}
			for ln := startLine; ln <= end; ln++ {
				line := buf.Line(ln)
				if line == nil {
					fmt.Fprintf(f, "%d: <nil>\n", ln)
					continue
				}
				fmt.Fprintf(f, "%d: data=%q styles=[", ln, line.Data)
				for i, s := range line.SyntaxStyles {
					if i > 0 {
						fmt.Fprint(f, ",")
					}
					fmt.Fprintf(f, "%04x", uint16(s))
				}
				fmt.Fprintln(f, "]")
			}
		}
	}
}

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
func synStateEqual(a, b SynState) bool {
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

// IncrementalReparse reparses starting at line 'startLine' (1-based) in buffer bp
// and continues until the computed end-state matches the previously stored end
// state for a line (i.e., no further changes propagate).
func IncrementalReparse(bp *buffer.Buffer, startLine uint) {
	if bp == nil || startLine == 0 || startLine > bp.LineCount {
		return
	}
	for ln := startLine; ln <= bp.LineCount; ln++ {
		lp := buffer.GetLine(bp, ln)
		if lp == nil {
			continue
		}
		// determine start state from previous line
		var start SynState
		if ln > 1 {
			prev := buffer.GetLine(bp, ln-1)
			if prev != nil {
				start = prev.SyntaxEndState
			} else {
				start = SynState{DFA: SS_NORMAL}
			}
		} else {
			start = SynState{DFA: SS_NORMAL}
		}
		oldEnd := lp.SyntaxEndState
		oldStyles := lp.SyntaxStyles
		newEnd, newSummary, newStyles := tokenizeLineFromState(lp, start)
		// update line
		lp.SyntaxStyles = newStyles
		lp.SyntaxSummary = newSummary
		lp.SyntaxEndState = newEnd
		lp.SyntaxValid = true
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
			fmt.Fprintf(f, "--- IncrementalReparse snapshot %s startLine=%d bufferLines=%d\n", time.Now().Format(time.RFC3339), startLine, bp.LineCount)
			end := startLine + 9
			if end > bp.LineCount {
				end = bp.LineCount
			}
			for ln := startLine; ln <= end; ln++ {
				lp := buffer.GetLine(bp, ln)
				if lp == nil {
					fmt.Fprintf(f, "%d: <nil>\n", ln)
					continue
				}
				fmt.Fprintf(f, "%d: data=%q styles=[", ln, lp.Data)
				for i, s := range lp.SyntaxStyles {
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

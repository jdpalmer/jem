package syntax

import "github.com/jdpalmer/jem/buffer"

// Test hooks for syntax package tests.
func setOnEnterHook(state int, hook StateHook) {
	if state >= 0 && state < ssStateCount {
		onEnterHooks[state] = hook
	}
}

func clearOnEnterHooks() {
	onEnterHooks = [ssStateCount]StateHook{}
}

func callReenterState(line *buffer.Line, syn *buffer.SynState, i *int, tokenStart int, pendingChar int, styles []buffer.TextStyle, summary *buffer.SyntaxLineSummary) {
	reenterState(line, syn, i, tokenStart, pendingChar, styles, summary)
}

func tokenizeLineFromStateExported(line *buffer.Line, start buffer.SynState) (buffer.SynState, buffer.SyntaxLineSummary, []buffer.TextStyle) {
	return tokenizeLineFromState(line, start)
}

func parenStyleExported(baseColor buffer.TermColor, depth int) buffer.TextStyle {
	return parenStyle(baseColor, depth)
}

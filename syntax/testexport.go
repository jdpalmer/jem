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

func callReenterState(line *buffer.Line, syn *SynState, i *int, tokenStart int, pendingChar int, styles []buffer.TextStyle, summary *buffer.SyntaxLineSummary) {
	reenterState(line, syn, i, tokenStart, pendingChar, styles, summary)
}

func tokenizeLineFromStateExported(lp *buffer.Line, start SynState) (SynState, buffer.SyntaxLineSummary, []buffer.TextStyle) {
	return tokenizeLineFromState(lp, start)
}

func parenStyleExported(baseColor TermColor, depth int) buffer.TextStyle {
	return parenStyle(baseColor, depth)
}

// LangModeSpecForTest exposes langModeSpec for cross-package consistency checks.
func LangModeSpecForTest(mode buffer.LangMode) (ModeSyntaxKind, uint32) {
	spec := langModeSpec(mode)
	return spec.SyntaxKind, spec.SyntaxFlags
}

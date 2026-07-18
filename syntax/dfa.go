package syntax

import "github.com/jdpalmer/jem/buffer"

// DFA-based syntax highlighter ported from src/syntax.c
//
// Architecture mirrors the C implementation exactly:
//   - 21 DFA states (SS_NORMAL … SS_OPERATOR)
//   - Each state: on_enter (every iteration before transition), optional on_exit
//   - on_enter is called BEFORE the transition for every character
//   - STATE_REPROCESS: decrement i so the same char is re-entered next iteration
//   - Delimiter painting: pending_char / reenterState pattern

// ------------------------------------------------------------------
// 1. DFA State Constants
// ------------------------------------------------------------------

const (
	SS_NORMAL       = 0
	SS_IDENT        = 1
	SS_NUMBER       = 2
	SS_STRING_D     = 3
	SS_STRING_D_ESC = 4
	SS_STRING_S     = 5
	SS_STRING_S_ESC = 6
	SS_CMT_LINE     = 7
	SS_CMT_BLOCK    = 8
	SS_CMT_STAR     = 9
	SS_CMT_BRACE    = 10
	SS_CMT_PAREN    = 11
	SS_CMT_PAREN2   = 12
	SS_PREPROC      = 13
	SS_LUA_DASH     = 14
	SS_LUA_BLOCK    = 15
	SS_LUA_BLKEND   = 16
	SS_HTML_CMT     = 17
	SS_HTML_CMT_D1  = 18
	SS_HTML_CMT_D2  = 19
	SS_OPERATOR     = 20
	ssStateCount    = 21
)

// ------------------------------------------------------------------
// 2. Style Helpers (A_* macro equivalents)
// ------------------------------------------------------------------

func aNormal() TextStyle  { return PackagePalette.NormalStyle }
func aComment() TextStyle { return PackagePalette.CommentStyle }
func aString() TextStyle  { return MakeTextStyle(TermColorCyan, TermColorDefault, 0) }
func aNumber() TextStyle  { return MakeTextStyle(TermColorMagenta, TermColorDefault, 0) }
func aPreproc() TextStyle { return MakeTextStyle(TermColorRed, TermColorDefault, 0) }
func aHeading() TextStyle { return MakeTextStyle(TermColorYellow, TermColorDefault, TextStyleBold) }
func aBold() TextStyle {
	return MakeTextStyle(TextStyleFg(PackagePalette.NormalStyle), TermColorDefault, TextStyleBold)
}
func aCodeMD() TextStyle { return MakeTextStyle(TermColorCyan, TermColorDefault, 0) }

// parenStyle returns the rainbow-paren style for a delimiter at depth.
// Cycle: depth%4==0 → baseColor, 1→CYAN, 2→RED, 3→YELLOW (matches C paren_style).
func parenStyle(baseColor TermColor, depth int) TextStyle {
	cycle := [4]TermColor{0, TermColorCyan, TermColorRed, TermColorYellow}
	var fg TermColor
	if (depth & 3) == 0 {
		fg = baseColor
	} else {
		fg = cycle[depth&3]
	}
	return MakeTextStyle(fg, TermColorDefault, TextStyleBold)
}

// ------------------------------------------------------------------
// 3. State Hooks (testable overrides for on_enter / on_exit)
// ------------------------------------------------------------------

// StateHook is a user-provided hook called instead of the built-in on_enter/on_exit.
type StateHook func(line *buffer.Line, syn *SynState, i *int, tokenStart *int, summary *SyntaxLineSummary, styles []buffer.TextStyle, pendingChar int)

var onEnterHooks [ssStateCount]StateHook
var onExitHooks [ssStateCount]StateHook

// reenterActive guards reenterState against infinite recursion.
var reenterActive bool

// reenterState re-invokes on_exit then on_enter for the current DFA state.
// Typically called from a transition when a delimiter needs to be painted via
// the pending_char mechanism.
func reenterState(line *buffer.Line, syn *SynState, i *int, tokenStart int, pendingChar int, styles []buffer.TextStyle, summary *SyntaxLineSummary) {
	cur := int(syn.DFA)
	if cur < 0 || cur >= ssStateCount {
		return
	}
	// Guard: if already inside reenterState, skip hooks and paint directly.
	if reenterActive {
		if pendingChar != 0 {
			paintDelimiter(syn, pendingChar, tokenStart, styles, summary)
		}
		return
	}
	reenterActive = true
	defer func() { reenterActive = false }()
	lm := line.LangMode
	if lm == buffer.LModeNone && line.Buffer != nil {
		lm = line.Buffer.LangMode
	}
	// on_exit
	if onExitHooks[cur] != nil {
		onExitHooks[cur](line, syn, i, &tokenStart, summary, styles, pendingChar)
	} else {
		doBuiltinOnExit(cur, line, syn, i, &tokenStart, summary, styles, pendingChar, lm)
	}
	// on_enter
	if onEnterHooks[cur] != nil {
		onEnterHooks[cur](line, syn, i, &tokenStart, summary, styles, pendingChar)
	} else {
		doBuiltinOnEnter(cur, line, syn, i, &tokenStart, summary, styles, pendingChar, lm)
	}
}

// ------------------------------------------------------------------
// 4. Delimiter / Rainbow Paren Helpers
// ------------------------------------------------------------------

type delimSpec struct {
	open, close int
	color       TermColor
	mask        uint8
}

var kDelims = [3]delimSpec{
	{'(', ')', TermColorMagenta, uint8(buffer.SyntaxDelimParen)},
	{'[', ']', TermColorBlue, uint8(buffer.SyntaxDelimBracket)},
	{'{', '}', TermColorGreen, uint8(buffer.SyntaxDelimCurly)},
}

// delimiterIndex returns the table index (0=paren, 1=bracket, 2=curly) for ch,
// or -1 if ch is not a delimiter.
func delimiterIndex(ch int) int {
	for i := range kDelims {
		d := &kDelims[i]
		if ch == d.open || ch == d.close {
			return i
		}
	}
	return -1
}

// paintDelimiter applies rainbow paren coloring for ch at index idx.
// Openers: paint at current depth then increment. Closers: decrement then paint.
// The high bit (0x80) of each depth byte is metadata and is preserved.
func paintDelimiter(syn *SynState, ch int, idx int, styles []buffer.TextStyle, summary *SyntaxLineSummary) {
	di := delimiterIndex(ch)
	if di < 0 {
		return
	}
	noteSummaryDelimiter(summary, ch, idx)
	var dptr *uint8
	switch di {
	case 0:
		dptr = &syn.Paren
	case 1:
		dptr = &syn.Bracket
	default:
		dptr = &syn.Curly
	}
	old := *dptr
	meta := old & 0x80
	d := kDelims[di]
	if ch == d.open {
		depth := int(old & 0x7F)
		style := parenStyle(d.color, depth)
		putPaint(styles, len(styles), idx, style)
		*dptr = uint8((depth+1)&0x7F) | meta
	} else {
		depth := int(old & 0x7F)
		if depth > 0 {
			depth--
		}
		*dptr = uint8(depth&0x7F) | meta
		style := parenStyle(d.color, depth)
		putPaint(styles, len(styles), idx, style)
	}
}

// ------------------------------------------------------------------
// 5. Low-Level Paint Helpers
// ------------------------------------------------------------------

func putPaint(styles []buffer.TextStyle, n int, i int, val TextStyle) {
	if i >= 0 && i < n {
		styles[i] = val
	}
}

func fillPaint(styles []buffer.TextStyle, n int, a int, b int, val TextStyle) {
	if b > n {
		b = n
	}
	for j := a; j < b; j++ {
		styles[j] = val
	}
}

// ------------------------------------------------------------------
// 6. Summary Helpers
// ------------------------------------------------------------------

func newSummary() SyntaxLineSummary {
	return SyntaxLineSummary{
		FirstCodeOffset: ^uint(0),
		OpenOffsets:     [3]uint{^uint(0), ^uint(0), ^uint(0)},
		CloseOffsets:    [3]uint{^uint(0), ^uint(0), ^uint(0)},
	}
}

func noteSummaryCode(summary *SyntaxLineSummary, offset int) {
	if summary.FirstCodeOffset == ^uint(0) {
		summary.FirstCodeOffset = uint(offset)
	}
}

func noteSummaryDelimiter(summary *SyntaxLineSummary, ch int, offset int) {
	di := delimiterIndex(ch)
	if di < 0 {
		return
	}
	noteSummaryCode(summary, offset)
	d := kDelims[di]
	if ch == d.open {
		summary.OpenMask |= d.mask
		if summary.OpenOffsets[di] == ^uint(0) {
			summary.OpenOffsets[di] = uint(offset)
		}
	} else {
		summary.CloseMask |= d.mask
		if summary.CloseOffsets[di] == ^uint(0) {
			summary.CloseOffsets[di] = uint(offset)
		}
	}
}

// ------------------------------------------------------------------
// 7. Character Classification Helpers
// ------------------------------------------------------------------

// lineGetcR returns the character at rune index i as an int, or 0 if out of range.
func lineGetcR(lp *buffer.Line, i int) int {
	if lp == nil || i < 0 || i >= len(lp.RuneCache) {
		return 0
	}
	return int(lp.RuneCache[i])
}

func isNumberCont(c int) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'a' && c <= 'f') ||
		(c >= 'A' && c <= 'F') ||
		c == '.' || c == '_' ||
		c == 'x' || c == 'X' ||
		c == 'b' || c == 'B' ||
		c == 'l' || c == 'L' ||
		c == 'u' || c == 'U' ||
		c == 'f' || c == 'F' ||
		c == 'e' || c == 'E' ||
		c == '+' || c == '-'
}

func isIdentStart(c int, flags uint32) bool {
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
		return true
	}
	if flags&ModeFlagIdentLispSigil != 0 && (c == '#' || c == '&') {
		return true
	}
	return false
}

func isOperatorChar(c int) bool {
	switch c {
	case '+', '-', '*', '/', '%', '=', '<', '>', '!', '&', '|', '^', '~', '?', ':', '.', '#', '$', '@':
		return true
	}
	return false
}

func isIdentCont(c int, flags uint32) bool {
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' {
		return true
	}
	if flags&ModeFlagIdentDash != 0 && c == '-' {
		return true
	}
	if flags&ModeFlagIdentLispExtra != 0 {
		switch c {
		case '-', '?', '!', '/', '<', '>', '=', '+', '*', '.', '#':
			return true
		}
	}
	// Non-ASCII runes are part of identifiers
	if c > 127 {
		return true
	}
	return false
}

// ------------------------------------------------------------------
// 8. Built-in On-Enter / On-Exit Logic
// ------------------------------------------------------------------

// doBuiltinOnEnter is the default (non-override) on_enter handler.
func doBuiltinOnEnter(state int, lp *buffer.Line, syn *SynState, i *int, tokenStart *int, summary *SyntaxLineSummary, styles []buffer.TextStyle, pendingChar int, lm LangMode) {
	n := len(styles)
	idx := *i
	switch state {
	case SS_NORMAL:
		if pendingChar != 0 {
			paintDelimiter(syn, pendingChar, *tokenStart, styles, summary)
			return
		}
		c := lineGetcR(lp, idx)
		if c != ' ' && c != '\t' {
			noteSummaryCode(summary, idx)
		}
		putPaint(styles, n, idx, aNormal())

	case SS_NUMBER:
		putPaint(styles, n, idx, aNumber())

	case SS_STRING_D, SS_STRING_D_ESC, SS_STRING_S, SS_STRING_S_ESC:
		putPaint(styles, n, idx, aString())

	case SS_CMT_LINE, SS_CMT_BLOCK, SS_CMT_STAR,
		SS_CMT_BRACE, SS_CMT_PAREN, SS_CMT_PAREN2:
		putPaint(styles, n, idx, aComment())

	case SS_LUA_DASH:
		c := lineGetcR(lp, idx)
		if c == '-' {
			putPaint(styles, n, *tokenStart, aComment())
			putPaint(styles, n, idx, aComment())
		} else {
			putPaint(styles, n, *tokenStart, aNormal())
		}

	case SS_LUA_BLOCK, SS_LUA_BLKEND:
		putPaint(styles, n, idx, aComment())

	case SS_HTML_CMT, SS_HTML_CMT_D1, SS_HTML_CMT_D2:
		putPaint(styles, n, idx, aComment())

		// SS_IDENT: no on_enter (chars stay normal until on_exit classifies the token)
		// SS_PREPROC: handled whole-line before the DFA loop
	}
}

// skipEnterOnStringClose reports whether entering newState should be skipped because
// cur's on_enter already painted a closing string quote at the current index.
func skipEnterOnStringClose(cur, newState, c int) bool {
	if newState != SS_NORMAL {
		return false
	}
	switch cur {
	case SS_STRING_D:
		return c == '"'
	case SS_STRING_S:
		return c == '\''
	default:
		return false
	}
}

// paintOperatorRange highlights the longest known operator prefix in [start, end).
func paintOperatorRange(lp *buffer.Line, start, end int, lm LangMode, styles []buffer.TextStyle) {
	if start >= end || end > len(styles) {
		return
	}
	ops, ok := operatorsByLang[lm]
	if !ok || len(ops) == 0 {
		return
	}
	text := string(lp.RuneCache[start:end])
	matchLen := 0
	for n := len(text); n > 0; n-- {
		if ops[text[:n]] {
			matchLen = n
			break
		}
	}
	if matchLen == 0 {
		return
	}
	style := operatorStyleForLang(lm, text[:matchLen])
	if style == TextStyleDefault {
		return
	}
	fillPaint(styles, len(styles), start, start+matchLen, style)
}

// paintIdentRange applies keyword/type coloring to an identifier token span.
func paintIdentRange(lp *buffer.Line, start, end int, lm LangMode, styles []buffer.TextStyle) {
	if start >= end || end > len(styles) {
		return
	}
	text := string(lp.RuneCache[start:end])
	a := ident_color_for_lang(lm, text)
	if a == TextStyleDefault {
		a = aNormal()
	}
	fillPaint(styles, len(styles), start, end, a)
}

// doBuiltinOnExit is the default (non-override) on_exit handler.
func doBuiltinOnExit(state int, lp *buffer.Line, syn *SynState, i *int, tokenStart *int, summary *SyntaxLineSummary, styles []buffer.TextStyle, pendingChar int, lm LangMode) {
	switch state {
	case SS_IDENT:
		paintIdentRange(lp, *tokenStart, *i, lm, styles)
	case SS_OPERATOR:
		paintOperatorRange(lp, *tokenStart, *i, lm, styles)
	}
}

// ------------------------------------------------------------------
// 9. Markdown / HTML Specialised Highlighters
// ------------------------------------------------------------------

// highlightMarkdown is a direct port of C's highlight_markdown().
func highlightMarkdown(lp *buffer.Line, styles []buffer.TextStyle, n int) {
	if n == 0 || len(lp.RuneCache) == 0 {
		return
	}
	i := 0
	// Skip leading whitespace
	for i < n && (lineGetcR(lp, i) == ' ' || lineGetcR(lp, i) == '\t') {
		i++
	}
	if i >= n {
		return
	}
	// Heading: line starting with '#'
	if lineGetcR(lp, i) == '#' {
		fillPaint(styles, n, 0, n, aHeading())
		return
	}
	// Optional line-number prefix "L<digits>:"
	if lineGetcR(lp, i) == 'L' && i+1 < n {
		checkpoint := i
		i++ // skip 'L'
		start := i
		for i < n && lineGetcR(lp, i) >= '0' && lineGetcR(lp, i) <= '9' {
			i++
		}
		if i > start && i < n && lineGetcR(lp, i) == ':' {
			i++ // include ':'
			fillPaint(styles, n, checkpoint, i, aNumber())
		} else {
			i = checkpoint
		}
	}
	// Inline spans: **bold** and `code`
	inBold, boldStart := false, 0
	inCode, codeStart := false, 0
	for i < n {
		c := lineGetcR(lp, i)
		nc := lineGetcR(lp, i+1)
		if !inCode && c == '*' && nc == '*' {
			if !inBold {
				inBold = true
				boldStart = i
			} else {
				fillPaint(styles, n, boldStart, i+2, aBold())
				inBold = false
			}
			i += 2
			continue
		}
		if c == '`' {
			if !inCode {
				inCode = true
				codeStart = i
			} else {
				fillPaint(styles, n, codeStart, i+1, aCodeMD())
				inCode = false
			}
			i++
			continue
		}
		if inCode {
			putPaint(styles, n, i, aCodeMD())
		} else if inBold {
			putPaint(styles, n, i, aBold())
		}
		i++
	}
}

// highlightHTML is a port of C's highlight_html().
func highlightHTML(lp *buffer.Line, styles []buffer.TextStyle, n int, syn *SynState) {
	for i := 0; i < n; i++ {
		c := lineGetcR(lp, i)
		dfa := int(syn.DFA)
		if dfa == SS_HTML_CMT || dfa == SS_HTML_CMT_D1 || dfa == SS_HTML_CMT_D2 {
			putPaint(styles, n, i, aComment())
			if c == '-' {
				if dfa == SS_HTML_CMT_D1 {
					syn.DFA = SS_HTML_CMT_D2
				} else {
					syn.DFA = SS_HTML_CMT_D1
				}
			} else if c == '>' && dfa == SS_HTML_CMT_D2 {
				syn.DFA = SS_NORMAL
			} else {
				syn.DFA = SS_HTML_CMT
			}
		} else if c == '<' &&
			i+3 < n &&
			lineGetcR(lp, i+1) == '!' &&
			lineGetcR(lp, i+2) == '-' &&
			lineGetcR(lp, i+3) == '-' {
			fillPaint(styles, n, i, i+4, aComment())
			syn.DFA = SS_HTML_CMT
			i += 3
		} else {
			putPaint(styles, n, i, aNormal())
		}
	}
}

// ------------------------------------------------------------------
// 10. Main Tokenizer
// ------------------------------------------------------------------

// tokenizeLineFromState runs the DFA syntax highlighter on lp from start state.
// Returns the end state, line summary, and per-rune style slice.
func tokenizeLineFromState(lp *buffer.Line, start SynState) (SynState, SyntaxLineSummary, []buffer.TextStyle) {
	return tokenizeLineFromStateLimit(lp, start, -1)
}

// tokenizeLineFromStateLimit scans up to scanLimit runes (-1 = full line).
func tokenizeLineFromStateLimit(lp *buffer.Line, start SynState, scanLimit int) (SynState, SyntaxLineSummary, []buffer.TextStyle) {
	lp.EnsureCache()

	lm := lp.LangMode
	if lm == buffer.LModeNone && lp.Buffer != nil {
		lm = lp.Buffer.LangMode
	}
	info := langModeSpec(lm)

	syn := start
	n := len(lp.RuneCache)
	loopEnd := n
	fullScan := scanLimit < 0 || scanLimit >= n
	if !fullScan {
		loopEnd = scanLimit
	}
	styles := make([]buffer.TextStyle, n)
	for j := range styles {
		styles[j] = aNormal()
	}
	summary := newSummary()

	getc := func(i int) int {
		if i >= 0 && i < n {
			return int(lp.RuneCache[i])
		}
		return 0
	}

	// ---- Special syntax kinds (handled before DFA loop) ----

	switch info.SyntaxKind {
	case ModeSyntaxNone:
		syn = SynState{}
		if fullScan {
			lp.SyntaxEndState = syn
			lp.SyntaxSummary = summary
			lp.SyntaxValid = true
		}
		return syn, summary, styles

	case ModeSyntaxHashCommentOnly:
		syn = SynState{}
		for i := 0; i < n; i++ {
			if getc(i) == '#' {
				fillPaint(styles, n, i, n, aComment())
				syn.DFA = SS_CMT_LINE
				break
			}
		}
		if fullScan {
			lp.SyntaxEndState = syn
			lp.SyntaxSummary = summary
			lp.SyntaxValid = true
		}
		return syn, summary, styles

	case ModeSyntaxMarkdown:
		highlightMarkdown(lp, styles, n)
		if fullScan {
			lp.SyntaxEndState = syn
			lp.SyntaxSummary = summary
			lp.SyntaxValid = true
		}
		return syn, summary, styles

	case ModeSyntaxHTML:
		highlightHTML(lp, styles, n, &syn)
		syn.Paren = 0
		syn.Bracket = 0
		syn.Curly = 0
		if fullScan {
			lp.SyntaxEndState = syn
			lp.SyntaxSummary = summary
			lp.SyntaxValid = true
		}
		return syn, summary, styles
	}

	// ---- Preproc continuation ----

	if syn.DFA == SS_PREPROC {
		fillPaint(styles, n, 0, n, aPreproc())
		for i := 0; i < n; i++ {
			if getc(i) != ' ' && getc(i) != '\t' {
				noteSummaryCode(&summary, i)
				break
			}
		}
		if n == 0 || getc(n-1) != '\\' {
			syn.DFA = SS_NORMAL
		}
		if fullScan {
			lp.SyntaxEndState = syn
			lp.SyntaxSummary = summary
			lp.SyntaxValid = true
		}
		return syn, summary, styles
	}

	// ---- DFA loop ----

	flags := info.SyntaxFlags
	tokenStart := 0
	pendingChar := 0

	// callEnter dispatches on_enter for state, honoring hook overrides.
	callEnter := func(state int, i *int) {
		if state >= 0 && state < ssStateCount && onEnterHooks[state] != nil {
			onEnterHooks[state](lp, &syn, i, &tokenStart, &summary, styles, pendingChar)
		} else {
			doBuiltinOnEnter(state, lp, &syn, i, &tokenStart, &summary, styles, pendingChar, lm)
		}
	}

	// callExit dispatches on_exit for state, honoring hook overrides.
	callExit := func(state int, i *int) {
		if state >= 0 && state < ssStateCount && onExitHooks[state] != nil {
			onExitHooks[state](lp, &syn, i, &tokenStart, &summary, styles, pendingChar)
		} else {
			doBuiltinOnExit(state, lp, &syn, i, &tokenStart, &summary, styles, pendingChar, lm)
		}
	}

	for i := 0; i < loopEnd; {
		cur := int(syn.DFA)
		if cur < 0 || cur >= ssStateCount {
			syn.DFA = SS_NORMAL
			cur = SS_NORMAL
		}
		lookahead := getc(i + 1)

		// on_enter for current state (called every iteration before transition)
		callEnter(cur, &i)

		// Transition: updates syn.DFA; returns reprocess flag
		reprocess := false
		c := getc(i)

		switch cur {
		case SS_NORMAL:
			// // line comment
			if flags&ModeFlagCommentSlashLine != 0 && c == '/' && lookahead == '/' {
				fillPaint(styles, n, i, n, aComment())
				syn.DFA = SS_CMT_LINE
				i = loopEnd // skip to end; loop's i++ will exit
				goto afterTransition
			}
			// /* block comment
			if flags&ModeFlagCommentSlashBlock != 0 && c == '/' && lookahead == '*' {
				putPaint(styles, n, i, aComment())
				putPaint(styles, n, i+1, aComment())
				syn.DFA = SS_CMT_BLOCK
				i++
				goto afterTransition
			}
			// # preprocessor directive at BOL
			if flags&ModeFlagPreprocHashAtBOL != 0 && c == '#' {
				anyBefore := false
				for k := 0; k < i; k++ {
					if getc(k) != ' ' && getc(k) != '\t' {
						anyBefore = true
						break
					}
				}
				if !anyBefore {
					noteSummaryCode(&summary, i)
					fillPaint(styles, n, 0, n, aPreproc())
					last := getc(n - 1)
					if last == '\\' {
						syn.DFA = SS_PREPROC
					} else {
						syn.DFA = SS_NORMAL
					}
					i = loopEnd
					goto afterTransition
				}
			}
			// # line comment (hash comment mode)
			if flags&ModeFlagCommentHash != 0 && c == '#' {
				fillPaint(styles, n, i, n, aComment())
				syn.DFA = SS_CMT_LINE
				i = loopEnd
				goto afterTransition
			}
			// ; line comment (lisp mode)
			if flags&ModeFlagCommentSemi != 0 && c == ';' {
				fillPaint(styles, n, i, n, aComment())
				syn.DFA = SS_CMT_LINE
				i = loopEnd
				goto afterTransition
			}
			// -- lua comment / lua block comment
			if flags&ModeFlagCommentLua != 0 && c == '-' {
				tokenStart = i
				syn.DFA = SS_LUA_DASH
				noteSummaryCode(&summary, i)
				putPaint(styles, n, i, aNormal())
				goto afterTransition
			}
			// { pascal brace comment
			if flags&ModeFlagCommentPascalBrace != 0 && c == '{' {
				putPaint(styles, n, i, aComment())
				syn.DFA = SS_CMT_BRACE
				goto afterTransition
			}
			// (* pascal paren comment
			if flags&ModeFlagCommentPascalParen != 0 && c == '(' && lookahead == '*' {
				putPaint(styles, n, i, aComment())
				putPaint(styles, n, i+1, aComment())
				syn.DFA = SS_CMT_PAREN
				i++
				goto afterTransition
			}
			// " double-quoted string
			if c == '"' {
				noteSummaryCode(&summary, i)
				syn.DFA = SS_STRING_D
				goto afterTransition
			}
			// ' single-quoted string
			if c == '\'' {
				noteSummaryCode(&summary, i)
				syn.DFA = SS_STRING_S
				goto afterTransition
			}
			// number: starts with digit or '.' followed by digit
			if (c >= '0' && c <= '9') || (c == '.' && lookahead >= '0' && lookahead <= '9') {
				noteSummaryCode(&summary, i)
				tokenStart = i
				syn.DFA = SS_NUMBER
				goto afterTransition
			}
			// identifier
			if isIdentStart(c, flags) {
				noteSummaryCode(&summary, i)
				tokenStart = i
				syn.DFA = SS_IDENT
				goto afterTransition
			}
			// @ at-rule (CSS)
			if flags&ModeFlagAtRule != 0 && c == '@' && lookahead >= 'a' && lookahead <= 'z' {
				noteSummaryCode(&summary, i)
				putPaint(styles, n, i, aPreproc())
				tokenStart = i + 1
				syn.DFA = SS_IDENT
				goto afterTransition
			}
			// delimiter (rainbow paren)
			if c == '(' || c == ')' || c == '[' || c == ']' ||
				(c == '{' && flags&ModeFlagNoCurlyRainbow == 0) ||
				(c == '}' && flags&ModeFlagNoCurlyRainbow == 0) {
				if delimiterIndex(c) >= 0 {
					pendingChar = c
					tokenStart = i
					reenterState(lp, &syn, &i, tokenStart, pendingChar, styles, &summary)
					pendingChar = 0
					goto afterTransition
				}
			}
			// operator / assignment
			if isOperatorChar(c) {
				noteSummaryCode(&summary, i)
				tokenStart = i
				syn.DFA = SS_OPERATOR
				goto afterTransition
			}
			// other non-whitespace is code
			if c != ' ' && c != '\t' {
				noteSummaryCode(&summary, i)
			}

		case SS_IDENT:
			if !isIdentCont(c, flags) {
				syn.DFA = SS_NORMAL
				reprocess = true
			}

		case SS_OPERATOR:
			if !isOperatorChar(c) {
				syn.DFA = SS_NORMAL
				reprocess = true
			}

		case SS_NUMBER:
			if !isNumberCont(c) {
				syn.DFA = SS_NORMAL
				reprocess = true
			}

		case SS_STRING_D:
			if c == '"' {
				syn.DFA = SS_NORMAL
			} else if c == '\\' {
				syn.DFA = SS_STRING_D_ESC
			}

		case SS_STRING_D_ESC:
			syn.DFA = SS_STRING_D

		case SS_STRING_S:
			if c == '\'' {
				syn.DFA = SS_NORMAL
			} else if c == '\\' {
				syn.DFA = SS_STRING_S_ESC
			}

		case SS_STRING_S_ESC:
			syn.DFA = SS_STRING_S

		case SS_CMT_BLOCK:
			if c == '*' {
				syn.DFA = SS_CMT_STAR
			}

		case SS_CMT_STAR:
			if c == '/' {
				syn.DFA = SS_NORMAL
			} else if c != '*' {
				syn.DFA = SS_CMT_BLOCK
			}

		case SS_CMT_BRACE:
			if c == '}' {
				syn.DFA = SS_NORMAL
			}

		case SS_CMT_PAREN:
			if c == '*' {
				syn.DFA = SS_CMT_PAREN2
			}

		case SS_CMT_PAREN2:
			if c == ')' {
				syn.DFA = SS_NORMAL
			} else if c != '*' {
				syn.DFA = SS_CMT_PAREN
			}

		case SS_CMT_LINE:
			// stay in line-comment state until EOL

		case SS_LUA_DASH:
			if c == '-' {
				nnc := getc(i + 2)
				if lookahead == '[' && nnc == '[' {
					syn.DFA = SS_LUA_BLOCK
				} else {
					syn.DFA = SS_CMT_LINE
				}
			} else {
				syn.DFA = SS_NORMAL
				reprocess = true
			}

		case SS_LUA_BLOCK:
			if c == ']' {
				syn.DFA = SS_LUA_BLKEND
			}

		case SS_LUA_BLKEND:
			if c == ']' {
				syn.DFA = SS_NORMAL
			} else {
				syn.DFA = SS_LUA_BLOCK
			}

		case SS_HTML_CMT:
			if c == '-' {
				syn.DFA = SS_HTML_CMT_D1
			}
			// else stay in SS_HTML_CMT

		case SS_HTML_CMT_D1:
			if c == '-' {
				syn.DFA = SS_HTML_CMT_D2
			} else {
				syn.DFA = SS_HTML_CMT
			}

		case SS_HTML_CMT_D2:
			if c == '>' {
				syn.DFA = SS_NORMAL
			} else if c != '-' {
				syn.DFA = SS_HTML_CMT
			}

		default:
			syn.DFA = SS_NORMAL
		}

	afterTransition:
		newState := int(syn.DFA)
		if newState < 0 || newState >= ssStateCount {
			syn.DFA = SS_NORMAL
			newState = SS_NORMAL
		}

		if newState != cur {
			// Left old state: call on_exit
			callExit(cur, &i)
			// Enter new state (unless reprocessing — new state's on_enter fires next iteration).
			// Skip when closing a string: on_enter for the string state already painted the quote.
			if !reprocess && !skipEnterOnStringClose(cur, newState, c) {
				callEnter(newState, &i)
			}
		}

		if reprocess {
			// Do NOT increment i — same char will be reprocessed in next iteration
			// (new state's on_enter fires at top of next loop iteration)
		} else {
			i++
		}
	}

	if !fullScan {
		return syn, summary, styles
	}

	// ---- End-of-line cleanup ----

	// Finalize identifier token if we hit EOL inside one
	if syn.DFA == SS_IDENT {
		paintIdentRange(lp, tokenStart, n, lm, styles)
		syn.DFA = SS_NORMAL
	}
	if syn.DFA == SS_OPERATOR {
		paintOperatorRange(lp, tokenStart, n, lm, styles)
		syn.DFA = SS_NORMAL
	}

	// States that do NOT persist across lines (reset to NORMAL at EOL)
	switch syn.DFA {
	case SS_CMT_LINE, SS_NUMBER, SS_STRING_D, SS_STRING_D_ESC,
		SS_STRING_S, SS_STRING_S_ESC, SS_LUA_DASH:
		syn.DFA = SS_NORMAL
	}

	// Persist block-comment / multi-line states across lines:
	// SS_CMT_BLOCK, SS_CMT_STAR, SS_CMT_BRACE, SS_CMT_PAREN, SS_CMT_PAREN2,
	// SS_LUA_BLOCK, SS_LUA_BLKEND, SS_HTML_CMT, SS_HTML_CMT_D1, SS_HTML_CMT_D2,
	// SS_PREPROC are all left as-is.

	lp.SyntaxEndState = syn
	lp.SyntaxSummary = summary
	lp.SyntaxValid = true
	return syn, summary, styles
}

// ------------------------------------------------------------------
// 11. SyntaxEnsureLine
// ------------------------------------------------------------------

// SyntaxEnsureLine ensures lp has up-to-date syntax styles.
// It walks back through the buffer to find a valid start state when possible.
func SyntaxEnsureLine(lp *buffer.Line) {
	if lp == nil {
		return
	}
	if lp.SyntaxValid && lp.SyntaxStyles != nil {
		return
	}

	// Find start state: use previous line's end state if available.
	start := SynState{DFA: SS_NORMAL}
	if lp.Buffer != nil {
		bp := lp.Buffer
		// Find this line's index in the buffer.
		lineNum := lineNumberInBuffer(bp, lp)
		if lineNum > 1 {
			prev := bp.Line(lineNum-1)
			if prev != nil {
				if !prev.SyntaxValid {
					SyntaxEnsureLine(prev) // ensure prev is computed first
				}
				start = prev.SyntaxEndState
			}
		}
	}

	_, summary, styles := tokenizeLineFromState(lp, start)
	lp.SyntaxStyles = styles
	lp.SyntaxSummary = summary
}

// lineNumberInBuffer returns the 1-based line number of lp within bp, or 0.
func lineNumberInBuffer(bp *buffer.Buffer, lp *buffer.Line) uint {
	if bp == nil || lp == nil {
		return 0
	}
	for i := range bp.Lines {
		if &bp.Lines[i] == lp {
			return uint(i + 1)
		}
	}
	return 0
}

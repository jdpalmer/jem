package syntax

import (
	"github.com/jdpalmer/jem/buffer"
)

// DFA-based syntax highlighter (ported from the C editor).
//
// Architecture:
//   - 21 DFA states (SynStateNormal … SynStateOperator)
//   - Each state: on_enter (every iteration before transition), optional on_exit
//   - on_enter is called BEFORE the transition for every character
//   - STATE_REPROCESS: decrement i so the same char is re-entered next iteration
//   - Delimiter painting: pending_char / reenterState pattern

// ------------------------------------------------------------------
// 1. DFA State Constants
// ------------------------------------------------------------------

const (
	SynStateNormal     = 0
	SynStateIdent      = 1
	SynStateNumber     = 2
	SynStateStringD    = 3
	SynStateStringDEsc = 4
	SynStateStringS    = 5
	SynStateStringSEsc = 6
	SynStateCmtLine    = 7
	SynStateCmtBlock   = 8
	SynStateCmtStar    = 9
	SynStateCmtBrace   = 10
	SynStateCmtParen   = 11
	SynStateCmtParen2  = 12
	SynStatePreproc    = 13
	SynStateLuaDash    = 14
	SynStateLuaBlock   = 15
	SynStateLuaBlkEnd  = 16
	SynStateHTMLCmt    = 17
	SynStateHTMLCmtD1  = 18
	SynStateHTMLCmtD2  = 19
	SynStateOperator   = 20
	ssStateCount       = 21
)

// ------------------------------------------------------------------
// 2. Style Helpers (A_* macro equivalents)
// ------------------------------------------------------------------

func aNormal() buffer.TextStyle  { return PackagePalette.NormalStyle }
func aComment() buffer.TextStyle { return PackagePalette.CommentStyle }
func aString() buffer.TextStyle {
	return buffer.MakeTextStyle(buffer.TermColorCyan, buffer.TermColorDefault, 0)
}
func aNumber() buffer.TextStyle {
	return buffer.MakeTextStyle(buffer.TermColorMagenta, buffer.TermColorDefault, 0)
}
func aPreproc() buffer.TextStyle {
	return buffer.MakeTextStyle(buffer.TermColorRed, buffer.TermColorDefault, 0)
}
func aHeading() buffer.TextStyle {
	return buffer.MakeTextStyle(buffer.TermColorYellow, buffer.TermColorDefault, buffer.TextStyleBold)
}
func aBold() buffer.TextStyle {
	return buffer.MakeTextStyle(PackagePalette.NormalStyle.Fg(), buffer.TermColorDefault, buffer.TextStyleBold)
}
func aCodeMD() buffer.TextStyle {
	return buffer.MakeTextStyle(buffer.TermColorCyan, buffer.TermColorDefault, 0)
}

// parenStyle returns the rainbow-paren style for a delimiter at depth.
// Cycle: depth%4==0 → baseColor, 1→CYAN, 2→RED, 3→YELLOW (matches C paren_style).
func parenStyle(baseColor buffer.TermColor, depth int) buffer.TextStyle {
	cycle := [4]buffer.TermColor{0, buffer.TermColorCyan, buffer.TermColorRed, buffer.TermColorYellow}
	var fg buffer.TermColor
	if (depth & 3) == 0 {
		fg = baseColor
	} else {
		fg = cycle[depth&3]
	}
	return buffer.MakeTextStyle(fg, buffer.TermColorDefault, buffer.TextStyleBold)
}

// ------------------------------------------------------------------
// 3. State Hooks (testable overrides for on_enter / on_exit)
// ------------------------------------------------------------------

// StateHook is a user-provided hook called instead of the built-in on_enter/on_exit.
type StateHook func(line *buffer.Line, syn *buffer.SynState, i *int, tokenStart *int, summary *buffer.SyntaxLineSummary, styles []buffer.TextStyle, pendingChar int)

var onEnterHooks [ssStateCount]StateHook
var onExitHooks [ssStateCount]StateHook

// reenterActive guards reenterState against infinite recursion.
var reenterActive bool

// reenterState re-invokes on_exit then on_enter for the current DFA state.
// Typically called from a transition when a delimiter needs to be painted via
// the pending_char mechanism.
func reenterState(line *buffer.Line, syn *buffer.SynState, i *int, tokenStart int, pendingChar int, styles []buffer.TextStyle, summary *buffer.SyntaxLineSummary) {
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
	color       buffer.TermColor
	mask        uint8
}

var kDelims = [3]delimSpec{
	{'(', ')', buffer.TermColorMagenta, uint8(buffer.SyntaxDelimParen)},
	{'[', ']', buffer.TermColorBlue, uint8(buffer.SyntaxDelimBracket)},
	{'{', '}', buffer.TermColorGreen, uint8(buffer.SyntaxDelimCurly)},
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
func paintDelimiter(syn *buffer.SynState, ch int, idx int, styles []buffer.TextStyle, summary *buffer.SyntaxLineSummary) {
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

func putPaint(styles []buffer.TextStyle, n int, i int, val buffer.TextStyle) {
	if i >= 0 && i < n {
		styles[i] = val
	}
}

func fillPaint(styles []buffer.TextStyle, n int, a int, b int, val buffer.TextStyle) {
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

func newSummary() buffer.SyntaxLineSummary {
	return buffer.SyntaxLineSummary{
		FirstCodeOffset: ^uint(0),
		OpenOffsets:     [3]uint{^uint(0), ^uint(0), ^uint(0)},
		CloseOffsets:    [3]uint{^uint(0), ^uint(0), ^uint(0)},
	}
}

func noteSummaryCode(summary *buffer.SyntaxLineSummary, offset int) {
	if summary.FirstCodeOffset == ^uint(0) {
		summary.FirstCodeOffset = uint(offset)
	}
}

func noteSummaryDelimiter(summary *buffer.SyntaxLineSummary, ch int, offset int) {
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
func lineGetcR(line *buffer.Line, i int) int {
	if line == nil || i < 0 || i >= len(line.RuneCache) {
		return 0
	}
	return int(line.RuneCache[i])
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
func doBuiltinOnEnter(state int, line *buffer.Line, syn *buffer.SynState, i *int, tokenStart *int, summary *buffer.SyntaxLineSummary, styles []buffer.TextStyle, pendingChar int, lm buffer.LangMode) {
	n := len(styles)
	idx := *i
	switch state {
	case SynStateNormal:
		if pendingChar != 0 {
			paintDelimiter(syn, pendingChar, *tokenStart, styles, summary)
			return
		}
		c := lineGetcR(line, idx)
		if c != ' ' && c != '\t' {
			noteSummaryCode(summary, idx)
		}
		putPaint(styles, n, idx, aNormal())

	case SynStateNumber:
		putPaint(styles, n, idx, aNumber())

	case SynStateStringD, SynStateStringDEsc, SynStateStringS, SynStateStringSEsc:
		putPaint(styles, n, idx, aString())

	case SynStateCmtLine, SynStateCmtBlock, SynStateCmtStar,
		SynStateCmtBrace, SynStateCmtParen, SynStateCmtParen2:
		putPaint(styles, n, idx, aComment())

	case SynStateLuaDash:
		c := lineGetcR(line, idx)
		if c == '-' {
			putPaint(styles, n, *tokenStart, aComment())
			putPaint(styles, n, idx, aComment())
		} else {
			putPaint(styles, n, *tokenStart, aNormal())
		}

	case SynStateLuaBlock, SynStateLuaBlkEnd:
		putPaint(styles, n, idx, aComment())

	case SynStateHTMLCmt, SynStateHTMLCmtD1, SynStateHTMLCmtD2:
		putPaint(styles, n, idx, aComment())

		// SynStateIdent: no on_enter (chars stay normal until on_exit classifies the token)
		// SynStatePreproc: handled whole-line before the DFA loop
	}
}

// skipEnterOnStringClose reports whether entering newState should be skipped because
// cur's on_enter already painted a closing string quote at the current index.
func skipEnterOnStringClose(cur, newState, c int) bool {
	if newState != SynStateNormal {
		return false
	}
	switch cur {
	case SynStateStringD:
		return c == '"'
	case SynStateStringS:
		return c == '\''
	default:
		return false
	}
}

// paintOperatorRange highlights the longest known operator prefix in [start, end).
func paintOperatorRange(line *buffer.Line, start, end int, lm buffer.LangMode, styles []buffer.TextStyle) {
	if start >= end || end > len(styles) {
		return
	}
	ops, ok := operatorsByLang[lm]
	if !ok || len(ops) == 0 {
		return
	}
	text := string(line.RuneCache[start:end])
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
	if style == buffer.TextStyleDefault {
		return
	}
	fillPaint(styles, len(styles), start, start+matchLen, style)
}

// paintIdentRange applies keyword/type coloring to an identifier token span.
func paintIdentRange(line *buffer.Line, start, end int, lm buffer.LangMode, styles []buffer.TextStyle) {
	if start >= end || end > len(styles) {
		return
	}
	text := string(line.RuneCache[start:end])
	a := identColorForLang(lm, text)
	if a == buffer.TextStyleDefault {
		a = aNormal()
	}
	fillPaint(styles, len(styles), start, end, a)
}

// doBuiltinOnExit is the default (non-override) on_exit handler.
func doBuiltinOnExit(state int, line *buffer.Line, syn *buffer.SynState, i *int, tokenStart *int, summary *buffer.SyntaxLineSummary, styles []buffer.TextStyle, pendingChar int, lm buffer.LangMode) {
	switch state {
	case SynStateIdent:
		paintIdentRange(line, *tokenStart, *i, lm, styles)
	case SynStateOperator:
		paintOperatorRange(line, *tokenStart, *i, lm, styles)
	}
}

// ------------------------------------------------------------------
// 9. Markdown / HTML Specialised Highlighters
// ------------------------------------------------------------------

// highlightMarkdown applies markdown-specific style overrides on a line.
func highlightMarkdown(line *buffer.Line, styles []buffer.TextStyle, n int) {
	if n == 0 || len(line.RuneCache) == 0 {
		return
	}
	i := 0
	// Skip leading whitespace
	for i < n && (lineGetcR(line, i) == ' ' || lineGetcR(line, i) == '\t') {
		i++
	}
	if i >= n {
		return
	}
	// Heading: line starting with '#'
	if lineGetcR(line, i) == '#' {
		fillPaint(styles, n, 0, n, aHeading())
		return
	}
	// Optional line-number prefix "L<digits>:"
	if lineGetcR(line, i) == 'L' && i+1 < n {
		checkpoint := i
		i++ // skip 'L'
		start := i
		for i < n && lineGetcR(line, i) >= '0' && lineGetcR(line, i) <= '9' {
			i++
		}
		if i > start && i < n && lineGetcR(line, i) == ':' {
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
		c := lineGetcR(line, i)
		nc := lineGetcR(line, i+1)
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

// highlightHTML applies HTML/XML-specific style overrides on a line.
func highlightHTML(line *buffer.Line, styles []buffer.TextStyle, n int, syn *buffer.SynState) {
	for i := 0; i < n; i++ {
		c := lineGetcR(line, i)
		dfa := int(syn.DFA)
		if dfa == SynStateHTMLCmt || dfa == SynStateHTMLCmtD1 || dfa == SynStateHTMLCmtD2 {
			putPaint(styles, n, i, aComment())
			if c == '-' {
				if dfa == SynStateHTMLCmtD1 {
					syn.DFA = SynStateHTMLCmtD2
				} else {
					syn.DFA = SynStateHTMLCmtD1
				}
			} else if c == '>' && dfa == SynStateHTMLCmtD2 {
				syn.DFA = SynStateNormal
			} else {
				syn.DFA = SynStateHTMLCmt
			}
		} else if c == '<' &&
			i+3 < n &&
			lineGetcR(line, i+1) == '!' &&
			lineGetcR(line, i+2) == '-' &&
			lineGetcR(line, i+3) == '-' {
			fillPaint(styles, n, i, i+4, aComment())
			syn.DFA = SynStateHTMLCmt
			i += 3
		} else {
			putPaint(styles, n, i, aNormal())
		}
	}
}

// ------------------------------------------------------------------
// 10. Main Tokenizer
// ------------------------------------------------------------------

// tokenizeLineFromState runs the DFA syntax highlighter on line from start state.
// Returns the end state, line summary, and per-rune style slice.
func tokenizeLineFromState(line *buffer.Line, start buffer.SynState) (buffer.SynState, buffer.SyntaxLineSummary, []buffer.TextStyle) {
	return tokenizeLineFromStateLimit(line, start, -1)
}

// tokenizeLineFromStateLimit scans up to scanLimit runes (-1 = full line).
func tokenizeLineFromStateLimit(line *buffer.Line, start buffer.SynState, scanLimit int) (buffer.SynState, buffer.SyntaxLineSummary, []buffer.TextStyle) {
	line.EnsureCache()

	lm := line.LangMode
	if lm == buffer.LModeNone && line.Buffer != nil {
		lm = line.Buffer.LangMode
	}
	info := For(lm)

	syn := start
	n := len(line.RuneCache)
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
			return int(line.RuneCache[i])
		}
		return 0
	}

	// ---- Special syntax kinds (handled before DFA loop) ----

	switch info.Kind {
	case ModeSyntaxNone:
		syn = buffer.SynState{}
		if fullScan {
			line.SyntaxEndState = syn
			line.SyntaxSummary = summary
			line.SyntaxValid = true
		}
		return syn, summary, styles

	case ModeSyntaxHashCommentOnly:
		syn = buffer.SynState{}
		for i := 0; i < n; i++ {
			if getc(i) == '#' {
				fillPaint(styles, n, i, n, aComment())
				syn.DFA = SynStateCmtLine
				break
			}
		}
		if fullScan {
			line.SyntaxEndState = syn
			line.SyntaxSummary = summary
			line.SyntaxValid = true
		}
		return syn, summary, styles

	case ModeSyntaxMarkdown:
		highlightMarkdown(line, styles, n)
		if fullScan {
			line.SyntaxEndState = syn
			line.SyntaxSummary = summary
			line.SyntaxValid = true
		}
		return syn, summary, styles

	case ModeSyntaxHTML:
		highlightHTML(line, styles, n, &syn)
		syn.Paren = 0
		syn.Bracket = 0
		syn.Curly = 0
		if fullScan {
			line.SyntaxEndState = syn
			line.SyntaxSummary = summary
			line.SyntaxValid = true
		}
		return syn, summary, styles
	}

	// ---- Preproc continuation ----

	if syn.DFA == SynStatePreproc {
		fillPaint(styles, n, 0, n, aPreproc())
		for i := 0; i < n; i++ {
			if getc(i) != ' ' && getc(i) != '\t' {
				noteSummaryCode(&summary, i)
				break
			}
		}
		if n == 0 || getc(n-1) != '\\' {
			syn.DFA = SynStateNormal
		}
		if fullScan {
			line.SyntaxEndState = syn
			line.SyntaxSummary = summary
			line.SyntaxValid = true
		}
		return syn, summary, styles
	}

	// ---- DFA loop ----

	flags := info.Flags
	tokenStart := 0
	pendingChar := 0

	// callEnter dispatches on_enter for state, honoring hook overrides.
	callEnter := func(state int, i *int) {
		if state >= 0 && state < ssStateCount && onEnterHooks[state] != nil {
			onEnterHooks[state](line, &syn, i, &tokenStart, &summary, styles, pendingChar)
		} else {
			doBuiltinOnEnter(state, line, &syn, i, &tokenStart, &summary, styles, pendingChar, lm)
		}
	}

	// callExit dispatches on_exit for state, honoring hook overrides.
	callExit := func(state int, i *int) {
		if state >= 0 && state < ssStateCount && onExitHooks[state] != nil {
			onExitHooks[state](line, &syn, i, &tokenStart, &summary, styles, pendingChar)
		} else {
			doBuiltinOnExit(state, line, &syn, i, &tokenStart, &summary, styles, pendingChar, lm)
		}
	}

	for i := 0; i < loopEnd; {
		cur := int(syn.DFA)
		if cur < 0 || cur >= ssStateCount {
			syn.DFA = SynStateNormal
			cur = SynStateNormal
		}
		lookahead := getc(i + 1)

		// on_enter for current state (called every iteration before transition)
		callEnter(cur, &i)

		// Transition: updates syn.DFA; returns reprocess flag
		reprocess := false
		c := getc(i)

		switch cur {
		case SynStateNormal:
			// // line comment
			if flags&ModeFlagCommentSlashLine != 0 && c == '/' && lookahead == '/' {
				fillPaint(styles, n, i, n, aComment())
				syn.DFA = SynStateCmtLine
				i = loopEnd // skip to end; loop's i++ will exit
				goto afterTransition
			}
			// /* block comment
			if flags&ModeFlagCommentSlashBlock != 0 && c == '/' && lookahead == '*' {
				putPaint(styles, n, i, aComment())
				putPaint(styles, n, i+1, aComment())
				syn.DFA = SynStateCmtBlock
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
						syn.DFA = SynStatePreproc
					} else {
						syn.DFA = SynStateNormal
					}
					i = loopEnd
					goto afterTransition
				}
			}
			// # line comment (hash comment mode)
			if flags&ModeFlagCommentHash != 0 && c == '#' {
				fillPaint(styles, n, i, n, aComment())
				syn.DFA = SynStateCmtLine
				i = loopEnd
				goto afterTransition
			}
			// ; line comment (lisp mode)
			if flags&ModeFlagCommentSemi != 0 && c == ';' {
				fillPaint(styles, n, i, n, aComment())
				syn.DFA = SynStateCmtLine
				i = loopEnd
				goto afterTransition
			}
			// -- lua comment / lua block comment
			if flags&ModeFlagCommentLua != 0 && c == '-' {
				tokenStart = i
				syn.DFA = SynStateLuaDash
				noteSummaryCode(&summary, i)
				putPaint(styles, n, i, aNormal())
				goto afterTransition
			}
			// { pascal brace comment
			if flags&ModeFlagCommentPascalBrace != 0 && c == '{' {
				putPaint(styles, n, i, aComment())
				syn.DFA = SynStateCmtBrace
				goto afterTransition
			}
			// (* pascal paren comment
			if flags&ModeFlagCommentPascalParen != 0 && c == '(' && lookahead == '*' {
				putPaint(styles, n, i, aComment())
				putPaint(styles, n, i+1, aComment())
				syn.DFA = SynStateCmtParen
				i++
				goto afterTransition
			}
			// " double-quoted string
			if c == '"' {
				noteSummaryCode(&summary, i)
				syn.DFA = SynStateStringD
				goto afterTransition
			}
			// ' single-quoted string
			if c == '\'' {
				noteSummaryCode(&summary, i)
				syn.DFA = SynStateStringS
				goto afterTransition
			}
			// number: starts with digit or '.' followed by digit
			if (c >= '0' && c <= '9') || (c == '.' && lookahead >= '0' && lookahead <= '9') {
				noteSummaryCode(&summary, i)
				tokenStart = i
				syn.DFA = SynStateNumber
				goto afterTransition
			}
			// identifier
			if isIdentStart(c, flags) {
				noteSummaryCode(&summary, i)
				tokenStart = i
				syn.DFA = SynStateIdent
				goto afterTransition
			}
			// @ at-rule (CSS)
			if flags&ModeFlagAtRule != 0 && c == '@' && lookahead >= 'a' && lookahead <= 'z' {
				noteSummaryCode(&summary, i)
				putPaint(styles, n, i, aPreproc())
				tokenStart = i + 1
				syn.DFA = SynStateIdent
				goto afterTransition
			}
			// delimiter (rainbow paren)
			if c == '(' || c == ')' || c == '[' || c == ']' ||
				(c == '{' && flags&ModeFlagNoCurlyRainbow == 0) ||
				(c == '}' && flags&ModeFlagNoCurlyRainbow == 0) {
				if delimiterIndex(c) >= 0 {
					pendingChar = c
					tokenStart = i
					reenterState(line, &syn, &i, tokenStart, pendingChar, styles, &summary)
					pendingChar = 0
					goto afterTransition
				}
			}
			// operator / assignment
			if isOperatorChar(c) {
				noteSummaryCode(&summary, i)
				tokenStart = i
				syn.DFA = SynStateOperator
				goto afterTransition
			}
			// other non-whitespace is code
			if c != ' ' && c != '\t' {
				noteSummaryCode(&summary, i)
			}

		case SynStateIdent:
			if !isIdentCont(c, flags) {
				syn.DFA = SynStateNormal
				reprocess = true
			}

		case SynStateOperator:
			if !isOperatorChar(c) {
				syn.DFA = SynStateNormal
				reprocess = true
			}

		case SynStateNumber:
			if !isNumberCont(c) {
				syn.DFA = SynStateNormal
				reprocess = true
			}

		case SynStateStringD:
			if c == '"' {
				syn.DFA = SynStateNormal
			} else if c == '\\' {
				syn.DFA = SynStateStringDEsc
			}

		case SynStateStringDEsc:
			syn.DFA = SynStateStringD

		case SynStateStringS:
			if c == '\'' {
				syn.DFA = SynStateNormal
			} else if c == '\\' {
				syn.DFA = SynStateStringSEsc
			}

		case SynStateStringSEsc:
			syn.DFA = SynStateStringS

		case SynStateCmtBlock:
			if c == '*' {
				syn.DFA = SynStateCmtStar
			}

		case SynStateCmtStar:
			if c == '/' {
				syn.DFA = SynStateNormal
			} else if c != '*' {
				syn.DFA = SynStateCmtBlock
			}

		case SynStateCmtBrace:
			if c == '}' {
				syn.DFA = SynStateNormal
			}

		case SynStateCmtParen:
			if c == '*' {
				syn.DFA = SynStateCmtParen2
			}

		case SynStateCmtParen2:
			if c == ')' {
				syn.DFA = SynStateNormal
			} else if c != '*' {
				syn.DFA = SynStateCmtParen
			}

		case SynStateCmtLine:
			// stay in line-comment state until EOL

		case SynStateLuaDash:
			if c == '-' {
				nnc := getc(i + 2)
				if lookahead == '[' && nnc == '[' {
					syn.DFA = SynStateLuaBlock
				} else {
					syn.DFA = SynStateCmtLine
				}
			} else {
				syn.DFA = SynStateNormal
				reprocess = true
			}

		case SynStateLuaBlock:
			if c == ']' {
				syn.DFA = SynStateLuaBlkEnd
			}

		case SynStateLuaBlkEnd:
			if c == ']' {
				syn.DFA = SynStateNormal
			} else {
				syn.DFA = SynStateLuaBlock
			}

		case SynStateHTMLCmt:
			if c == '-' {
				syn.DFA = SynStateHTMLCmtD1
			}
			// else stay in SynStateHTMLCmt

		case SynStateHTMLCmtD1:
			if c == '-' {
				syn.DFA = SynStateHTMLCmtD2
			} else {
				syn.DFA = SynStateHTMLCmt
			}

		case SynStateHTMLCmtD2:
			if c == '>' {
				syn.DFA = SynStateNormal
			} else if c != '-' {
				syn.DFA = SynStateHTMLCmt
			}

		default:
			syn.DFA = SynStateNormal
		}

	afterTransition:
		newState := int(syn.DFA)
		if newState < 0 || newState >= ssStateCount {
			syn.DFA = SynStateNormal
			newState = SynStateNormal
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
	if syn.DFA == SynStateIdent {
		paintIdentRange(line, tokenStart, n, lm, styles)
		syn.DFA = SynStateNormal
	}
	if syn.DFA == SynStateOperator {
		paintOperatorRange(line, tokenStart, n, lm, styles)
		syn.DFA = SynStateNormal
	}

	// States that do NOT persist across lines (reset to NORMAL at EOL)
	switch syn.DFA {
	case SynStateCmtLine, SynStateNumber, SynStateStringD, SynStateStringDEsc,
		SynStateStringS, SynStateStringSEsc, SynStateLuaDash:
		syn.DFA = SynStateNormal
	}

	// Persist block-comment / multi-line states across lines:
	// SynStateCmtBlock/Star/Brace/Paren*, SynStateLuaBlock/BlkEnd,
	// SynStateHTMLCmt*, SynStatePreproc are all left as-is.

	line.SyntaxEndState = syn
	line.SyntaxSummary = summary
	line.SyntaxValid = true
	return syn, summary, styles
}

// ------------------------------------------------------------------
// 11. SyntaxEnsureLine
// ------------------------------------------------------------------

// SyntaxEnsureLine ensures line has up-to-date syntax styles.
// It walks back through the buffer to find a valid start state when possible.
func SyntaxEnsureLine(line *buffer.Line) {
	if line == nil {
		return
	}
	if line.SyntaxValid && line.SyntaxStyles != nil {
		return
	}

	// Find start state: use previous line's end state if available.
	start := buffer.SynState{DFA: SynStateNormal}
	if line.Buffer != nil {
		buf := line.Buffer
		// Find this line's index in the buffer.
		lineNum := lineNumberInBuffer(buf, line)
		if lineNum > 1 {
			prev := buf.Line(lineNum - 1)
			if prev != nil {
				if !prev.SyntaxValid {
					SyntaxEnsureLine(prev) // ensure prev is computed first
				}
				start = prev.SyntaxEndState
			}
		}
	}

	_, summary, styles := tokenizeLineFromState(line, start)
	line.SyntaxStyles = styles
	line.SyntaxSummary = summary
}

// lineNumberInBuffer returns the 1-based line number of line within buf, or 0.
func lineNumberInBuffer(buf *buffer.Buffer, line *buffer.Line) uint {
	if buf == nil || line == nil {
		return 0
	}
	for i := range buf.Lines {
		if &buf.Lines[i] == line {
			return uint(i + 1)
		}
	}
	return 0
}

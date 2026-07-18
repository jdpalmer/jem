package search

import (
	"bytes"
	"regexp"
	"unicode"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
)

type (
	Buffer           = app.Buffer
	Window           = app.Window
	Location         = app.Location
	PromptResult     = app.PromptResult
	TextStyle        = app.TextStyle
	TransientAction  = app.TransientAction
	TransientBinding = app.TransientBinding
)

const (
	PatternCapacity       = app.PatternCapacity
	PromptResultNo        = app.PromptResultNo
	PromptResultYes       = app.PromptResultYes
	PromptResultAbort     = app.PromptResultAbort
	SearchScopeBuffer     = app.SearchScopeBuffer
	SearchScopeAllBuffers = app.SearchScopeAllBuffers
	MinibufEditUnhandled  = app.MinibufEditUnhandled
	MinibufEditNoChange   = app.MinibufEditNoChange
	CTL                   = app.CTL
	KeyEnter              = app.KeyEnter
	KeyMask               = app.KeyMask
	TermColorRed          = app.TermColorRed
)

var (
	MakeTextStyle = buffer.MakeTextStyle
	TextStyleBg   = buffer.TextStyleBg
)

type State struct {
	SearchCaseSensitive bool
	RegexSearchPattern  string
	TransientBindings   []TransientBinding
}

var DefaultState = &State{}

func currentState() *State {
	if DefaultState == nil {
		DefaultState = &State{}
	}
	return DefaultState
}

func mbWrite(format string, args ...interface{}) {
	if PackageHooks.MBWrite != nil {
		PackageHooks.MBWrite(format, args...)
	}
}

func mbClear() {
	if PackageHooks.MBClear != nil {
		PackageHooks.MBClear()
	}
}

func mbReadString(prompt, initial string) (string, PromptResult) {
	if PackageHooks.MBReadString == nil {
		return "", PromptResultAbort
	}
	return PackageHooks.MBReadString(prompt, initial)
}

func mbWritePromptStyle(prompt string, text []byte, cpos int, style TextStyle) {
	if PackageHooks.MBWritePromptStyle != nil {
		PackageHooks.MBWritePromptStyle(prompt, text, cpos, style)
	}
}

func mbHistoryAdd(text string) {
	if PackageHooks.MBHistoryAdd != nil {
		PackageHooks.MBHistoryAdd(text)
	}
}

func mbEditKeyHistory(buf []byte, cpos *int, nbuf int, initial []byte, historyPos *int16, haveSavedEdit *bool, savedEdit []byte, k uint32) app.MinibufferEditResult {
	if PackageHooks.MBEditKeyHistory == nil {
		return MinibufEditUnhandled
	}
	return PackageHooks.MBEditKeyHistory(buf, cpos, nbuf, initial, historyPos, haveSavedEdit, savedEdit, k)
}

func displayUpdate() {
	if PackageHooks.DisplayUpdate != nil {
		PackageHooks.DisplayUpdate()
	}
}

func markPushCurrent() {
	if PackageHooks.MarkPushCurrent != nil {
		PackageHooks.MarkPushCurrent()
	}
}

func isearchReadKey() (uint32, bool) {
	if PackageHooks.ReadKey == nil {
		return 0, false
	}
	return PackageHooks.ReadKey()
}

func isPasteRedrawKey(k uint32) bool {
	if PackageHooks.IsPasteRedrawKey == nil {
		return false
	}
	return PackageHooks.IsPasteRedrawKey(k)
}

func doBeep() {
	if PackageHooks.Beep != nil {
		PackageHooks.Beep()
	}
}

func setText(bp *Buffer, begin, end Location, newText []byte, newLen uint, newEndOut *Location, kill bool) bool {
	if PackageHooks.SetText != nil {
		return PackageHooks.SetText(bp, begin, end, newText, newLen, newEndOut, kill)
	}
	return buffer.SetText(bp, nil, begin, end, newText, newLen, newEndOut)
}

func truncatePattern(s string) string {
	if len(s) >= PatternCapacity {
		return s[:PatternCapacity-1]
	}
	return s
}

type ISearchSnapshot struct {
	Buffer     *Buffer
	Line       uint
	Offset     uint
	MarkLine   uint
	MarkOff    uint
	PatternLen int
}

type bufferSearchScope struct {
	buffers    []*Buffer
	allBuffers bool
}

type RegexMatch struct {
	Start Location
	End   Location
	Text  []byte
	Index []int
}

type replaceAction TransientAction

const (
	replaceActionNone    replaceAction = 0
	replaceActionYes     replaceAction = 1
	replaceActionNo      replaceAction = 2
	replaceActionAll     replaceAction = 3
	replaceActionQuit    replaceAction = 4
	replaceActionYesQuit replaceAction = 5
)

var queryReplaceBindings = []TransientBinding{
	{'y', TransientAction(replaceActionYes)},
	{' ', TransientAction(replaceActionYes)},
	{KeyEnter, TransientAction(replaceActionYes)},
	{'n', TransientAction(replaceActionNo)},
	{CTL | 'H', TransientAction(replaceActionNo)},
	{0x7F, TransientAction(replaceActionNo)},
	{'!', TransientAction(replaceActionAll)},
	{'+', TransientAction(replaceActionYesQuit)},
	{'q', TransientAction(replaceActionQuit)},
	{CTL | 'G', TransientAction(replaceActionQuit)},
	{0x1B, TransientAction(replaceActionQuit)},
}

func transientSet(bindings []TransientBinding) {
	currentState().TransientBindings = bindings
}

func transientClear() {
	currentState().TransientBindings = nil
}

func transientLookup(code uint32, defaultAction replaceAction) replaceAction {
	for _, b := range currentState().TransientBindings {
		if b.Code == code {
			return replaceAction(b.Action)
		}
	}
	return defaultAction
}

func searchScopeIsAllBuffers() bool {
	return app.State.SearchScopeSetting == SearchScopeAllBuffers
}

func buildSearchPrompt(label string) string {
	prompt := label
	if searchScopeIsAllBuffers() {
		prompt += " [all]"
	}
	return prompt + ": "
}

func updateSearchCase(pattern string) {
	st := currentState()
	st.SearchCaseSensitive = false
	for _, b := range pattern {
		if b >= 'A' && b <= 'Z' {
			st.SearchCaseSensitive = true
			return
		}
	}
}

func searchPatternBytes() []byte {
	return []byte(app.State.SearchPattern)
}

func readPattern(label string) PromptResult {
	display := buildSearchPrompt(label)
	pattern, pr := mbReadString(display, app.State.SearchPattern)
	if pr == PromptResultYes {
		app.State.SearchPattern = truncatePattern(pattern)
	} else if pr == PromptResultNo && app.State.SearchPattern != "" {
		pr = PromptResultYes
	}
	if pr == PromptResultYes {
		updateSearchCase(app.State.SearchPattern)
	}
	return pr
}

func searchScopeInit(origin *Buffer) bufferSearchScope {
	scope := bufferSearchScope{allBuffers: searchScopeIsAllBuffers()}
	scope.buffers = append(scope.buffers, origin)
	for i := 0; i < int(app.State.BufferCount); i++ {
		bp := app.State.Buffers[i]
		if bp != nil && bp != origin {
			scope.buffers = append(scope.buffers, bp)
		}
	}
	return scope
}

func searchScopeIndex(scope *bufferSearchScope, bp *Buffer) int {
	for i, b := range scope.buffers {
		if b == bp {
			return i
		}
	}
	return 0
}

func saveSearchSnapshot(wp *Window, patternLen int) ISearchSnapshot {
	s := ISearchSnapshot{Buffer: wp.Buffer, PatternLen: patternLen}
	if wp.Buffer != nil {
		s.Line = wp.Cursor.Line
		s.Offset = wp.Cursor.Offset
		s.MarkLine = wp.Mark.Line
		s.MarkOff = wp.Mark.Offset
	}
	return s
}

func restoreSearchSnapshot(wp *Window, snap *ISearchSnapshot) {
	if snap == nil || snap.Buffer == nil || snap.Line == 0 {
		return
	}
	if app.State.CurrentBuffer != snap.Buffer {
		if app.PackageHooks.SwitchBuffer != nil {
			app.PackageHooks.SwitchBuffer(snap.Buffer)
		} else {
			app.SetCurrentBuffer(snap.Buffer)
		}
		wp = app.State.CurrentWindow
	}
	if wp == nil {
		return
	}
	app.WindowSetCursor(wp, Location{Line: snap.Line, Offset: snap.Offset})
	wp.Mark.Line = snap.MarkLine
	wp.Mark.Offset = snap.MarkOff
	wp.DidMove = true
}

func isearchClearHighlight(wp *Window) {
	if wp == nil {
		return
	}
	if wp.Mark.Line != 0 {
		wp.ShouldRedraw = true
	}
	wp.Mark.Line = 0
	wp.Mark.Offset = 0
}

func searchSwitchBuffer(wp *Window, bp *Buffer, loc Location) {
	if bp == nil || loc.Line == 0 {
		return
	}
	if app.State.CurrentBuffer != bp {
		if app.PackageHooks.SwitchBuffer != nil {
			app.PackageHooks.SwitchBuffer(bp)
		} else {
			app.SetCurrentBuffer(bp)
		}
		wp = app.State.CurrentWindow
	}
	if wp != nil {
		app.WindowSetCursor(wp, loc)
		wp.DidMove = true
	}
}

func bufferSearchStart(bp *Buffer) Location { return Location{Line: 1, Offset: 0} }
func bufferSearchEnd(bp *Buffer) Location   { return Location{Line: buffer.EOF(bp), Offset: 0} }

func searchCharsEqual(bc, pc int) bool {
	if currentState().SearchCaseSensitive {
		return bc == pc
	}
	return unicode.ToUpper(rune(bc)) == unicode.ToUpper(rune(pc))
}

func searchReadForward(bp *Buffer, line *uint, offset *uint) (int, bool) {
	if bp == nil || line == nil || offset == nil {
		return -1, false
	}
	if *line > bp.LineCount {
		return -1, false
	}
	lp := buffer.GetLine(bp, *line)
	if lp == nil {
		return -1, false
	}
	if *offset >= buffer.LineLength(lp) {
		*line++
		*offset = 0
		return '\n', true
	}
	c := int(lp.Data[*offset])
	*offset++
	return c, true
}

func searchReadBackward(bp *Buffer, line *uint, offset *uint) (int, bool) {
	if bp == nil || line == nil || offset == nil {
		return -1, false
	}
	if *offset == 0 {
		if *line <= 1 {
			return -1, false
		}
		*line--
		lp := buffer.GetLine(bp, *line)
		if lp == nil {
			return -1, false
		}
		*offset = buffer.LineLength(lp) + 1
	}
	*offset--
	lp := buffer.GetLine(bp, *line)
	if lp == nil {
		return -1, false
	}
	if *offset == buffer.LineLength(lp) {
		return '\n', true
	}
	return int(lp.Data[*offset]), true
}

func findNextPlain(wp *Window, pattern []byte) bool {
	if wp == nil || wp.Buffer == nil || len(pattern) == 0 {
		return false
	}
	bp := wp.Buffer
	cline := wp.Cursor.Line
	cbo := wp.Cursor.Offset

	for cline <= bp.LineCount {
		c, ok := searchReadForward(bp, &cline, &cbo)
		if !ok {
			break
		}
		if !searchCharsEqual(c, int(pattern[0])) {
			continue
		}
		tline := cline
		tbo := cbo
		matched := true
		for i := 1; i < len(pattern); i++ {
			if tline > bp.LineCount {
				matched = false
				break
			}
			nc, ok := searchReadForward(bp, &tline, &tbo)
			if !ok || !searchCharsEqual(nc, int(pattern[i])) {
				matched = false
				break
			}
		}
		if matched {
			app.WindowSetCursor(wp, Location{Line: tline, Offset: tbo})
			wp.DidMove = true
			return true
		}
	}
	return false
}

func findPrevPlain(wp *Window, pattern []byte) bool {
	if wp == nil || wp.Buffer == nil || len(pattern) == 0 {
		return false
	}
	bp := wp.Buffer
	last := len(pattern) - 1
	cline := wp.Cursor.Line
	cbo := wp.Cursor.Offset

	for {
		c, ok := searchReadBackward(bp, &cline, &cbo)
		if !ok {
			return false
		}
		if !searchCharsEqual(c, int(pattern[last])) {
			continue
		}
		tline := cline
		tbo := cbo
		matched := true
		for i := last - 1; i >= 0; i-- {
			nc, ok := searchReadBackward(bp, &tline, &tbo)
			if !ok || !searchCharsEqual(nc, int(pattern[i])) {
				matched = false
				break
			}
		}
		if matched {
			app.WindowSetCursor(wp, Location{Line: tline, Offset: tbo})
			wp.DidMove = true
			return true
		}
	}
}

func findNextInScope(wp *Window, scope *bufferSearchScope, pattern []byte) bool {
	if wp == nil || scope == nil {
		return false
	}
	origin := wp.Buffer
	idx := searchScopeIndex(scope, origin)
	if findNextPlain(wp, pattern) {
		return true
	}
	if !scope.allBuffers {
		return false
	}
	for step := 1; step < len(scope.buffers); step++ {
		bp := scope.buffers[(idx+step)%len(scope.buffers)]
		searchSwitchBuffer(wp, bp, bufferSearchStart(bp))
		wp = app.State.CurrentWindow
		if wp != nil && findNextPlain(wp, pattern) {
			return true
		}
	}
	return false
}

func findPrevInScope(wp *Window, scope *bufferSearchScope, pattern []byte) bool {
	if wp == nil || scope == nil {
		return false
	}
	origin := wp.Buffer
	idx := searchScopeIndex(scope, origin)
	if findPrevPlain(wp, pattern) {
		return true
	}
	if !scope.allBuffers {
		return false
	}
	for step := 1; step < len(scope.buffers); step++ {
		bp := scope.buffers[(idx+step)%len(scope.buffers)]
		searchSwitchBuffer(wp, bp, bufferSearchEnd(bp))
		wp = app.State.CurrentWindow
		if wp != nil && findPrevPlain(wp, pattern) {
			return true
		}
	}
	return false
}

func isearchHighlightPlain(wp *Window, patLen int, backward bool) {
	if wp == nil || patLen <= 0 {
		return
	}
	cursor := wp.Cursor
	var mark Location
	if backward {
		mark = buffer.LocationAdvanceBytes(wp.Buffer, cursor, patLen)
	} else {
		mark = buffer.LocationRewindBytes(wp.Buffer, cursor, patLen)
	}
	wp.Mark.Line = mark.Line
	wp.Mark.Offset = mark.Offset
	wp.ShouldRedraw = true
}

func isearchHighlightMatch(wp *Window, start, end Location, backward bool) {
	if wp == nil {
		return
	}
	if backward {
		wp.Mark = end
	} else {
		wp.Mark = start
	}
	wp.ShouldRedraw = true
}

func isearchSetPlainPattern(pattern string) {
	app.State.SearchPattern = truncatePattern(pattern)
	updateSearchCase(app.State.SearchPattern)
}

func isearchRunPlain(wp *Window, scope *bufferSearchScope, start *ISearchSnapshot, pattern []byte, backward bool, out *ISearchSnapshot) bool {
	if wp == nil || scope == nil || start == nil || out == nil {
		return false
	}
	restoreSearchSnapshot(wp, start)
	wp = app.State.CurrentWindow
	if len(pattern) == 0 {
		*out = saveSearchSnapshot(wp, 0)
		return true
	}
	isearchSetPlainPattern(string(pattern))
	ok := false
	if backward {
		ok = findPrevInScope(wp, scope, pattern)
	} else {
		ok = findNextInScope(wp, scope, pattern)
	}
	if !ok {
		return false
	}
	wp = app.State.CurrentWindow
	isearchHighlightPlain(wp, len(pattern), backward)
	*out = saveSearchSnapshot(wp, len(pattern))
	return true
}

func locationCompare(a, b Location) int {
	if a.Line != b.Line {
		if a.Line < b.Line {
			return -1
		}
		return 1
	}
	if a.Offset < b.Offset {
		return -1
	}
	if a.Offset > b.Offset {
		return 1
	}
	return 0
}

func bufferSliceFrom(bp *Buffer, start Location) []byte {
	if bp == nil || start.Line == 0 {
		return nil
	}
	var length uint
	return buffer.GetText(bp, start, bufferSearchEnd(bp), &length)
}

func findNextRegexMatchFrom(bp *Buffer, searchStart Location, pattern string) (RegexMatch, int) {
	text := bufferSliceFrom(bp, searchStart)
	if len(text) == 0 {
		return RegexMatch{}, 0
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		mbWrite("[invalid regular expression]")
		return RegexMatch{}, -1
	}
	loc := re.FindIndex(text)
	if loc == nil {
		return RegexMatch{}, 0
	}
	if loc[0] == loc[1] {
		mbWrite("[zero-length regex matches not supported]")
		return RegexMatch{}, -1
	}
	match := RegexMatch{
		Text:  text,
		Index: re.FindSubmatchIndex(text),
		Start: buffer.LocationAdvanceBytes(bp, searchStart, loc[0]),
		End:   buffer.LocationAdvanceBytes(bp, searchStart, loc[1]),
	}
	return match, 1
}

func findPrevRegexMatchFrom(bp *Buffer, limit Location, pattern string) (RegexMatch, int) {
	scan := Location{Line: 1, Offset: 0}
	var best RegexMatch
	haveBest := false
	for {
		candidate, found := findNextRegexMatchFrom(bp, scan, pattern)
		if found < 0 {
			return RegexMatch{}, -1
		}
		if found == 0 {
			break
		}
		if locationCompare(candidate.End, limit) > 0 {
			break
		}
		best = candidate
		haveBest = true
		scan = buffer.LocationAdvanceBytes(bp, candidate.Start, 1)
	}
	if !haveBest {
		return RegexMatch{}, 0
	}
	return best, 1
}

func findNextRegexInScope(wp *Window, scope *bufferSearchScope, pattern string) (RegexMatch, int) {
	if wp == nil || scope == nil {
		return RegexMatch{}, 0
	}
	origin := wp.Buffer
	idx := searchScopeIndex(scope, origin)
	match, found := findNextRegexMatchFrom(wp.Buffer, wp.Cursor, pattern)
	if found != 0 || !scope.allBuffers {
		return match, found
	}
	for step := 1; step < len(scope.buffers); step++ {
		bp := scope.buffers[(idx+step)%len(scope.buffers)]
		searchSwitchBuffer(wp, bp, bufferSearchStart(bp))
		wp = app.State.CurrentWindow
		if wp == nil {
			continue
		}
		match, found = findNextRegexMatchFrom(bp, wp.Cursor, pattern)
		if found != 0 {
			return match, found
		}
	}
	return RegexMatch{}, 0
}

func findPrevRegexInScope(wp *Window, scope *bufferSearchScope, pattern string) (RegexMatch, int) {
	if wp == nil || scope == nil {
		return RegexMatch{}, 0
	}
	origin := wp.Buffer
	idx := searchScopeIndex(scope, origin)
	match, found := findPrevRegexMatchFrom(wp.Buffer, wp.Cursor, pattern)
	if found != 0 || !scope.allBuffers {
		return match, found
	}
	for step := 1; step < len(scope.buffers); step++ {
		bp := scope.buffers[(idx+step)%len(scope.buffers)]
		searchSwitchBuffer(wp, bp, bufferSearchEnd(bp))
		wp = app.State.CurrentWindow
		if wp == nil {
			continue
		}
		match, found = findPrevRegexMatchFrom(bp, wp.Cursor, pattern)
		if found != 0 {
			return match, found
		}
	}
	return RegexMatch{}, 0
}

func isearchRunRegex(wp *Window, scope *bufferSearchScope, start *ISearchSnapshot, pattern string, backward bool, out *ISearchSnapshot) bool {
	if wp == nil || scope == nil || start == nil || out == nil {
		return false
	}
	restoreSearchSnapshot(wp, start)
	wp = app.State.CurrentWindow
	if pattern == "" {
		*out = saveSearchSnapshot(wp, 0)
		return true
	}
	var match RegexMatch
	var found int
	if backward {
		match, found = findPrevRegexInScope(wp, scope, pattern)
	} else {
		match, found = findNextRegexInScope(wp, scope, pattern)
	}
	if found < 0 {
		isearchClearHighlight(wp)
		return false
	}
	if found == 0 {
		isearchClearHighlight(wp)
		return false
	}
	wp = app.State.CurrentWindow
	if backward {
		app.WindowSetCursor(wp, match.Start)
	} else {
		app.WindowSetCursor(wp, match.End)
	}
	wp.DidMove = true
	isearchHighlightMatch(wp, match.Start, match.End, backward)
	*out = saveSearchSnapshot(wp, len(pattern))
	return true
}

func writeISearchPrompt(label string, pattern []byte, cpos int, failing bool, bp *Buffer) {
	prompt := label
	if searchScopeIsAllBuffers() && bp != nil {
		prompt += "[all " + bp.Name + "]: "
	} else {
		prompt += ": "
	}
	style := app.State.Theme.NormalStyle
	if failing {
		style = MakeTextStyle(TermColorRed, TextStyleBg(app.State.Theme.NormalStyle), 0)
	}
	end := bytes.IndexByte(pattern, 0)
	if end < 0 {
		end = len(pattern)
	}
	mbWritePromptStyle(prompt, pattern[:end], cpos, style)
}

func isearchPlainLoop(backward bool) bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	scope := searchScopeInit(bp)
	markPushCurrent()
	origin := saveSearchSnapshot(wp, 0)
	lastSuccess := origin
	var pat [PatternCapacity]byte
	var savedEdit [PatternCapacity]byte
	cpos := 0
	var historyPos int16 = -1
	haveSavedEdit := false
	failing := false
	repeatKey := CTL | 'S'
	label := "isearch forward"
	if backward {
		repeatKey = CTL | 'R'
		label = "isearch backward"
	}

	mbState := app.MinibufferState{}
	app.State.ActiveMinibuffer = &mbState
	defer func() { app.State.ActiveMinibuffer = nil }()

	app.State.ShowPhantomCursor = true
	defer func() {
		app.State.ShowPhantomCursor = false
		isearchClearHighlight(app.State.CurrentWindow)
	}()

	for {
		plen := bytes.IndexByte(pat[:], 0)
		if plen < 0 {
			plen = len(pat)
		}
		displayUpdate()
		writeISearchPrompt(label, pat[:], cpos, failing, wp.Buffer)

		k, ok := isearchReadKey()
		if !ok {
			return false
		}
		if isPasteRedrawKey(k) {
			continue
		}
		if k == (CTL|'G') || k == 0x1B {
			restoreSearchSnapshot(wp, &origin)
			mbWrite("[cancelled]")
			return false
		}
		if k == KeyEnter || k == '\r' || k == '\n' || k == (CTL|'M') || k == (CTL|'J') {
			if plen > 0 {
				isearchSetPlainPattern(string(pat[:plen]))
				mbHistoryAdd(string(pat[:plen]))
			}
			mbClear()
			return true
		}
		if k == repeatKey {
			if plen == 0 {
				old := searchPatternBytes()
				if len(old) > 0 {
					copy(pat[:], old)
					pat[len(old)] = 0
					cpos = len(old)
					plen = len(old)
				}
			}
			if plen == 0 {
				continue
			}
			var next ISearchSnapshot
			wp = app.State.CurrentWindow
			if isearchRunPlain(wp, &scope, &lastSuccess, pat[:plen], backward, &next) {
				lastSuccess = next
				failing = false
			} else {
				restoreSearchSnapshot(wp, &lastSuccess)
				failing = true
			}
			continue
		}

		oldPat := string(pat[:plen])
		edit := mbEditKeyHistory(pat[:], &cpos, PatternCapacity, searchPatternBytes(), &historyPos, &haveSavedEdit, savedEdit[:], k)
		if edit == MinibufEditUnhandled {
			if plen > 0 {
				isearchSetPlainPattern(string(pat[:plen]))
				mbHistoryAdd(string(pat[:plen]))
			}
			mbClear()
			return true
		}
		if edit == MinibufEditNoChange {
			doBeep()
			continue
		}
		plen = bytes.IndexByte(pat[:], 0)
		if plen < 0 {
			plen = len(pat)
		}
		if string(pat[:plen]) == oldPat {
			continue
		}
		if plen == 0 {
			restoreSearchSnapshot(wp, &origin)
			lastSuccess = origin
			failing = false
			continue
		}
		var next ISearchSnapshot
		wp = app.State.CurrentWindow
		if isearchRunPlain(wp, &scope, &origin, pat[:plen], backward, &next) {
			lastSuccess = next
			failing = false
		} else {
			restoreSearchSnapshot(wp, &lastSuccess)
			failing = true
		}
	}
}

func isearchRegexLoop(backward bool) bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	scope := searchScopeInit(bp)
	origin := saveSearchSnapshot(wp, 0)
	lastSuccess := origin
	var pat [PatternCapacity]byte
	var savedEdit [PatternCapacity]byte
	cpos := 0
	var historyPos int16 = -1
	haveSavedEdit := false
	failing := false
	repeatKey := CTL | 'S'
	label := "RE isearch forward"
	if backward {
		repeatKey = CTL | 'R'
		label = "RE isearch backward"
	}

	mbState := app.MinibufferState{}
	app.State.ActiveMinibuffer = &mbState
	defer func() { app.State.ActiveMinibuffer = nil }()

	app.State.ShowPhantomCursor = true
	defer func() {
		app.State.ShowPhantomCursor = false
		isearchClearHighlight(app.State.CurrentWindow)
	}()

	for {
		plen := bytes.IndexByte(pat[:], 0)
		if plen < 0 {
			plen = len(pat)
		}
		displayUpdate()
		writeISearchPrompt(label, pat[:], cpos, failing, wp.Buffer)

		k, ok := isearchReadKey()
		if !ok {
			return false
		}
		if isPasteRedrawKey(k) {
			continue
		}
		if k == (CTL|'G') || k == 0x1B {
			restoreSearchSnapshot(wp, &origin)
			mbWrite("[cancelled]")
			return false
		}
		if k == KeyEnter || k == '\r' || k == '\n' || k == (CTL|'M') || k == (CTL|'J') {
			if plen > 0 {
				currentState().RegexSearchPattern = string(pat[:plen])
				mbHistoryAdd(string(pat[:plen]))
			}
			mbClear()
			return true
		}
		if k == repeatKey {
			if plen == 0 {
				if currentState().RegexSearchPattern != "" {
					copy(pat[:], currentState().RegexSearchPattern)
					pat[len(currentState().RegexSearchPattern)] = 0
					cpos = len(currentState().RegexSearchPattern)
					plen = len(currentState().RegexSearchPattern)
				}
			}
			if plen == 0 {
				continue
			}
			var next ISearchSnapshot
			wp = app.State.CurrentWindow
			if isearchRunRegex(wp, &scope, &lastSuccess, string(pat[:plen]), backward, &next) {
				lastSuccess = next
				failing = false
			} else {
				restoreSearchSnapshot(wp, &lastSuccess)
				failing = true
			}
			continue
		}

		oldPat := string(pat[:plen])
		initial := []byte(currentState().RegexSearchPattern)
		edit := mbEditKeyHistory(pat[:], &cpos, PatternCapacity, initial, &historyPos, &haveSavedEdit, savedEdit[:], k)
		if edit == MinibufEditUnhandled {
			if plen > 0 {
				currentState().RegexSearchPattern = string(pat[:plen])
				mbHistoryAdd(string(pat[:plen]))
			}
			mbClear()
			return true
		}
		if edit == MinibufEditNoChange {
			doBeep()
			continue
		}
		plen = bytes.IndexByte(pat[:], 0)
		if plen < 0 {
			plen = len(pat)
		}
		if string(pat[:plen]) == oldPat {
			continue
		}
		if plen == 0 {
			restoreSearchSnapshot(wp, &origin)
			lastSuccess = origin
			failing = false
			continue
		}
		var next ISearchSnapshot
		wp = app.State.CurrentWindow
		if isearchRunRegex(wp, &scope, &origin, string(pat[:plen]), backward, &next) {
			lastSuccess = next
			failing = false
		} else {
			restoreSearchSnapshot(wp, &lastSuccess)
			failing = true
		}
	}
}

type matchCase int

const (
	matchCaseLower matchCase = iota
	matchCaseUpper
	matchCaseCapitalized
)

func markMatchStart(wp *Window, patLen int) {
	if wp == nil || wp.Buffer == nil || patLen == 0 {
		return
	}
	line := wp.Cursor.Line
	off := wp.Cursor.Offset
	for i := 0; i < patLen; i++ {
		if off > 0 {
			off--
		} else if line > 1 {
			line--
			lp := buffer.GetLine(wp.Buffer, line)
			if lp != nil {
				off = buffer.LineLength(lp)
			}
		}
	}
	wp.Mark = Location{Line: line, Offset: off}
}

func markMatchLocation(wp *Window, start Location) {
	if wp != nil {
		wp.Mark = start
	}
}

func checkMatchCase(wp *Window, patLen int) matchCase {
	if wp == nil || wp.Buffer == nil || patLen == 0 {
		return matchCaseLower
	}
	lp := buffer.GetLine(wp.Buffer, wp.Cursor.Line)
	if lp == nil || wp.Cursor.Offset < uint(patLen) {
		return matchCaseLower
	}
	start := int(wp.Cursor.Offset) - patLen
	text := lp.Data[start : start+patLen]
	if len(text) == 0 || !unicode.IsUpper(rune(text[0])) {
		return matchCaseLower
	}
	for i := 1; i < len(text); i++ {
		if unicode.IsLower(rune(text[i])) {
			return matchCaseCapitalized
		}
	}
	return matchCaseUpper
}

func applyMatchCase(mc matchCase, repl []byte, out []byte) int {
	n := len(repl)
	if n >= len(out) {
		n = len(out) - 1
	}
	copy(out, repl[:n])
	out[n] = 0
	switch mc {
	case matchCaseUpper:
		for i := 0; i < n; i++ {
			out[i] = byte(unicode.ToUpper(rune(out[i])))
		}
	case matchCaseCapitalized:
		if n > 0 {
			out[0] = byte(unicode.ToUpper(rune(out[0])))
		}
	}
	return n
}

func doReplace(wp *Window, patLen int, repl []byte) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	end := wp.Cursor
	begin := buffer.LocationRewindBytes(wp.Buffer, end, patLen)
	return setText(wp.Buffer, begin, end, repl, uint(len(repl)), nil, false)
}

func doReplacePreservingCase(wp *Window, patLen int, repl []byte, preserve bool) bool {
	if preserve {
		mc := checkMatchCase(wp, patLen)
		if mc != matchCaseLower {
			var caseRepl [PatternCapacity]byte
			n := applyMatchCase(mc, repl, caseRepl[:])
			return doReplace(wp, patLen, caseRepl[:n])
		}
	}
	return doReplace(wp, patLen, repl)
}

func doReplaceRange(wp *Window, start, end Location, repl []byte) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	return setText(wp.Buffer, start, end, repl, uint(len(repl)), nil, false)
}

func writeReplacePrompt(bp *Buffer, from, to string) {
	prompt := ""
	if searchScopeIsAllBuffers() && bp != nil {
		prompt = "[" + bp.Name + "] "
	}
	prompt += "replace '" + from + "' with '" + to + "' (y/n/!/+/q): "
	mbWrite("%s", prompt)
}

func expandRegexReplacement(repl string, match RegexMatch) ([]byte, error) {
	var out bytes.Buffer
	text := match.Text
	indices := match.Index
	for i := 0; i < len(repl); i++ {
		if repl[i] == '\\' && i+1 < len(repl) {
			esc := repl[i+1]
			i++
			if esc >= '0' && esc <= '9' {
				group := int(esc - '0')
				start, end := -1, -1
				if group*2+1 < len(indices) {
					start = indices[group*2]
					end = indices[group*2+1]
				}
				if start >= 0 && end >= start && end <= len(text) {
					out.Write(text[start:end])
				}
				continue
			}
			if esc == 'n' {
				out.WriteByte('\n')
				continue
			}
			out.WriteByte(esc)
			continue
		}
		out.WriteByte(repl[i])
	}
	return out.Bytes(), nil
}

func readReplaceKey() replaceAction {
	for {
		k, ok := isearchReadKey()
		if !ok {
			return replaceActionQuit
		}
		if isPasteRedrawKey(k) {
			continue
		}
		action := transientLookup(k, replaceActionNone)
		if action == replaceActionNone {
			doBeep()
			continue
		}
		return action
	}
}

func SearchForward() bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	if readPattern("Search") != PromptResultYes {
		return false
	}
	pat := searchPatternBytes()
	if len(pat) == 0 {
		return false
	}
	scope := searchScopeInit(bp)
	if findNextInScope(wp, &scope, pat) {
		return true
	}
	mbWrite("[not found]")
	return false
}

func SearchBackward() bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	if readPattern("Reverse search") != PromptResultYes {
		return false
	}
	pat := searchPatternBytes()
	if len(pat) == 0 {
		return false
	}
	scope := searchScopeInit(bp)
	if findPrevInScope(wp, &scope, pat) {
		return true
	}
	mbWrite("[not found]")
	return false
}

func IsearchForward() bool    { return isearchPlainLoop(false) }
func IsearchBackward() bool   { return isearchPlainLoop(true) }
func IsearchReForward() bool  { return isearchRegexLoop(false) }
func IsearchReBackward() bool { return isearchRegexLoop(true) }

func ToggleSearchScope() bool {
	if app.State.SearchScopeSetting == SearchScopeBuffer {
		app.State.SearchScopeSetting = SearchScopeAllBuffers
	} else {
		app.State.SearchScopeSetting = SearchScopeBuffer
	}
	if searchScopeIsAllBuffers() {
		mbWrite("[search scope: all buffers]")
	} else {
		mbWrite("[search scope: current buffer]")
	}
	return true
}

func QueryReplace() bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	if readPattern("replace") != PromptResultYes {
		return false
	}
	pat := searchPatternBytes()
	patLen := len(pat)
	if patLen == 0 {
		return false
	}
	preserveCase := !currentState().SearchCaseSensitive

	repl, pr := mbReadString("Replace '"+string(pat)+"' with: ", "")
	if pr == PromptResultAbort {
		return false
	}
	replBytes := []byte(repl)

	scope := searchScopeInit(bp)
	transientSet(queryReplaceBindings)
	defer transientClear()

	mbState := app.MinibufferState{}
	app.State.ActiveMinibuffer = &mbState
	defer func() { app.State.ActiveMinibuffer = nil }()

	nReplaced := 0
	replaceAll := false
	displayUpdate()

	for {
		wp = app.State.CurrentWindow
		if !findNextInScope(wp, &scope, pat) {
			wp.Mark.Line = 0
			wp.ShouldRedraw = true
			suffix := "s"
			if nReplaced == 1 {
				suffix = ""
			}
			mbWrite("[replaced %d occurrence%s]", nReplaced, suffix)
			return true
		}
		if replaceAll {
			if !doReplacePreservingCase(wp, patLen, replBytes, preserveCase) {
				return false
			}
			nReplaced++
			continue
		}

		markMatchStart(wp, patLen)
		wp.ShouldRedraw = true
		displayUpdate()
		writeReplacePrompt(bp, string(pat), repl)

		switch readReplaceKey() {
		case replaceActionYes:
			if !doReplacePreservingCase(wp, patLen, replBytes, preserveCase) {
				return false
			}
			nReplaced++
		case replaceActionYesQuit:
			if !doReplacePreservingCase(wp, patLen, replBytes, preserveCase) {
				return false
			}
			nReplaced++
			suffix := "s"
			if nReplaced == 1 {
				suffix = ""
			}
			mbWrite("Replaced %d occurrence%s", nReplaced, suffix)
			return true
		case replaceActionNo:
		case replaceActionAll:
			if !doReplacePreservingCase(wp, patLen, replBytes, preserveCase) {
				return false
			}
			nReplaced++
			replaceAll = true
		case replaceActionQuit:
			suffix := "s"
			if nReplaced == 1 {
				suffix = ""
			}
			mbWrite("Replaced %d occurrence%s", nReplaced, suffix)
			return true
		}
		wp.Mark.Line = 0
		wp.ShouldRedraw = true
	}
}

func QueryReReplace() bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	pattern, pr := mbReadString(buildSearchPrompt("Query re-replace"), app.State.SearchPattern)
	if pr != PromptResultYes {
		return false
	}
	if pattern == "" {
		return false
	}

	replStr, pr := mbReadString("Replace '"+pattern+"' with (\\0..\\9): ", "")
	if pr == PromptResultAbort {
		return false
	}

	scope := searchScopeInit(bp)
	transientSet(queryReplaceBindings)
	defer transientClear()

	mbState := app.MinibufferState{}
	app.State.ActiveMinibuffer = &mbState
	defer func() { app.State.ActiveMinibuffer = nil }()

	nReplaced := 0
	replaceAll := false
	displayUpdate()

	for {
		wp = app.State.CurrentWindow
		match, found := findNextRegexInScope(wp, &scope, pattern)
		if found < 0 {
			wp.Mark.Line = 0
			wp.ShouldRedraw = true
			return false
		}
		if found == 0 {
			wp.Mark.Line = 0
			wp.ShouldRedraw = true
			suffix := "s"
			if nReplaced == 1 {
				suffix = ""
			}
			mbWrite("[replaced %d occurrence%s]", nReplaced, suffix)
			return true
		}

		expanded, err := expandRegexReplacement(replStr, match)
		if err != nil {
			return false
		}
		matchText := string(match.Text[match.Index[0]:match.Index[1]])

		if replaceAll {
			if !doReplaceRange(wp, match.Start, match.End, expanded) {
				return false
			}
			nReplaced++
			continue
		}

		markMatchLocation(wp, match.Start)
		wp.ShouldRedraw = true
		displayUpdate()
		writeReplacePrompt(bp, matchText, string(expanded))

		switch readReplaceKey() {
		case replaceActionYes:
			if !doReplaceRange(wp, match.Start, match.End, expanded) {
				return false
			}
			nReplaced++
		case replaceActionYesQuit:
			if !doReplaceRange(wp, match.Start, match.End, expanded) {
				return false
			}
			nReplaced++
			suffix := "s"
			if nReplaced == 1 {
				suffix = ""
			}
			mbWrite("Replaced %d occurrence%s", nReplaced, suffix)
			return true
		case replaceActionNo:
		case replaceActionAll:
			if !doReplaceRange(wp, match.Start, match.End, expanded) {
				return false
			}
			nReplaced++
			replaceAll = true
		case replaceActionQuit:
			suffix := "s"
			if nReplaced == 1 {
				suffix = ""
			}
			mbWrite("Replaced %d occurrence%s", nReplaced, suffix)
			return true
		}
		wp.Mark.Line = 0
		wp.ShouldRedraw = true
	}
}

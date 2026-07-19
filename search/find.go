package search

import (
	"regexp"
	"unicode"

	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
)

func searchScopeIsAllBuffers() bool {
	return model.State.SearchScopeSetting == model.SearchScopeAllBuffers
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
	return []byte(model.State.SearchPattern)
}

func readPattern(label string, onDone func(pr model.PromptResult)) {
	display := buildSearchPrompt(label)
	askString(display, model.State.SearchPattern, func(pattern string, pr model.PromptResult) {
		if pr == model.PromptResultYes {
			model.State.SearchPattern = truncatePattern(pattern)
		} else if pr == model.PromptResultNo && model.State.SearchPattern != "" {
			pr = model.PromptResultYes
		}
		if pr == model.PromptResultYes {
			updateSearchCase(model.State.SearchPattern)
		}
		if onDone != nil {
			onDone(pr)
		}
	})
}

func searchScopeInit(origin *buffer.Buffer) bufferSearchScope {
	scope := bufferSearchScope{allBuffers: searchScopeIsAllBuffers()}
	scope.buffers = append(scope.buffers, origin)
	for i := 0; i < int(len(model.State.Buffers)); i++ {
		bp := model.State.Buffers[i]
		if bp != nil && bp != origin {
			scope.buffers = append(scope.buffers, bp)
		}
	}
	return scope
}

func searchScopeIndex(scope *bufferSearchScope, bp *buffer.Buffer) int {
	for i, b := range scope.buffers {
		if b == bp {
			return i
		}
	}
	return 0
}

func saveSearchSnapshot(wp *model.Window, patternLen int) ISearchSnapshot {
	s := ISearchSnapshot{Buffer: wp.Buffer, PatternLen: patternLen}
	if wp.Buffer != nil {
		s.Line = wp.Cursor.Line
		s.Offset = wp.Cursor.Offset
		s.MarkLine = wp.Mark.Line
		s.MarkOff = wp.Mark.Offset
	}
	return s
}

func restoreSearchSnapshot(wp *model.Window, snap *ISearchSnapshot) {
	if snap == nil || snap.Buffer == nil || snap.Line == 0 {
		return
	}
	if model.State.CurrentBuffer != snap.Buffer {
		if model.PackageHooks.SwitchBuffer != nil {
			model.PackageHooks.SwitchBuffer(snap.Buffer)
		} else {
			model.SetCurrentBuffer(snap.Buffer)
		}
		wp = model.State.CurrentWindow
	}
	if wp == nil {
		return
	}
	wp.SetCursor(buffer.Location{Line: snap.Line, Offset: snap.Offset})
	wp.Mark.Line = snap.MarkLine
	wp.Mark.Offset = snap.MarkOff
	wp.DidMove = true
}

func isearchClearHighlight(wp *model.Window) {
	if wp == nil {
		return
	}
	if wp.Mark.Line != 0 {
		wp.ShouldRedraw = true
	}
	wp.Mark.Line = 0
	wp.Mark.Offset = 0
}

func searchSwitchBuffer(wp *model.Window, bp *buffer.Buffer, loc buffer.Location) {
	if bp == nil || loc.Line == 0 {
		return
	}
	if model.State.CurrentBuffer != bp {
		if model.PackageHooks.SwitchBuffer != nil {
			model.PackageHooks.SwitchBuffer(bp)
		} else {
			model.SetCurrentBuffer(bp)
		}
		wp = model.State.CurrentWindow
	}
	if wp != nil {
		wp.SetCursor(loc)
		wp.DidMove = true
	}
}

func bufferSearchStart(bp *buffer.Buffer) buffer.Location { return buffer.Location{Line: 1, Offset: 0} }
func bufferSearchEnd(bp *buffer.Buffer) buffer.Location {
	return buffer.Location{Line: bp.EOF(), Offset: 0}
}

func searchCharsEqual(bc, pc int) bool {
	if currentState().SearchCaseSensitive {
		return bc == pc
	}
	return unicode.ToUpper(rune(bc)) == unicode.ToUpper(rune(pc))
}

func searchReadForward(bp *buffer.Buffer, line *uint, offset *uint) (int, bool) {
	if bp == nil || line == nil || offset == nil {
		return -1, false
	}
	if *line > bp.LineCount {
		return -1, false
	}
	lp := bp.Line(*line)
	if lp == nil {
		return -1, false
	}
	if *offset >= lp.Len() {
		*line++
		*offset = 0
		return '\n', true
	}
	c := int(lp.Data[*offset])
	*offset++
	return c, true
}

func searchReadBackward(bp *buffer.Buffer, line *uint, offset *uint) (int, bool) {
	if bp == nil || line == nil || offset == nil {
		return -1, false
	}
	if *offset == 0 {
		if *line <= 1 {
			return -1, false
		}
		*line--
		lp := bp.Line(*line)
		if lp == nil {
			return -1, false
		}
		*offset = lp.Len() + 1
	}
	*offset--
	lp := bp.Line(*line)
	if lp == nil {
		return -1, false
	}
	if *offset == lp.Len() {
		return '\n', true
	}
	return int(lp.Data[*offset]), true
}

func findNextPlain(wp *model.Window, pattern []byte) bool {
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
			wp.SetCursor(buffer.Location{Line: tline, Offset: tbo})
			wp.DidMove = true
			return true
		}
	}
	return false
}

func findPrevPlain(wp *model.Window, pattern []byte) bool {
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
			wp.SetCursor(buffer.Location{Line: tline, Offset: tbo})
			wp.DidMove = true
			return true
		}
	}
}

func findNextInScope(wp *model.Window, scope *bufferSearchScope, pattern []byte) bool {
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
		wp = model.State.CurrentWindow
		if wp != nil && findNextPlain(wp, pattern) {
			return true
		}
	}
	return false
}

func findPrevInScope(wp *model.Window, scope *bufferSearchScope, pattern []byte) bool {
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
		wp = model.State.CurrentWindow
		if wp != nil && findPrevPlain(wp, pattern) {
			return true
		}
	}
	return false
}

func isearchHighlightPlain(wp *model.Window, patLen int, backward bool) {
	if wp == nil || patLen <= 0 {
		return
	}
	cursor := wp.Cursor
	var mark buffer.Location
	if backward {
		mark = cursor.AdvanceBytes(wp.Buffer, patLen)
	} else {
		mark = cursor.RewindBytes(wp.Buffer, patLen)
	}
	wp.Mark.Line = mark.Line
	wp.Mark.Offset = mark.Offset
	wp.ShouldRedraw = true
}

func isearchHighlightMatch(wp *model.Window, start, end buffer.Location, backward bool) {
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
	model.State.SearchPattern = truncatePattern(pattern)
	updateSearchCase(model.State.SearchPattern)
}

func isearchRunPlain(wp *model.Window, scope *bufferSearchScope, start *ISearchSnapshot, pattern []byte, backward bool, out *ISearchSnapshot) bool {
	if wp == nil || scope == nil || start == nil || out == nil {
		return false
	}
	restoreSearchSnapshot(wp, start)
	wp = model.State.CurrentWindow
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
	wp = model.State.CurrentWindow
	isearchHighlightPlain(wp, len(pattern), backward)
	*out = saveSearchSnapshot(wp, len(pattern))
	return true
}

func locationCompare(a, b buffer.Location) int {
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

func bufferSliceFrom(bp *buffer.Buffer, start buffer.Location) []byte {
	if bp == nil || start.Line == 0 {
		return nil
	}
	return bp.GetText(start, bufferSearchEnd(bp))
}

func findNextRegexMatchFrom(bp *buffer.Buffer, searchStart buffer.Location, pattern string) (RegexMatch, int) {
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
		Start: searchStart.AdvanceBytes(bp, loc[0]),
		End:   searchStart.AdvanceBytes(bp, loc[1]),
	}
	return match, 1
}

func findPrevRegexMatchFrom(bp *buffer.Buffer, limit buffer.Location, pattern string) (RegexMatch, int) {
	scan := buffer.Location{Line: 1, Offset: 0}
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
		scan = candidate.Start.AdvanceBytes(bp, 1)
	}
	if !haveBest {
		return RegexMatch{}, 0
	}
	return best, 1
}

func findNextRegexInScope(wp *model.Window, scope *bufferSearchScope, pattern string) (RegexMatch, int) {
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
		wp = model.State.CurrentWindow
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

func findPrevRegexInScope(wp *model.Window, scope *bufferSearchScope, pattern string) (RegexMatch, int) {
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
		wp = model.State.CurrentWindow
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

func isearchRunRegex(wp *model.Window, scope *bufferSearchScope, start *ISearchSnapshot, pattern string, backward bool, out *ISearchSnapshot) bool {
	if wp == nil || scope == nil || start == nil || out == nil {
		return false
	}
	restoreSearchSnapshot(wp, start)
	wp = model.State.CurrentWindow
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
	wp = model.State.CurrentWindow
	if backward {
		wp.SetCursor(match.Start)
	} else {
		wp.SetCursor(match.End)
	}
	wp.DidMove = true
	isearchHighlightMatch(wp, match.Start, match.End, backward)
	*out = saveSearchSnapshot(wp, len(pattern))
	return true
}

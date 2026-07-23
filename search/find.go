package search

import (
	"cmp"
	"regexp"
	"unicode"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
)

func searchScopeIsAllBuffers() bool {
	return currentState().SearchScopeSetting == SearchScopeAllBuffers
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
	return []byte(currentState().SearchPattern)
}

func readPattern(label string, onDone func(pr minibuffer.PromptResult)) {
	display := buildSearchPrompt(label)
	askString(display, currentState().SearchPattern, func(pattern string, pr minibuffer.PromptResult) {
		if pr == minibuffer.PromptResultYes {
			currentState().SearchPattern = truncatePattern(pattern)
		} else if pr == minibuffer.PromptResultNo && currentState().SearchPattern != "" {
			pr = minibuffer.PromptResultYes
		}
		if pr == minibuffer.PromptResultYes {
			updateSearchCase(currentState().SearchPattern)
		}
		if onDone != nil {
			onDone(pr)
		}
	})
}

func searchScopeInit(origin *buffer.Buffer) bufferSearchScope {
	scope := bufferSearchScope{allBuffers: searchScopeIsAllBuffers()}
	scope.buffers = append(scope.buffers, origin)
	for i := 0; i < len(buffer.All.Buffers); i++ {
		buf := buffer.All.Buffers[i]
		if buf != nil && buf != origin {
			scope.buffers = append(scope.buffers, buf)
		}
	}
	return scope
}

func searchScopeIndex(scope *bufferSearchScope, buf *buffer.Buffer) int {
	for i, b := range scope.buffers {
		if b == buf {
			return i
		}
	}
	return 0
}

// saveSearchSnapshot captures the current cursor and mark position from a window.
func saveSearchSnapshot(win *window.Window, patternLen int) ISearchSnapshot {
	s := ISearchSnapshot{Buffer: win.Buffer, PatternLen: patternLen}
	if win.Buffer != nil {
		s.Line = win.Cursor.Line
		s.Offset = win.Cursor.Offset
		s.MarkLine = win.Mark.Line
		s.MarkOff = win.Mark.Offset
	}
	return s
}

// restoreSearchSnapshot restores a previously saved cursor and mark position.
func restoreSearchSnapshot(win *window.Window, snap *ISearchSnapshot) {
	if buffer.All.Current != snap.Buffer {
		window.SwitchBuffer(snap.Buffer)
		win = window.Active.CurrentWindow
	}
	if win == nil {
		return
	}
	win.SetCursor(buffer.Location{Line: snap.Line, Offset: snap.Offset})
	win.Mark.Line = snap.MarkLine
	win.Mark.Offset = snap.MarkOff
	win.DidMove = true
}

func isearchClearHighlight(win *window.Window) {
	if win.Mark.Line != 0 {
		win.ShouldRedraw = true
	}
	win.Mark.Line = 0
	win.Mark.Offset = 0
}

// searchSwitchBuffer switches to a buffer and moves the cursor to a specific location.
func searchSwitchBuffer(win *window.Window, buf *buffer.Buffer, loc buffer.Location) {
	if buffer.All.Current != buf {
		window.SwitchBuffer(buf)
		win = window.Active.CurrentWindow
	}
	if win != nil {
		win.SetCursor(loc)
		win.DidMove = true
	}
}

// bufferSearchStart returns the start location of a buffer.
func bufferSearchStart(buf *buffer.Buffer) buffer.Location {
	return buffer.Location{Line: 1, Offset: 0}
}

// bufferSearchEnd returns the end location of a buffer.
func bufferSearchEnd(buf *buffer.Buffer) buffer.Location {
	return buffer.Location{Line: buf.EOF(), Offset: 0}
}

func searchCharsEqual(bc, pc int) bool {
	if currentState().SearchCaseSensitive {
		return bc == pc
	}
	return unicode.ToUpper(rune(bc)) == unicode.ToUpper(rune(pc))
}

func searchReadForward(buf *buffer.Buffer, lineNum *int, offset *int) (int, bool) {
	if *lineNum > len(buf.Lines) {
		return -1, false
	}
	line := buf.Line(*lineNum)
	if line == nil {
		return -1, false
	}
	if *offset >= line.Len() {
		*lineNum++
		*offset = 0
		return '\n', true
	}
	c := int(line.Data[*offset])
	*offset++
	return c, true
}

func searchReadBackward(buf *buffer.Buffer, lineNum *int, offset *int) (int, bool) {
	if *offset == 0 {
		if *lineNum <= 1 {
			return -1, false
		}
		*lineNum--
		line := buf.Line(*lineNum)
		if line == nil {
			return -1, false
		}
		*offset = line.Len() + 1
	}
	*offset--
	line := buf.Line(*lineNum)
	if line == nil {
		return -1, false
	}
	if *offset == line.Len() {
		return '\n', true
	}
	return int(line.Data[*offset]), true
}

// findNextPlain searches forward for a plain text pattern starting from the current cursor position.
func findNextPlain(win *window.Window, pattern []byte) bool {
	buf := win.Buffer
	cline := win.Cursor.Line
	cbo := win.Cursor.Offset

	for cline <= len(buf.Lines) {
		c, ok := searchReadForward(buf, &cline, &cbo)
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
			if tline > len(buf.Lines) {
				matched = false
				break
			}
			nc, ok := searchReadForward(buf, &tline, &tbo)
			if !ok || !searchCharsEqual(nc, int(pattern[i])) {
				matched = false
				break
			}
		}
		if matched {
			win.SetCursor(buffer.Location{Line: tline, Offset: tbo})
			win.DidMove = true
			return true
		}
	}
	return false
}

// findPrevPlain searches backward for a plain text pattern starting from the current cursor position.
func findPrevPlain(win *window.Window, pattern []byte) bool {
	buf := win.Buffer
	last := len(pattern) - 1
	cline := win.Cursor.Line
	cbo := win.Cursor.Offset

	for {
		c, ok := searchReadBackward(buf, &cline, &cbo)
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
			nc, ok := searchReadBackward(buf, &tline, &tbo)
			if !ok || !searchCharsEqual(nc, int(pattern[i])) {
				matched = false
				break
			}
		}
		if matched {
			win.SetCursor(buffer.Location{Line: tline, Offset: tbo})
			win.DidMove = true
			return true
		}
	}
}

// findNextInScope searches forward for a plain text pattern across a buffer search scope.
func findNextInScope(win *window.Window, scope *bufferSearchScope, pattern []byte) bool {
	origin := win.Buffer
	idx := searchScopeIndex(scope, origin)
	if findNextPlain(win, pattern) {
		return true
	}
	if !scope.allBuffers {
		return false
	}
	for step := 1; step < len(scope.buffers); step++ {
		buf := scope.buffers[(idx+step)%len(scope.buffers)]
		searchSwitchBuffer(win, buf, bufferSearchStart(buf))
		win = window.Active.CurrentWindow
		if win != nil && findNextPlain(win, pattern) {
			return true
		}
	}
	return false
}

// findPrevInScope searches backward for a plain text pattern across a buffer search scope.
func findPrevInScope(win *window.Window, scope *bufferSearchScope, pattern []byte) bool {
	origin := win.Buffer
	idx := searchScopeIndex(scope, origin)
	if findPrevPlain(win, pattern) {
		return true
	}
	if !scope.allBuffers {
		return false
	}
	for step := 1; step < len(scope.buffers); step++ {
		buf := scope.buffers[(idx+step)%len(scope.buffers)]
		searchSwitchBuffer(win, buf, bufferSearchEnd(buf))
		win = window.Active.CurrentWindow
		if win != nil && findPrevPlain(win, pattern) {
			return true
		}
	}
	return false
}

// isearchHighlightPlain sets the highlight mark for a plain text incremental search.
func isearchHighlightPlain(win *window.Window, patLen int, backward bool) {
	cursor := win.Cursor
	var mark buffer.Location
	if backward {
		mark = cursor.AdvanceBytes(win.Buffer, patLen)
	} else {
		mark = cursor.RewindBytes(win.Buffer, patLen)
	}
	win.Mark.Line = mark.Line
	win.Mark.Offset = mark.Offset
	win.ShouldRedraw = true
}

// isearchHighlightMatch sets the highlight mark for a regex incremental search match.
func isearchHighlightMatch(win *window.Window, start, end buffer.Location, backward bool) {
	if backward {
		win.Mark = end
	} else {
		win.Mark = start
	}
	win.ShouldRedraw = true
}

// isearchSetPlainPattern sets the incremental search pattern for plain text mode.
func isearchSetPlainPattern(pattern string) {
	currentState().SearchPattern = truncatePattern(pattern)
	updateSearchCase(currentState().SearchPattern)
}

// isearchRunPlain runs a plain text incremental search.
func isearchRunPlain(win *window.Window, scope *bufferSearchScope, start *ISearchSnapshot, pattern []byte, backward bool, out *ISearchSnapshot) bool {
	restoreSearchSnapshot(win, start)
	win = window.Active.CurrentWindow
	if len(pattern) == 0 {
		*out = saveSearchSnapshot(win, 0)
		return true
	}
	isearchSetPlainPattern(string(pattern))
	ok := false
	if backward {
		ok = findPrevInScope(win, scope, pattern)
	} else {
		ok = findNextInScope(win, scope, pattern)
	}
	if !ok {
		return false
	}
	win = window.Active.CurrentWindow
	isearchHighlightPlain(win, len(pattern), backward)
	*out = saveSearchSnapshot(win, len(pattern))
	return true
}

func locationCompare(a, b buffer.Location) int {
	if c := cmp.Compare(a.Line, b.Line); c != 0 {
		return c
	}
	return cmp.Compare(a.Offset, b.Offset)
}

func bufferSliceFrom(buf *buffer.Buffer, start buffer.Location) []byte {
	return buf.GetText(start, bufferSearchEnd(buf))
}

// findNextRegexMatchFrom searches forward from a location for the next regex match.
func findNextRegexMatchFrom(buf *buffer.Buffer, searchStart buffer.Location, pattern string) (RegexMatch, int) {
	text := bufferSliceFrom(buf, searchStart)
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
		Start: searchStart.AdvanceBytes(buf, loc[0]),
		End:   searchStart.AdvanceBytes(buf, loc[1]),
	}
	return match, 1
}

// findPrevRegexMatchFrom searches backward from the cursor for the last regex match before a limit.
func findPrevRegexMatchFrom(buf *buffer.Buffer, limit buffer.Location, pattern string) (RegexMatch, int) {
	scan := buffer.Location{Line: 1, Offset: 0}
	var best RegexMatch
	haveBest := false
	for {
		candidate, found := findNextRegexMatchFrom(buf, scan, pattern)
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
		scan = candidate.Start.AdvanceBytes(buf, 1)
	}
	if !haveBest {
		return RegexMatch{}, 0
	}
	return best, 1
}

// findNextRegexInScope searches forward for the next regex match across a buffer search scope.
func findNextRegexInScope(win *window.Window, scope *bufferSearchScope, pattern string) (RegexMatch, int) {
	origin := win.Buffer
	idx := searchScopeIndex(scope, origin)
	match, found := findNextRegexMatchFrom(win.Buffer, win.Cursor, pattern)
	if found != 0 || !scope.allBuffers {
		return match, found
	}
	for step := 1; step < len(scope.buffers); step++ {
		buf := scope.buffers[(idx+step)%len(scope.buffers)]
		searchSwitchBuffer(win, buf, bufferSearchStart(buf))
		win = window.Active.CurrentWindow
		if win == nil {
			continue
		}
		match, found = findNextRegexMatchFrom(buf, win.Cursor, pattern)
		if found != 0 {
			return match, found
		}
	}
	return RegexMatch{}, 0
}

// findPrevRegexInScope searches backward for the last regex match across a buffer search scope.
func findPrevRegexInScope(win *window.Window, scope *bufferSearchScope, pattern string) (RegexMatch, int) {
	origin := win.Buffer
	idx := searchScopeIndex(scope, origin)
	match, found := findPrevRegexMatchFrom(win.Buffer, win.Cursor, pattern)
	if found != 0 || !scope.allBuffers {
		return match, found
	}
	for step := 1; step < len(scope.buffers); step++ {
		buf := scope.buffers[(idx+step)%len(scope.buffers)]
		searchSwitchBuffer(win, buf, bufferSearchEnd(buf))
		win = window.Active.CurrentWindow
		if win == nil {
			continue
		}
		match, found = findPrevRegexMatchFrom(buf, win.Cursor, pattern)
		if found != 0 {
			return match, found
		}
	}
	return RegexMatch{}, 0
}

// isearchRunRegex runs a regex incremental search.
func isearchRunRegex(win *window.Window, scope *bufferSearchScope, start *ISearchSnapshot, pattern string, backward bool, out *ISearchSnapshot) bool {
	restoreSearchSnapshot(win, start)
	win = window.Active.CurrentWindow
	if pattern == "" {
		*out = saveSearchSnapshot(win, 0)
		return true
	}
	var match RegexMatch
	var found int
	if backward {
		match, found = findPrevRegexInScope(win, scope, pattern)
	} else {
		match, found = findNextRegexInScope(win, scope, pattern)
	}
	if found < 0 {
		isearchClearHighlight(win)
		return false
	}
	if found == 0 {
		isearchClearHighlight(win)
		return false
	}
	win = window.Active.CurrentWindow
	if backward {
		win.SetCursor(match.Start)
	} else {
		win.SetCursor(match.End)
	}
	win.DidMove = true
	isearchHighlightMatch(win, match.Start, match.End, backward)
	*out = saveSearchSnapshot(win, len(pattern))
	return true
}

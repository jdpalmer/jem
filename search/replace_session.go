package search

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
)

type queryReplaceSession struct {
	buf           *buffer.Buffer
	pat          []byte
	patLen       int
	replBytes    []byte
	repl         string
	preserveCase bool
	regex        bool
	pattern      string // regex pattern
	replStr      string // regex replacement template
	scope        bufferSearchScope
	nReplaced    int
	replaceAll   bool
	mbState      minibuffer.MinibufferState
	awaitingKey  bool
	pendingMatch RegexMatch
	hasPending   bool
}

func newQueryReplaceSession(buf *buffer.Buffer, replBytes, pat []byte, patLen int, preserveCase bool) *queryReplaceSession {
	return &queryReplaceSession{
		buf:           buf,
		pat:          pat,
		patLen:       patLen,
		replBytes:    replBytes,
		repl:         string(replBytes),
		preserveCase: preserveCase,
		scope:        searchScopeInit(buf),
	}
}

func newQueryReReplaceSession(buf *buffer.Buffer, pattern, replStr string) *queryReplaceSession {
	return &queryReplaceSession{
		buf:      buf,
		regex:   true,
		pattern: pattern,
		replStr: replStr,
		scope:   searchScopeInit(buf),
	}
}

// Open initializes the session and advances to the next match.
func (s *queryReplaceSession) Open() (done bool) {
	transientSet(queryReplaceBindings)
	minibuffer.Active = &s.mbState
	displayUpdate()
	return s.advance()
}

// Close cleans up the session and resets the window mark.
func (s *queryReplaceSession) Close() {
	transientClear()
	minibuffer.Active = nil
	if win := window.Active.CurrentWindow; win != nil {
		win.Mark.Line = 0
		win.ShouldRedraw = true
	}
}

func (s *queryReplaceSession) finishMessage() {
	suffix := "s"
	if s.nReplaced == 1 {
		suffix = ""
	}
	mbWrite("[replaced %d occurrence%s]", s.nReplaced, suffix)
}

func (s *queryReplaceSession) advance() (done bool) {
	for {
		win := window.Active.CurrentWindow
		if s.regex {
			match, found := findNextRegexInScope(win, &s.scope, s.pattern)
			if found < 0 {
				return true
			}
			if found == 0 {
				s.finishMessage()
				return true
			}
			expanded, err := expandRegexReplacement(s.replStr, match)
			if err != nil {
				return true
			}
			if s.replaceAll {
				if !doReplaceRange(win, match.Start, match.End, expanded) {
					return true
				}
				s.nReplaced++
				continue
			}
			s.pendingMatch = match
			s.hasPending = true
			s.replBytes = expanded
			matchText := string(match.Text[match.Index[0]:match.Index[1]])
			markMatchLocation(win, match.Start)
			win.ShouldRedraw = true
			displayUpdate()
			writeReplacePrompt(s.buf, matchText, string(expanded))
			s.awaitingKey = true
			return false
		}

		if !findNextInScope(win, &s.scope, s.pat) {
			s.finishMessage()
			return true
		}
		if s.replaceAll {
			if !doReplacePreservingCase(win, s.patLen, s.replBytes, s.preserveCase) {
				return true
			}
			s.nReplaced++
			continue
		}
		markMatchStart(win, s.patLen)
		win.ShouldRedraw = true
		displayUpdate()
		writeReplacePrompt(s.buf, string(s.pat), s.repl)
		s.awaitingKey = true
		return false
	}
}

// HandleKey dispatches a keypress to perform a replace action.
func (s *queryReplaceSession) HandleKey(k uint32) (done bool) {
	action := transientLookup(k, replaceActionNone)
	if action == replaceActionNone {
		doBeep()
		return false
	}

	win := window.Active.CurrentWindow
	switch action {
	case replaceActionYes:
		if !s.applyCurrent(win) {
			return true
		}
		s.nReplaced++
	case replaceActionYesQuit:
		if !s.applyCurrent(win) {
			return true
		}
		s.nReplaced++
		suffix := "s"
		if s.nReplaced == 1 {
			suffix = ""
		}
		mbWrite("Replaced %d occurrence%s", s.nReplaced, suffix)
		return true
	case replaceActionNo:
		// skip
	case replaceActionAll:
		if !s.applyCurrent(win) {
			return true
		}
		s.nReplaced++
		s.replaceAll = true
	case replaceActionQuit:
		suffix := "s"
		if s.nReplaced == 1 {
			suffix = ""
		}
		mbWrite("Replaced %d occurrence%s", s.nReplaced, suffix)
		return true
	}
	if win != nil {
		win.Mark.Line = 0
		win.ShouldRedraw = true
	}
	s.awaitingKey = false
	s.hasPending = false
	return s.advance()
}

func (s *queryReplaceSession) applyCurrent(win *window.Window) bool {
	if s.regex {
		if !s.hasPending {
			return false
		}
		m := s.pendingMatch
		return doReplaceRange(win, m.Start, m.End, s.replBytes)
	}
	return doReplacePreservingCase(win, s.patLen, s.replBytes, s.preserveCase)
}

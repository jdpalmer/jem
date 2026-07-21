package search

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
)

type queryReplaceSession struct {
	bp           *buffer.Buffer
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

func newQueryReplaceSession(bp *buffer.Buffer, replBytes, pat []byte, patLen int, preserveCase bool) *queryReplaceSession {
	return &queryReplaceSession{
		bp:           bp,
		pat:          pat,
		patLen:       patLen,
		replBytes:    replBytes,
		repl:         string(replBytes),
		preserveCase: preserveCase,
		scope:        searchScopeInit(bp),
	}
}

func newQueryReReplaceSession(bp *buffer.Buffer, pattern, replStr string) *queryReplaceSession {
	return &queryReplaceSession{
		bp:      bp,
		regex:   true,
		pattern: pattern,
		replStr: replStr,
		scope:   searchScopeInit(bp),
	}
}

func (s *queryReplaceSession) Open() (done bool) {
	transientSet(queryReplaceBindings)
	minibuffer.Active = &s.mbState
	displayUpdate()
	return s.advance()
}

func (s *queryReplaceSession) Close() {
	transientClear()
	minibuffer.Active = nil
	if wp := window.Active.CurrentWindow; wp != nil {
		wp.Mark.Line = 0
		wp.ShouldRedraw = true
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
		wp := window.Active.CurrentWindow
		if s.regex {
			match, found := findNextRegexInScope(wp, &s.scope, s.pattern)
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
				if !doReplaceRange(wp, match.Start, match.End, expanded) {
					return true
				}
				s.nReplaced++
				continue
			}
			s.pendingMatch = match
			s.hasPending = true
			s.replBytes = expanded
			matchText := string(match.Text[match.Index[0]:match.Index[1]])
			markMatchLocation(wp, match.Start)
			wp.ShouldRedraw = true
			displayUpdate()
			writeReplacePrompt(s.bp, matchText, string(expanded))
			s.awaitingKey = true
			return false
		}

		if !findNextInScope(wp, &s.scope, s.pat) {
			s.finishMessage()
			return true
		}
		if s.replaceAll {
			if !doReplacePreservingCase(wp, s.patLen, s.replBytes, s.preserveCase) {
				return true
			}
			s.nReplaced++
			continue
		}
		markMatchStart(wp, s.patLen)
		wp.ShouldRedraw = true
		displayUpdate()
		writeReplacePrompt(s.bp, string(s.pat), s.repl)
		s.awaitingKey = true
		return false
	}
}

func (s *queryReplaceSession) HandleKey(k uint32) (done bool) {
	action := transientLookup(k, replaceActionNone)
	if action == replaceActionNone {
		doBeep()
		return false
	}

	wp := window.Active.CurrentWindow
	switch action {
	case replaceActionYes:
		if !s.applyCurrent(wp) {
			return true
		}
		s.nReplaced++
	case replaceActionYesQuit:
		if !s.applyCurrent(wp) {
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
		if !s.applyCurrent(wp) {
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
	if wp != nil {
		wp.Mark.Line = 0
		wp.ShouldRedraw = true
	}
	s.awaitingKey = false
	s.hasPending = false
	return s.advance()
}

func (s *queryReplaceSession) applyCurrent(wp *window.Window) bool {
	if s.regex {
		if !s.hasPending {
			return false
		}
		m := s.pendingMatch
		return doReplaceRange(wp, m.Start, m.End, s.replBytes)
	}
	return doReplacePreservingCase(wp, s.patLen, s.replBytes, s.preserveCase)
}

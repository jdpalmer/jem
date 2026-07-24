package search

import (
	"bytes"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/window"
)

type isearchSession struct {
	backward      bool
	regex         bool
	win           *window.Window
	scope         bufferSearchScope
	origin        ISearchSnapshot
	lastSuccess   ISearchSnapshot
	pat           [display.PatternCapacity]byte
	savedEdit     [display.PatternCapacity]byte
	cpos          int
	historyPos    int
	haveSavedEdit bool
	failing       bool
	repeatKey     uint32
	label         string
	mbState       minibuffer.MinibufferState
}

func newISearchSession(backward, regex bool) *isearchSession {
	s := &isearchSession{
		backward:   backward,
		regex:      regex,
		historyPos: -1,
		repeatKey:  term.CTL | 'S',
	}
	if backward {
		s.repeatKey = term.CTL | 'R'
	}
	if regex {
		s.label = "RE isearch forward"
		if backward {
			s.label = "RE isearch backward"
		}
	} else {
		s.label = "isearch forward"
		if backward {
			s.label = "isearch backward"
		}
	}
	return s
}

func (s *isearchSession) Open() (done bool) {
	s.win = window.Active.CurrentWindow
	buf := buffer.All.Current
	s.scope = searchScopeInit(buf)
	if !s.regex {
		markring.PushCurrent()
	}
	s.origin = saveSearchSnapshot(s.win, 0)
	s.lastSuccess = s.origin
	minibuffer.Active = &s.mbState
	display.Active.ShowPhantomCursor = true
	s.redraw()
	return false
}

func (s *isearchSession) Close() {
	display.Active.ShowPhantomCursor = false
	isearchClearHighlight(window.Active.CurrentWindow)
	minibuffer.Active = nil
}

func (s *isearchSession) redraw() {
	display.DisplayUpdate()
	buf := (*buffer.Buffer)(nil)
	if s.win != nil {
		buf = s.win.Buffer
	}
	writeISearchPrompt(s.label, s.pat[:], s.cpos, s.failing, buf)
}

func (s *isearchSession) plen() int {
	n := bytes.IndexByte(s.pat[:], 0)
	if n < 0 {
		return len(s.pat)
	}
	return n
}

func (s *isearchSession) HandleKey(k uint32) (done bool) {
	plen := s.plen()
	if k == (term.CTL|'G') || k == 0x1B {
		restoreSearchSnapshot(s.win, &s.origin)
		display.MBWrite("[cancelled]")
		return true
	}
	if k == term.KeyEnter || k == '\r' || k == '\n' || k == (term.CTL|'M') || k == (term.CTL|'J') {
		s.commitPattern(plen)
		display.MBClear()
		return true
	}
	if k == s.repeatKey {
		s.handleRepeat(plen)
		s.redraw()
		return false
	}

	oldPat := string(s.pat[:plen])
	edit := minibuffer.EditKeyHistory(s.pat[:], &s.cpos, display.PatternCapacity, &s.historyPos, &s.haveSavedEdit, s.savedEdit[:], k)
	if edit == minibuffer.MinibufEditUnhandled {
		s.commitPattern(plen)
		display.MBClear()
		return true
	}
	if edit == minibuffer.MinibufEditNoChange {
		term.Beep()
		return false
	}
	plen = s.plen()
	if string(s.pat[:plen]) == oldPat {
		s.redraw()
		return false
	}
	if plen == 0 {
		restoreSearchSnapshot(s.win, &s.origin)
		s.lastSuccess = s.origin
		s.failing = false
		s.redraw()
		return false
	}
	var next ISearchSnapshot
	s.win = window.Active.CurrentWindow
	ok := false
	if s.regex {
		ok = isearchRunRegex(s.win, &s.scope, &s.origin, string(s.pat[:plen]), s.backward, &next)
	} else {
		ok = isearchRunPlain(s.win, &s.scope, &s.origin, s.pat[:plen], s.backward, &next)
	}
	if ok {
		s.lastSuccess = next
		s.failing = false
	} else {
		restoreSearchSnapshot(s.win, &s.lastSuccess)
		s.failing = true
	}
	s.redraw()
	return false
}

func (s *isearchSession) commitPattern(plen int) {
	if plen <= 0 {
		return
	}
	text := string(s.pat[:plen])
	if s.regex {
		DefaultState.RegexSearchPattern = text
	} else {
		isearchSetPlainPattern(text)
	}
	display.MBHistoryAdd(text)
}

func (s *isearchSession) handleRepeat(plen int) {
	if plen == 0 {
		if s.regex {
			patStr := DefaultState.RegexSearchPattern
			if patStr != "" {
				n := len(patStr)
				if n >= len(s.pat) {
					n = len(s.pat) - 1
				}
				copy(s.pat[:], patStr[:n])
				s.pat[n] = 0
				s.cpos = n
				plen = n
			}
		} else {
			old := searchPatternBytes()
			if len(old) > 0 {
				n := len(old)
				if n >= len(s.pat) {
					n = len(s.pat) - 1
				}
				copy(s.pat[:], old[:n])
				s.pat[n] = 0
				s.cpos = n
				plen = n
			}
		}
	}
	if plen == 0 {
		return
	}
	var next ISearchSnapshot
	s.win = window.Active.CurrentWindow
	ok := false
	if s.regex {
		ok = isearchRunRegex(s.win, &s.scope, &s.lastSuccess, string(s.pat[:plen]), s.backward, &next)
	} else {
		ok = isearchRunPlain(s.win, &s.scope, &s.lastSuccess, s.pat[:plen], s.backward, &next)
	}
	if ok {
		s.lastSuccess = next
		s.failing = false
	} else {
		restoreSearchSnapshot(s.win, &s.lastSuccess)
		s.failing = true
	}
}

// IsearchForward starts incremental search forward.
func IsearchForward() KeySession {
	return newISearchSession(false, false)
}

// IsearchBackward starts incremental search backward.
func IsearchBackward() KeySession {
	return newISearchSession(true, false)
}

// IsearchReForward starts regex incremental search forward.
func IsearchReForward() KeySession {
	return newISearchSession(false, true)
}

// IsearchReBackward starts regex incremental search backward.
func IsearchReBackward() KeySession {
	return newISearchSession(true, true)
}

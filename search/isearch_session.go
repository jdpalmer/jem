package search

import (
	"bytes"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/term"
)

type isearchSession struct {
	backward      bool
	regex         bool
	wp            *window.Window
	scope         bufferSearchScope
	origin        ISearchSnapshot
	lastSuccess   ISearchSnapshot
	pat           [display.PatternCapacity]byte
	savedEdit     [display.PatternCapacity]byte
	cpos          int
	historyPos    int16
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
	s.wp = window.Active.CurrentWindow
	bp := buffer.All.Current
	if s.wp == nil || bp == nil {
		return true
	}
	s.scope = searchScopeInit(bp)
	if !s.regex {
		markPushCurrent()
	}
	s.origin = saveSearchSnapshot(s.wp, 0)
	s.lastSuccess = s.origin
	display.ShowMinibuffer(&s.mbState)
	display.Active.ShowPhantomCursor = true
	s.redraw()
	return false
}

func (s *isearchSession) Close() {
	display.Active.ShowPhantomCursor = false
	isearchClearHighlight(window.Active.CurrentWindow)
	display.HideMinibuffer()
}

func (s *isearchSession) redraw() {
	displayUpdate()
	bp := (*buffer.Buffer)(nil)
	if s.wp != nil {
		bp = s.wp.Buffer
	}
	writeISearchPrompt(s.label, s.pat[:], s.cpos, s.failing, bp)
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
		restoreSearchSnapshot(s.wp, &s.origin)
		mbWrite("[cancelled]")
		return true
	}
	if k == term.KeyEnter || k == '\r' || k == '\n' || k == (term.CTL|'M') || k == (term.CTL|'J') {
		s.commitPattern(plen)
		mbClear()
		return true
	}
	if k == s.repeatKey {
		s.handleRepeat(plen)
		s.redraw()
		return false
	}

	oldPat := string(s.pat[:plen])
	initial := searchPatternBytes()
	if s.regex {
		initial = []byte(currentState().RegexSearchPattern)
	}
	edit := mbEditKeyHistory(s.pat[:], &s.cpos, display.PatternCapacity, initial, &s.historyPos, &s.haveSavedEdit, s.savedEdit[:], k)
	if edit == minibuffer.MinibufEditUnhandled {
		s.commitPattern(plen)
		mbClear()
		return true
	}
	if edit == minibuffer.MinibufEditNoChange {
		doBeep()
		return false
	}
	plen = s.plen()
	if string(s.pat[:plen]) == oldPat {
		s.redraw()
		return false
	}
	if plen == 0 {
		restoreSearchSnapshot(s.wp, &s.origin)
		s.lastSuccess = s.origin
		s.failing = false
		s.redraw()
		return false
	}
	var next ISearchSnapshot
	s.wp = window.Active.CurrentWindow
	ok := false
	if s.regex {
		ok = isearchRunRegex(s.wp, &s.scope, &s.origin, string(s.pat[:plen]), s.backward, &next)
	} else {
		ok = isearchRunPlain(s.wp, &s.scope, &s.origin, s.pat[:plen], s.backward, &next)
	}
	if ok {
		s.lastSuccess = next
		s.failing = false
	} else {
		restoreSearchSnapshot(s.wp, &s.lastSuccess)
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
		currentState().RegexSearchPattern = text
	} else {
		isearchSetPlainPattern(text)
	}
	mbHistoryAdd(text)
}

func (s *isearchSession) handleRepeat(plen int) {
	if plen == 0 {
		if s.regex {
			if currentState().RegexSearchPattern != "" {
				copy(s.pat[:], currentState().RegexSearchPattern)
				s.pat[len(currentState().RegexSearchPattern)] = 0
				s.cpos = len(currentState().RegexSearchPattern)
				plen = s.cpos
			}
		} else {
			old := searchPatternBytes()
			if len(old) > 0 {
				copy(s.pat[:], old)
				s.pat[len(old)] = 0
				s.cpos = len(old)
				plen = len(old)
			}
		}
	}
	if plen == 0 {
		return
	}
	var next ISearchSnapshot
	s.wp = window.Active.CurrentWindow
	ok := false
	if s.regex {
		ok = isearchRunRegex(s.wp, &s.scope, &s.lastSuccess, string(s.pat[:plen]), s.backward, &next)
	} else {
		ok = isearchRunPlain(s.wp, &s.scope, &s.lastSuccess, s.pat[:plen], s.backward, &next)
	}
	if ok {
		s.lastSuccess = next
		s.failing = false
	} else {
		restoreSearchSnapshot(s.wp, &s.lastSuccess)
		s.failing = true
	}
}

// IsearchForward starts incremental search forward (async listener).
func IsearchForward() bool {
	return startISearch(false, false)
}

// IsearchBackward starts incremental search backward (async listener).
func IsearchBackward() bool {
	return startISearch(true, false)
}

// IsearchReForward starts regex incremental search forward.
func IsearchReForward() bool {
	return startISearch(false, true)
}

// ISearchReBackward starts regex incremental search backward.
func IsearchReBackward() bool {
	return startISearch(true, true)
}

func startISearch(backward, regex bool) bool {
	s := newISearchSession(backward, regex)
	return pushKeySession(s)
}

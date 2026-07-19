package search

import (
	"bytes"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/view"
)

type isearchSession struct {
	backward     bool
	regex        bool
	wp           *model.Window
	scope        bufferSearchScope
	origin       ISearchSnapshot
	lastSuccess  ISearchSnapshot
	pat          [model.PatternCapacity]byte
	savedEdit    [model.PatternCapacity]byte
	cpos         int
	historyPos   int16
	haveSavedEdit bool
	failing      bool
	repeatKey    uint32
	label        string
	mbState      model.MinibufferState
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
	s.wp = model.State.CurrentWindow
	bp := model.State.CurrentBuffer
	if s.wp == nil || bp == nil {
		return true
	}
	s.scope = searchScopeInit(bp)
	if !s.regex {
		markPushCurrent()
	}
	s.origin = saveSearchSnapshot(s.wp, 0)
	s.lastSuccess = s.origin
	view.ShowMinibuffer(&s.mbState)
	model.State.ShowPhantomCursor = true
	s.redraw()
	return false
}

func (s *isearchSession) Close() {
	model.State.ShowPhantomCursor = false
	isearchClearHighlight(model.State.CurrentWindow)
	view.HideMinibuffer()
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
	if isPasteRedrawKey(k) {
		s.redraw()
		return false
	}
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
	edit := mbEditKeyHistory(s.pat[:], &s.cpos, model.PatternCapacity, initial, &s.historyPos, &s.haveSavedEdit, s.savedEdit[:], k)
	if edit == model.MinibufEditUnhandled {
		s.commitPattern(plen)
		mbClear()
		return true
	}
	if edit == model.MinibufEditNoChange {
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
	s.wp = model.State.CurrentWindow
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
	s.wp = model.State.CurrentWindow
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
	if pushKeySession(s) {
		return true
	}
	// Fallback: blocking WaitKey loop (tests without editor hooks).
	if s.Open() {
		s.Close()
		return false
	}
	defer s.Close()
	for {
		k, ok := isearchReadKey()
		if !ok {
			return false
		}
		if s.HandleKey(k) {
			return true
		}
	}
}

package search

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/term"
)

type SearchScopeMode int

const (
	SearchScopeBuffer SearchScopeMode = iota
	SearchScopeAllBuffers
)

type State struct {
	SearchCaseSensitive bool
	RegexSearchPattern  string
	SearchScopeSetting  SearchScopeMode
	SearchPattern       string
}

var DefaultState = &State{}

func askString(prompt, initial string, onDone func(string, minibuffer.PromptResult)) {
	if PackageHooks.AskString != nil {
		PackageHooks.AskString(prompt, initial, onDone)
		return
	}
	if onDone != nil {
		onDone("", minibuffer.PromptResultAbort)
	}
}

func setText(buf *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error {
	if PackageHooks.SetText == nil {
		return buffer.ErrNilBuffer
	}
	return PackageHooks.SetText(buf, begin, end, newText, newEndOut)
}

func truncatePattern(s string) string {
	if len(s) >= display.PatternCapacity {
		return s[:display.PatternCapacity-1]
	}
	return s
}

type ISearchSnapshot struct {
	Buffer     *buffer.Buffer
	Line       int
	Offset     int
	MarkLine   int
	MarkOff    int
	PatternLen int
}

type bufferSearchScope struct {
	buffers    []*buffer.Buffer
	allBuffers bool
}

type RegexMatch struct {
	Start buffer.Location
	End   buffer.Location
	Text  []byte
	Index []int
}

type replaceAction int

const (
	replaceActionNone    replaceAction = 0
	replaceActionYes     replaceAction = 1
	replaceActionNo      replaceAction = 2
	replaceActionAll     replaceAction = 3
	replaceActionQuit    replaceAction = 4
	replaceActionYesQuit replaceAction = 5
)

var queryReplaceBindings = []struct {
	Code   uint32
	Action replaceAction
}{
	{'y', replaceActionYes},
	{' ', replaceActionYes},
	{term.KeyEnter, replaceActionYes},
	{'n', replaceActionNo},
	{term.CTL | 'H', replaceActionNo},
	{0x7F, replaceActionNo},
	{'!', replaceActionAll},
	{'+', replaceActionYesQuit},
	{'q', replaceActionQuit},
	{term.CTL | 'G', replaceActionQuit},
	{0x1B, replaceActionQuit},
}

func lookupReplaceAction(code uint32) replaceAction {
	for _, b := range queryReplaceBindings {
		if b.Code == code {
			return b.Action
		}
	}
	return replaceActionNone
}

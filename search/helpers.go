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

func truncatePattern(s string) string {
	if len(s) >= display.PatternCapacity {
		return s[:display.PatternCapacity-1]
	}
	return s
}

// SearchPromptLabel returns the minibuffer prompt for a search entry point.
func SearchPromptLabel(label string) string {
	return buildSearchPrompt(label)
}

// AcceptPromptedPattern updates DefaultState from an AskString reply.
// Returns true when the caller should proceed (Yes, or No with a retained prior pattern).
func AcceptPromptedPattern(pattern string, pr minibuffer.PromptResult) bool {
	if pr == minibuffer.PromptResultYes {
		DefaultState.SearchPattern = truncatePattern(pattern)
	} else if pr == minibuffer.PromptResultNo && DefaultState.SearchPattern != "" {
		// keep existing pattern
	} else {
		return false
	}
	updateSearchCase(DefaultState.SearchPattern)
	return true
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

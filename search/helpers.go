package search

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/term"
)

type SearchScopeMode int

const (
	SearchScopeBuffer SearchScopeMode = iota
	SearchScopeAllBuffers
)

type TransientAction int32

type TransientBinding struct {
	Code   uint32
	Action TransientAction
}

type State struct {
	SearchCaseSensitive bool
	RegexSearchPattern  string
	SearchScopeSetting  SearchScopeMode
	SearchPattern       string
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
	display.MBWrite(format, args...)
}

func mbClear() {
	display.MBClear()
}

func askString(prompt, initial string, onDone func(string, minibuffer.PromptResult)) {
	if PackageHooks.AskString != nil {
		PackageHooks.AskString(prompt, initial, onDone)
		return
	}
	if onDone != nil {
		onDone("", minibuffer.PromptResultAbort)
	}
}

func mbWritePromptStyle(prompt string, text []byte, cpos int, style buffer.TextStyle) {
	display.MBWritePromptStyle(prompt, text, cpos, style)
}

func mbHistoryAdd(text string) {
	display.MBHistoryAdd(text)
}

func displayUpdate() {
	display.DisplayUpdate()
}

func markPushCurrent() {
	markring.PushCurrent()
}

func doBeep() {
	term.Beep()
}

func setText(bp *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error {
	if PackageHooks.SetText == nil {
		return buffer.ErrNilBuffer
	}
	return PackageHooks.SetText(bp, begin, end, newText, newEndOut)
}

func truncatePattern(s string) string {
	if len(s) >= display.PatternCapacity {
		return s[:display.PatternCapacity-1]
	}
	return s
}

type ISearchSnapshot struct {
	Buffer     *buffer.Buffer
	Line       uint
	Offset     uint
	MarkLine   uint
	MarkOff    uint
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
	{term.KeyEnter, TransientAction(replaceActionYes)},
	{'n', TransientAction(replaceActionNo)},
	{term.CTL | 'H', TransientAction(replaceActionNo)},
	{0x7F, TransientAction(replaceActionNo)},
	{'!', TransientAction(replaceActionAll)},
	{'+', TransientAction(replaceActionYesQuit)},
	{'q', TransientAction(replaceActionQuit)},
	{term.CTL | 'G', TransientAction(replaceActionQuit)},
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

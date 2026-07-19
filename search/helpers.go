package search

import (
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/view"
)

type State struct {
	SearchCaseSensitive bool
	RegexSearchPattern  string
	TransientBindings   []model.TransientBinding
}

var DefaultState = &State{}

func currentState() *State {
	if DefaultState == nil {
		DefaultState = &State{}
	}
	return DefaultState
}

func mbWrite(format string, args ...interface{}) {
	view.MBWrite(format, args...)
}

func mbClear() {
	view.MBClear()
}

func askString(prompt, initial string, onDone func(string, model.PromptResult)) {
	view.AskString(prompt, initial, onDone)
}

func mbWritePromptStyle(prompt string, text []byte, cpos int, style buffer.TextStyle) {
	view.MBWritePromptStyle(prompt, text, cpos, style)
}

func mbHistoryAdd(text string) {
	view.MBHistoryAdd(text)
}

func mbEditKeyHistory(buf []byte, cpos *int, nbuf int, initial []byte, historyPos *int16, haveSavedEdit *bool, savedEdit []byte, k uint32) model.MinibufferEditResult {
	return view.MBEditKeyHistory(buf, cpos, nbuf, initial, historyPos, haveSavedEdit, savedEdit, k)
}

func displayUpdate() {
	view.DisplayUpdate()
}

func markPushCurrent() {
	model.MarkPushCurrent()
}

func isearchReadKey() (uint32, bool) {
	return view.WaitKey()
}

func activateMinibuffer(state *model.MinibufferState) {
	view.ActivateMinibuffer(state)
}

func deactivateMinibuffer() {
	view.DeactivateMinibuffer()
}

func isPasteRedrawKey(k uint32) bool {
	return view.IsPasteRedrawKey(k)
}

func doBeep() {
	term.Beep()
}

func setText(bp *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location) error {
	return model.SetText(bp, begin, end, newText, newEndOut)
}

func truncatePattern(s string) string {
	if len(s) >= model.PatternCapacity {
		return s[:model.PatternCapacity-1]
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

type replaceAction model.TransientAction

const (
	replaceActionNone    replaceAction = 0
	replaceActionYes     replaceAction = 1
	replaceActionNo      replaceAction = 2
	replaceActionAll     replaceAction = 3
	replaceActionQuit    replaceAction = 4
	replaceActionYesQuit replaceAction = 5
)

var queryReplaceBindings = []model.TransientBinding{
	{'y', model.TransientAction(replaceActionYes)},
	{' ', model.TransientAction(replaceActionYes)},
	{term.KeyEnter, model.TransientAction(replaceActionYes)},
	{'n', model.TransientAction(replaceActionNo)},
	{term.CTL | 'H', model.TransientAction(replaceActionNo)},
	{0x7F, model.TransientAction(replaceActionNo)},
	{'!', model.TransientAction(replaceActionAll)},
	{'+', model.TransientAction(replaceActionYesQuit)},
	{'q', model.TransientAction(replaceActionQuit)},
	{term.CTL | 'G', model.TransientAction(replaceActionQuit)},
	{0x1B, model.TransientAction(replaceActionQuit)},
}

func transientSet(bindings []model.TransientBinding) {
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

package minibuffer

type PromptResult int

const (
	PromptResultNo PromptResult = iota
	PromptResultYes
	PromptResultAbort
)

type MinibufferEditResult int

const (
	MinibufEditUnhandled MinibufferEditResult = iota
	MinibufEditNoChange
	MinibufEditChanged
)

type MinibufferState struct {
	Text          []byte
	CursorPos     int
	Nbuf          int
	HistoryPos    int
	HaveSavedEdit bool
	SavedEdit     []byte
}

type MLChoiceLabelFn func(ctx any, index int) []byte
type MbNameProviderFn func(ctx any, index int) []byte
type MbMatchFormatter func(out []byte, outSize int, idx int, ctx any)

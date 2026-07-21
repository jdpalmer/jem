package minibuffer

import "github.com/jdpalmer/jem/buffer"

type PromptResult int

const (
	PromptResultNo    PromptResult = 0
	PromptResultYes   PromptResult = 1
	PromptResultAbort PromptResult = 2
)

type MinibufferEditResult int

const (
	MinibufEditUnhandled MinibufferEditResult = 0
	MinibufEditNoChange  MinibufferEditResult = 1
	MinibufEditChanged   MinibufferEditResult = 2
)

type MinibufferState struct {
	Prompt           string
	Text             []byte
	CursorPos        int
	Nbuf             int
	Style            buffer.TextStyle
	HistoryPos       int16
	HaveSavedEdit    bool
	SavedEdit        []byte
	SavedEditNbuf    int
	IsFilename       bool
	IsCommand        bool
	IsFuzzyList      bool
	FuzzyCtx         any
	FuzzyProvider    func(ctx any, index int) []byte
	FuzzyCount       int
	FuzzySelected    int
	DisplayFormatter func(out []byte, outSize int, idx int, ctx any)
	DisplayCtx       any
	MatchCount       int
	MatchSelected    int
}

type MLChoiceLabelFn func(ctx any, index int) []byte
type MbNameProviderFn func(ctx any, index int) []byte
type MbMatchFormatter func(out []byte, outSize int, idx int, ctx any)

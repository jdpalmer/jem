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
	CursorPos        uint
	Nbuf             uint
	Style            buffer.TextStyle
	HistoryPos       int16
	HaveSavedEdit    bool
	SavedEdit        []byte
	SavedEditNbuf    uint
	IsFilename       bool
	IsCommand        bool
	IsFuzzyList      bool
	FuzzyCtx         any
	FuzzyProvider    func(ctx any, index uint) []byte
	FuzzyCount       uint
	FuzzySelected    uint
	DisplayFormatter func(out []byte, outSize uint, idx uint, ctx any)
	DisplayCtx       any
	MatchCount       uint
	MatchSelected    uint
}

type MLChoiceLabelFn func(ctx any, index uint8) []byte
type MbNameProviderFn func(ctx any, index uint) []byte
type MbMatchFormatter func(out []byte, outSize uint, idx uint, ctx any)

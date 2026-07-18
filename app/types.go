package app

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/modesyntax"
)

const (
	Version               = "26.1"
	BufferNameCapacity    = buffer.BufferNameCapacity
	CommandPromptCapacity = 256
	MacroCapacity         = 256
	PatternCapacity       = 256
	MaxBuffers            = 255
	MaxWindows            = 255
	HookCapacity          = 8
)

type CommandState int

const (
	CmdStateNone    CommandState = 0
	CmdStateChained CommandState = 1
	CmdStateCurrent CommandState = 2
)

type SearchScopeMode int

const (
	SearchScopeBuffer     SearchScopeMode = 0
	SearchScopeAllBuffers SearchScopeMode = 1
)

type PromptResult int

const (
	PromptResultNo    PromptResult = 0
	PromptResultYes   PromptResult = 1
	PromptResultAbort PromptResult = 2
)

type ThemeMode int

const (
	ThemeDark  ThemeMode = 0
	ThemeLight ThemeMode = 1
)

type GitLineDiff int

const (
	GitLineDiffNone     GitLineDiff = 0
	GitLineDiffAdded    GitLineDiff = 1
	GitLineDiffModified GitLineDiff = 2
	GitLineDiffDeleted  GitLineDiff = 3
)

type ModeWordAttr int

const (
	ModeWordAttrNone    ModeWordAttr = 0
	ModeWordAttrType    ModeWordAttr = 1
	ModeWordAttrBuiltin ModeWordAttr = 2
)

type ModeMiscIndentKind int

const (
	ModeMiscIndentNone    ModeMiscIndentKind = 0
	ModeMiscIndentMake    ModeMiscIndentKind = 1
	ModeMiscIndentLua     ModeMiscIndentKind = 2
	ModeMiscIndentPascal  ModeMiscIndentKind = 3
	ModeMiscIndentVerilog ModeMiscIndentKind = 4
	ModeMiscIndentR       ModeMiscIndentKind = 5
	ModeMiscIndentHTML    ModeMiscIndentKind = 6
	ModeMiscIndentLisp    ModeMiscIndentKind = 7
)

type HookEvent int

const (
	HookBufferVisit  HookEvent = 0
	HookModeChange   HookEvent = 1
	HookBeforeSave   HookEvent = 2
	HookAfterSave    HookEvent = 3
	HookWindowSwitch HookEvent = 4
	HookBufferCreate HookEvent = 5
	HookBufferKill   HookEvent = 6
	HookSearchJump   HookEvent = 7
	HookEventCount   HookEvent = 8
)

type MinibufferEditResult int

const (
	MinibufEditUnhandled MinibufferEditResult = 0
	MinibufEditNoChange  MinibufferEditResult = 1
	MinibufEditChanged   MinibufferEditResult = 2
)

type ScreenCoord struct {
	Row uint32
	Col uint32
}

type Window struct {
	Buffer               *buffer.Buffer
	TopLine              uint
	Cursor               buffer.Location
	Mark                 buffer.Location
	ScreenTopRow         uint32
	Height               uint32
	ForceReframe         bool
	ShouldReframe        bool
	DidMove              bool
	DidEdit              bool
	ShouldRedraw         bool
	ShouldUpdateModeLine bool
	HScroll              uint32
}

type Region struct {
	Start buffer.Location
	End   buffer.Location
}

type KillRingEntry struct {
	Data []byte
}

type ModeInfo struct {
	Mode              buffer.LangMode
	DisplayName       string
	CompletionName    string
	SyntaxKind        modesyntax.ModeSyntaxKind
	SyntaxFlags       uint32
	MiscIndentKind    ModeMiscIndentKind
	FillColumnDefault uint16
	CommentOpen       string
	CommentAltOpen    string
	CommentAppend     string
	CommentCursorBack uint8
	NewlineAndIndent  func(f bool, n int) bool
	IndentLine        func(f bool, n int) bool
	CloseBrace        func(f bool, n int) bool
	GotoMatch         func(f bool, n int) bool
	MakeComment       func(f bool, n int) bool
	TopOfFunction     func(f bool, n int) bool
	EndOfFunction     func(f bool, n int) bool
	MarkFunction      func(f bool, n int) bool
	Extensions        []string
	ExtensionCount    uint8
	Basenames         []string
	BasenameCount     uint8
}

type ThemeState struct {
	NormalStyle          buffer.TextStyle
	CommentStyle         buffer.TextStyle
	PickerSelectionStyle buffer.TextStyle
	GutterStyle          buffer.TextStyle
	SelectionBg          buffer.TermColor
	ModelineNameColor    buffer.TermColor
	Mode                 ThemeMode
}

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

type EditorRuntimeState struct {
	Mouse              ScreenCoord
	MovementState      CommandState
	KillState          CommandState
	CurrentWindow      *Window
	CurrentBuffer      *buffer.Buffer
	ActiveMinibuffer   *MinibufferState
	Dispatching        bool
	WINDOWS            []*Window
	Buffers            []*buffer.Buffer
	NextBufferSerial   uint32
	SearchScopeSetting SearchScopeMode
	SearchPattern      string
}

type EditorDisplayState struct {
	Cursor             ScreenCoord
	PhantomCursor      ScreenCoord
	GoalCol            uint32
	FillCol            uint32
	Theme              ThemeState
	PhantomText        byte
	MessagePresent     bool
	PhantomCursorValid bool
	ShowPhantomCursor  bool
	ScreenDirty        bool
	PhantomStyle       buffer.TextStyle
	ActiveStyle        buffer.TextStyle
}

type EditorMacroState struct {
	Keys      []int32
	RecordPos int
	PlayPos   int
}

type EditorSettingsState struct {
	WhitespaceCleanup bool
	StartupQuote      bool
	AutoRevertMode    bool
	CIndent           uint32
	CBrace            uint32
	CColonOffset      uint32
	PyIndent          uint32
	PyContinuedOffset uint32
}

type TransientAction int32

type TransientBinding struct {
	Code   uint32
	Action TransientAction
}

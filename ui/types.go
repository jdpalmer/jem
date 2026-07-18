package ui

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
)

const (
	Version               = app.Version
	BufferNameCapacity    = app.BufferNameCapacity
	CommandPromptCapacity = app.CommandPromptCapacity
	MacroCapacity         = app.MacroCapacity
	PatternCapacity       = app.PatternCapacity
	MaxBuffers            = app.MaxBuffers
	MaxWindows            = app.MaxWindows
	HookCapacity          = app.HookCapacity
)

const (
	CmdStateNone    = app.CmdStateNone
	CmdStateChained = app.CmdStateChained
	CmdStateCurrent = app.CmdStateCurrent
)

const (
	SearchScopeBuffer     = app.SearchScopeBuffer
	SearchScopeAllBuffers = app.SearchScopeAllBuffers
)

const (
	PromptResultNo    = app.PromptResultNo
	PromptResultYes   = app.PromptResultYes
	PromptResultAbort = app.PromptResultAbort
)

const (
	FIOSuc = app.FIOSuc
	FIOFNF = app.FIOFNF
	FIOEOF = app.FIOEOF
	FIOErr = app.FIOErr
)

const (
	ThemeDark  = app.ThemeDark
	ThemeLight = app.ThemeLight
)

const (
	GitLineDiffNone     = app.GitLineDiffNone
	GitLineDiffAdded    = app.GitLineDiffAdded
	GitLineDiffModified = app.GitLineDiffModified
	GitLineDiffDeleted  = app.GitLineDiffDeleted
)

const (
	ModeWordAttrNone    = app.ModeWordAttrNone
	ModeWordAttrType    = app.ModeWordAttrType
	ModeWordAttrBuiltin = app.ModeWordAttrBuiltin
)

const (
	ModeSyntaxGeneral         = app.ModeSyntaxGeneral
	ModeSyntaxNone            = app.ModeSyntaxNone
	ModeSyntaxHashCommentOnly = app.ModeSyntaxHashCommentOnly
	ModeSyntaxMarkdown        = app.ModeSyntaxMarkdown
	ModeSyntaxHTML            = app.ModeSyntaxHTML
)

const (
	ModeMiscIndentNone    = app.ModeMiscIndentNone
	ModeMiscIndentMake    = app.ModeMiscIndentMake
	ModeMiscIndentLua     = app.ModeMiscIndentLua
	ModeMiscIndentPascal  = app.ModeMiscIndentPascal
	ModeMiscIndentVerilog = app.ModeMiscIndentVerilog
	ModeMiscIndentR       = app.ModeMiscIndentR
	ModeMiscIndentHTML    = app.ModeMiscIndentHTML
	ModeMiscIndentLisp    = app.ModeMiscIndentLisp
)

const (
	ModeFlagIdentDash          = app.ModeFlagIdentDash
	ModeFlagIdentLispExtra     = app.ModeFlagIdentLispExtra
	ModeFlagIdentLispSigil     = app.ModeFlagIdentLispSigil
	ModeFlagCommentSlashLine   = app.ModeFlagCommentSlashLine
	ModeFlagCommentSlashBlock  = app.ModeFlagCommentSlashBlock
	ModeFlagPreprocHashAtBOL   = app.ModeFlagPreprocHashAtBOL
	ModeFlagCommentHash        = app.ModeFlagCommentHash
	ModeFlagCommentSemi        = app.ModeFlagCommentSemi
	ModeFlagCommentLua         = app.ModeFlagCommentLua
	ModeFlagCommentPascalBrace = app.ModeFlagCommentPascalBrace
	ModeFlagCommentPascalParen = app.ModeFlagCommentPascalParen
	ModeFlagAtRule             = app.ModeFlagAtRule
	ModeFlagNoCurlyRainbow     = app.ModeFlagNoCurlyRainbow
)

const (
	HookBufferVisit  = app.HookBufferVisit
	HookModeChange   = app.HookModeChange
	HookBeforeSave   = app.HookBeforeSave
	HookAfterSave    = app.HookAfterSave
	HookWindowSwitch = app.HookWindowSwitch
	HookBufferCreate = app.HookBufferCreate
	HookBufferKill   = app.HookBufferKill
	HookSearchJump   = app.HookSearchJump
	HookEventCount   = app.HookEventCount
)

const (
	MinibufEditUnhandled = app.MinibufEditUnhandled
	MinibufEditNoChange  = app.MinibufEditNoChange
	MinibufEditChanged   = app.MinibufEditChanged
)

type (
	Buffer               = app.Buffer
	Line                 = app.Line
	Location             = app.Location
	EolMode              = app.EolMode
	LangMode             = app.LangMode
	SynState             = app.SynState
	SyntaxLineSummary    = app.SyntaxLineSummary
	SyntaxContext        = app.SyntaxContext
	SyntaxDelimiterMask  = app.SyntaxDelimiterMask
	SyntaxBlock          = app.SyntaxBlock
	TextStyle            = app.TextStyle
	TermColor            = app.TermColor
	UndoHistory          = app.UndoHistory
	UndoKind             = app.UndoKind
	CommandState         = app.CommandState
	SearchScopeMode      = app.SearchScopeMode
	PromptResult         = app.PromptResult
	FileIoStatus         = app.FileIoStatus
	ThemeMode            = app.ThemeMode
	GitLineDiff          = app.GitLineDiff
	ModeWordAttr         = app.ModeWordAttr
	ModeSyntaxKind       = app.ModeSyntaxKind
	ModeMiscIndentKind   = app.ModeMiscIndentKind
	HookEvent            = app.HookEvent
	MinibufferEditResult = app.MinibufferEditResult
	ScreenCoord          = app.ScreenCoord
	Window               = app.Window
	Region               = app.Region
	KillRingEntry        = app.KillRingEntry
	ModeInfo             = app.ModeInfo
	ThemeState           = app.ThemeState
	MinibufferState      = app.MinibufferState
	MLChoiceLabelFn      = app.MLChoiceLabelFn
	MbNameProviderFn     = app.MbNameProviderFn
	MbMatchFormatter     = app.MbMatchFormatter
	EditorRuntimeState   = app.EditorRuntimeState
	EditorDisplayState   = app.EditorDisplayState
	EditorMacroState     = app.EditorMacroState
	EditorSettingsState  = app.EditorSettingsState
	TransientAction      = app.TransientAction
	TransientBinding     = app.TransientBinding
	MarkRing             = app.MarkRing
)

const (
	EModeLF   = app.EModeLF
	EModeCRLF = app.EModeCRLF
	EModeCR   = app.EModeCR

	SyntaxContextNone    = app.SyntaxContextNone
	SyntaxContextCode    = app.SyntaxContextCode
	SyntaxContextString  = app.SyntaxContextString
	SyntaxContextComment = app.SyntaxContextComment
	SyntaxContextPreproc = app.SyntaxContextPreproc

	SyntaxDelimParen   = app.SyntaxDelimParen
	SyntaxDelimBracket = app.SyntaxDelimBracket
	SyntaxDelimCurly   = app.SyntaxDelimCurly
	SyntaxDelimAll     = app.SyntaxDelimAll

	UndoDelete = app.UndoDelete
	UndoInsert = app.UndoInsert

	TermColorBlack   = app.TermColorBlack
	TermColorRed     = app.TermColorRed
	TermColorGreen   = app.TermColorGreen
	TermColorYellow  = app.TermColorYellow
	TermColorBlue    = app.TermColorBlue
	TermColorMagenta = app.TermColorMagenta
	TermColorCyan    = app.TermColorCyan
	TermColorWhite   = app.TermColorWhite
	TermColorDefault = app.TermColorDefault
	TermColorBase03  = app.TermColorBase03
	TermColorBase02  = app.TermColorBase02
	TermColorBase01  = app.TermColorBase01
	TermColorBase00  = app.TermColorBase00
	TermColorBase0   = app.TermColorBase0
	TermColorBase1   = app.TermColorBase1
	TermColorBase2   = app.TermColorBase2
	TermColorBase3   = app.TermColorBase3

	TextStyleFgShift   = buffer.TextStyleFgShift
	TextStyleBgShift   = buffer.TextStyleBgShift
	TextStyleColorMask = app.TextStyleColorMask
	TextStyleBold      = app.TextStyleBold
	TextStyleUnderline = app.TextStyleUnderline
	TextStyleReverse   = app.TextStyleReverse

	CTL            = app.CTL
	META           = app.META
	CTLX           = app.CTLX
	SHIFT          = app.SHIFT
	KeyMask        = app.KeyMask
	KeyUp          = app.KeyUp
	KeyDown        = app.KeyDown
	KeyLeft        = app.KeyLeft
	KeyRight       = app.KeyRight
	KeyTab         = app.KeyTab
	KeyEnter       = app.KeyEnter
	KeyHome        = app.KeyHome
	KeyEnd         = app.KeyEnd
	KeyPageUp      = app.KeyPageUp
	KeyPageDown    = app.KeyPageDown
	KeyDelete      = app.KeyDelete
	MouseLeft      = app.MouseLeft
	MouseWheelUp   = app.MouseWheelUp
	MouseWheelDown = app.MouseWheelDown
	MouseDrag      = app.MouseDrag
	UnicodeLimit   = app.UnicodeLimit
	LModeNone      = app.LModeNone
)

var (
	TextStyleDefault = buffer.TextStyleDefault
	TextStyleGutter  = buffer.TextStyleGutter
	MakeTextStyle    = buffer.MakeTextStyle
	TextStyleFg      = buffer.TextStyleFg
	TextStyleBg      = buffer.TextStyleBg
)

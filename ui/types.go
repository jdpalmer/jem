package ui

import sess "github.com/jdpalmer/jem/session"

const (
	Version               = sess.Version
	BufferNameCapacity    = sess.BufferNameCapacity
	CommandPromptCapacity = sess.CommandPromptCapacity
	MacroCapacity         = sess.MacroCapacity
	PatternCapacity       = sess.PatternCapacity
	MaxBuffers            = sess.MaxBuffers
	MaxWindows            = sess.MaxWindows
	HookCapacity          = sess.HookCapacity
)

const (
	CmdStateNone    = sess.CmdStateNone
	CmdStateChained = sess.CmdStateChained
	CmdStateCurrent = sess.CmdStateCurrent
)

const (
	SearchScopeBuffer     = sess.SearchScopeBuffer
	SearchScopeAllBuffers = sess.SearchScopeAllBuffers
)

const (
	PromptResultNo    = sess.PromptResultNo
	PromptResultYes   = sess.PromptResultYes
	PromptResultAbort = sess.PromptResultAbort
)

const (
	FIOSuc = sess.FIOSuc
	FIOFNF = sess.FIOFNF
	FIOEOF = sess.FIOEOF
	FIOErr = sess.FIOErr
)

const (
	ThemeDark  = sess.ThemeDark
	ThemeLight = sess.ThemeLight
)

const (
	GitLineDiffNone     = sess.GitLineDiffNone
	GitLineDiffAdded    = sess.GitLineDiffAdded
	GitLineDiffModified = sess.GitLineDiffModified
	GitLineDiffDeleted  = sess.GitLineDiffDeleted
)

const (
	ModeWordAttrNone    = sess.ModeWordAttrNone
	ModeWordAttrType    = sess.ModeWordAttrType
	ModeWordAttrBuiltin = sess.ModeWordAttrBuiltin
)

const (
	ModeSyntaxGeneral         = sess.ModeSyntaxGeneral
	ModeSyntaxNone            = sess.ModeSyntaxNone
	ModeSyntaxHashCommentOnly = sess.ModeSyntaxHashCommentOnly
	ModeSyntaxMarkdown        = sess.ModeSyntaxMarkdown
	ModeSyntaxHTML            = sess.ModeSyntaxHTML
)

const (
	ModeMiscIndentNone    = sess.ModeMiscIndentNone
	ModeMiscIndentMake    = sess.ModeMiscIndentMake
	ModeMiscIndentLua     = sess.ModeMiscIndentLua
	ModeMiscIndentPascal  = sess.ModeMiscIndentPascal
	ModeMiscIndentVerilog = sess.ModeMiscIndentVerilog
	ModeMiscIndentR       = sess.ModeMiscIndentR
	ModeMiscIndentHTML    = sess.ModeMiscIndentHTML
	ModeMiscIndentLisp    = sess.ModeMiscIndentLisp
)

const (
	ModeFlagIdentDash          = sess.ModeFlagIdentDash
	ModeFlagIdentLispExtra     = sess.ModeFlagIdentLispExtra
	ModeFlagIdentLispSigil     = sess.ModeFlagIdentLispSigil
	ModeFlagCommentSlashLine   = sess.ModeFlagCommentSlashLine
	ModeFlagCommentSlashBlock  = sess.ModeFlagCommentSlashBlock
	ModeFlagPreprocHashAtBOL   = sess.ModeFlagPreprocHashAtBOL
	ModeFlagCommentHash        = sess.ModeFlagCommentHash
	ModeFlagCommentSemi        = sess.ModeFlagCommentSemi
	ModeFlagCommentLua         = sess.ModeFlagCommentLua
	ModeFlagCommentPascalBrace = sess.ModeFlagCommentPascalBrace
	ModeFlagCommentPascalParen = sess.ModeFlagCommentPascalParen
	ModeFlagAtRule             = sess.ModeFlagAtRule
	ModeFlagNoCurlyRainbow     = sess.ModeFlagNoCurlyRainbow
)

const (
	HookBufferVisit  = sess.HookBufferVisit
	HookModeChange   = sess.HookModeChange
	HookBeforeSave   = sess.HookBeforeSave
	HookAfterSave    = sess.HookAfterSave
	HookWindowSwitch = sess.HookWindowSwitch
	HookBufferCreate = sess.HookBufferCreate
	HookBufferKill   = sess.HookBufferKill
	HookSearchJump   = sess.HookSearchJump
	HookEventCount   = sess.HookEventCount
)

const (
	MinibufEditUnhandled = sess.MinibufEditUnhandled
	MinibufEditNoChange  = sess.MinibufEditNoChange
	MinibufEditChanged   = sess.MinibufEditChanged
)

type (
	Buffer               = sess.Buffer
	Line                 = sess.Line
	Location             = sess.Location
	EolMode              = sess.EolMode
	LangMode             = sess.LangMode
	SynState             = sess.SynState
	SyntaxLineSummary    = sess.SyntaxLineSummary
	SyntaxContext        = sess.SyntaxContext
	SyntaxDelimiterMask  = sess.SyntaxDelimiterMask
	SyntaxBlock          = sess.SyntaxBlock
	TextStyle            = sess.TextStyle
	TermColor            = sess.TermColor
	UndoHistory          = sess.UndoHistory
	UndoKind             = sess.UndoKind
	CommandState         = sess.CommandState
	SearchScopeMode      = sess.SearchScopeMode
	PromptResult         = sess.PromptResult
	FileIoStatus         = sess.FileIoStatus
	ThemeMode            = sess.ThemeMode
	GitLineDiff          = sess.GitLineDiff
	ModeWordAttr         = sess.ModeWordAttr
	ModeSyntaxKind       = sess.ModeSyntaxKind
	ModeMiscIndentKind   = sess.ModeMiscIndentKind
	HookEvent            = sess.HookEvent
	MinibufferEditResult = sess.MinibufferEditResult
	ScreenCoord          = sess.ScreenCoord
	Window               = sess.Window
	Region               = sess.Region
	KillRingEntry        = sess.KillRingEntry
	ModeInfo             = sess.ModeInfo
	ThemeState           = sess.ThemeState
	MinibufferState      = sess.MinibufferState
	MLChoiceLabelFn      = sess.MLChoiceLabelFn
	MbNameProviderFn     = sess.MbNameProviderFn
	MbMatchFormatter     = sess.MbMatchFormatter
	EditorRuntimeState   = sess.EditorRuntimeState
	EditorDisplayState   = sess.EditorDisplayState
	EditorMacroState     = sess.EditorMacroState
	EditorSettingsState  = sess.EditorSettingsState
	TransientAction      = sess.TransientAction
	TransientBinding     = sess.TransientBinding
	MarkRing             = sess.MarkRing
)

const (
	EModeLF   = sess.EModeLF
	EModeCRLF = sess.EModeCRLF
	EModeCR   = sess.EModeCR

	SyntaxContextNone    = sess.SyntaxContextNone
	SyntaxContextCode    = sess.SyntaxContextCode
	SyntaxContextString  = sess.SyntaxContextString
	SyntaxContextComment = sess.SyntaxContextComment
	SyntaxContextPreproc = sess.SyntaxContextPreproc

	SyntaxDelimParen   = sess.SyntaxDelimParen
	SyntaxDelimBracket = sess.SyntaxDelimBracket
	SyntaxDelimCurly   = sess.SyntaxDelimCurly
	SyntaxDelimAll     = sess.SyntaxDelimAll

	UndoDelete = sess.UndoDelete
	UndoInsert = sess.UndoInsert

	TermColorBlack   = sess.TermColorBlack
	TermColorRed     = sess.TermColorRed
	TermColorGreen   = sess.TermColorGreen
	TermColorYellow  = sess.TermColorYellow
	TermColorBlue    = sess.TermColorBlue
	TermColorMagenta = sess.TermColorMagenta
	TermColorCyan    = sess.TermColorCyan
	TermColorWhite   = sess.TermColorWhite
	TermColorDefault = sess.TermColorDefault
	TermColorBase03  = sess.TermColorBase03
	TermColorBase02  = sess.TermColorBase02
	TermColorBase01  = sess.TermColorBase01
	TermColorBase00  = sess.TermColorBase00
	TermColorBase0   = sess.TermColorBase0
	TermColorBase1   = sess.TermColorBase1
	TermColorBase2   = sess.TermColorBase2
	TermColorBase3   = sess.TermColorBase3

	TextStyleFgShift   = sess.TextStyleFgShift
	TextStyleBgShift   = sess.TextStyleBgShift
	TextStyleColorMask = sess.TextStyleColorMask
	TextStyleBold      = sess.TextStyleBold
	TextStyleUnderline = sess.TextStyleUnderline
	TextStyleReverse   = sess.TextStyleReverse

	CTL            = sess.CTL
	META           = sess.META
	CTLX           = sess.CTLX
	SHIFT          = sess.SHIFT
	KeyMask        = sess.KeyMask
	KeyUp          = sess.KeyUp
	KeyDown        = sess.KeyDown
	KeyLeft        = sess.KeyLeft
	KeyRight       = sess.KeyRight
	KeyTab         = sess.KeyTab
	KeyEnter       = sess.KeyEnter
	KeyHome        = sess.KeyHome
	KeyEnd         = sess.KeyEnd
	KeyPageUp      = sess.KeyPageUp
	KeyPageDown    = sess.KeyPageDown
	KeyDelete      = sess.KeyDelete
	MouseLeft      = sess.MouseLeft
	MouseWheelUp   = sess.MouseWheelUp
	MouseWheelDown = sess.MouseWheelDown
	MouseDrag      = sess.MouseDrag
	UnicodeLimit   = sess.UnicodeLimit
	LModeNone      = sess.LModeNone
)

var (
	TextStyleDefault = sess.TextStyleDefault
	TextStyleGutter  = sess.TextStyleGutter
	MakeTextStyle    = sess.MakeTextStyle
	TextStyleFg      = sess.TextStyleFg
	TextStyleBg      = sess.TextStyleBg
)

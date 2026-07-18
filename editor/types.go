package editor

import "github.com/jdpalmer/jem/app"

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
	CommandState         = app.CommandState
	SearchScopeMode      = app.SearchScopeMode
	PromptResult         = app.PromptResult
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

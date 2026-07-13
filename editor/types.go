package editor

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

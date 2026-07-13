package syntax

import "github.com/jdpalmer/jem/buffer"

// Palette supplies theme-dependent syntax styles.
type Palette struct {
	NormalStyle  buffer.TextStyle
	CommentStyle buffer.TextStyle
}

// PackagePalette is set by the editor during init.
var PackagePalette Palette

// Mode syntax kind constants (subset of main ModeInfo).
type ModeSyntaxKind int

const (
	ModeSyntaxGeneral         ModeSyntaxKind = 0
	ModeSyntaxNone            ModeSyntaxKind = 1
	ModeSyntaxHashCommentOnly ModeSyntaxKind = 2
	ModeSyntaxMarkdown        ModeSyntaxKind = 3
	ModeSyntaxHTML            ModeSyntaxKind = 4
)

// Mode flag constants used by the DFA.
const (
	ModeFlagIdentDash          uint32 = 1 << 0
	ModeFlagIdentLispExtra     uint32 = 1 << 1
	ModeFlagIdentLispSigil     uint32 = 1 << 2
	ModeFlagCommentSlashLine   uint32 = 1 << 3
	ModeFlagCommentSlashBlock  uint32 = 1 << 4
	ModeFlagPreprocHashAtBOL   uint32 = 1 << 5
	ModeFlagCommentHash        uint32 = 1 << 6
	ModeFlagCommentSemi        uint32 = 1 << 7
	ModeFlagCommentLua         uint32 = 1 << 8
	ModeFlagCommentPascalBrace uint32 = 1 << 9
	ModeFlagCommentPascalParen uint32 = 1 << 10
	ModeFlagAtRule             uint32 = 1 << 11
	ModeFlagNoCurlyRainbow     uint32 = 1 << 12
)

type modeSpec struct {
	SyntaxKind  ModeSyntaxKind
	SyntaxFlags uint32
}

func langModeSpec(mode buffer.LangMode) modeSpec {
	switch mode {
	case buffer.LModeNone:
		return modeSpec{ModeSyntaxNone, ModeFlagCommentSlashLine | ModeFlagCommentSlashBlock}
	case buffer.LModeC:
		return modeSpec{ModeSyntaxGeneral, ModeFlagCommentSlashLine | ModeFlagCommentSlashBlock | ModeFlagPreprocHashAtBOL}
	case buffer.LModeJava, buffer.LModeJavaScript, buffer.LModeActionScript, buffer.LModeTypeScript,
		buffer.LModeDart, buffer.LModeGo, buffer.LModeRust, buffer.LModeKotlin, buffer.LModeVerilog:
		return modeSpec{ModeSyntaxGeneral, ModeFlagCommentSlashLine | ModeFlagCommentSlashBlock}
	case buffer.LModeCSharp, buffer.LModeSwift:
		return modeSpec{ModeSyntaxGeneral, ModeFlagCommentSlashLine | ModeFlagCommentSlashBlock | ModeFlagPreprocHashAtBOL}
	case buffer.LModePython, buffer.LModeR:
		return modeSpec{ModeSyntaxGeneral, ModeFlagCommentHash}
	case buffer.LModeMake:
		return modeSpec{ModeSyntaxHashCommentOnly, ModeFlagCommentHash}
	case buffer.LModeLua:
		return modeSpec{ModeSyntaxGeneral, ModeFlagCommentLua}
	case buffer.LModeLisp:
		return modeSpec{ModeSyntaxGeneral, ModeFlagIdentLispExtra | ModeFlagIdentLispSigil | ModeFlagCommentSemi}
	case buffer.LModePascal:
		return modeSpec{ModeSyntaxGeneral, ModeFlagCommentPascalBrace | ModeFlagCommentPascalParen | ModeFlagNoCurlyRainbow}
	case buffer.LModeCSS:
		return modeSpec{ModeSyntaxGeneral, ModeFlagIdentDash | ModeFlagCommentSlashBlock | ModeFlagAtRule}
	case buffer.LModeHTML:
		return modeSpec{ModeSyntaxHTML, 0}
	case buffer.LModeMarkdown:
		return modeSpec{ModeSyntaxMarkdown, 0}
	default:
		return modeSpec{ModeSyntaxNone, ModeFlagCommentSlashLine | ModeFlagCommentSlashBlock}
	}
}

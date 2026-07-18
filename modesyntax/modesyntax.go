// Package modesyntax is the single source for per-language syntax kind and flags.
// syntax and modes both read Spec; neither maintains a parallel table.
package modesyntax

import "github.com/jdpalmer/jem/buffer"

// ModeSyntaxKind selects the highlighter entry path for a language mode.
type ModeSyntaxKind int

const (
	ModeSyntaxGeneral         ModeSyntaxKind = 0
	ModeSyntaxNone            ModeSyntaxKind = 1
	ModeSyntaxHashCommentOnly ModeSyntaxKind = 2
	ModeSyntaxMarkdown        ModeSyntaxKind = 3
	ModeSyntaxHTML            ModeSyntaxKind = 4
)

// Mode flag bits used by the syntax DFA and comment helpers.
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

// Spec is the highlighter-facing metadata for a LangMode.
type Spec struct {
	Kind  ModeSyntaxKind
	Flags uint32
}

// For returns syntax kind and flags for mode. Unknown modes use the Text fallback.
func For(mode buffer.LangMode) Spec {
	switch mode {
	case buffer.LModeNone:
		return Spec{ModeSyntaxNone, ModeFlagCommentSlashLine | ModeFlagCommentSlashBlock}
	case buffer.LModeC:
		return Spec{ModeSyntaxGeneral, ModeFlagCommentSlashLine | ModeFlagCommentSlashBlock | ModeFlagPreprocHashAtBOL}
	case buffer.LModeJava, buffer.LModeJavaScript, buffer.LModeActionScript, buffer.LModeTypeScript,
		buffer.LModeDart, buffer.LModeGo, buffer.LModeRust, buffer.LModeKotlin, buffer.LModeVerilog:
		return Spec{ModeSyntaxGeneral, ModeFlagCommentSlashLine | ModeFlagCommentSlashBlock}
	case buffer.LModeCSharp, buffer.LModeSwift:
		return Spec{ModeSyntaxGeneral, ModeFlagCommentSlashLine | ModeFlagCommentSlashBlock | ModeFlagPreprocHashAtBOL}
	case buffer.LModePython, buffer.LModeR:
		return Spec{ModeSyntaxGeneral, ModeFlagCommentHash}
	case buffer.LModeMake:
		return Spec{ModeSyntaxHashCommentOnly, ModeFlagCommentHash}
	case buffer.LModeLua:
		return Spec{ModeSyntaxGeneral, ModeFlagCommentLua}
	case buffer.LModeLisp:
		return Spec{ModeSyntaxGeneral, ModeFlagIdentLispExtra | ModeFlagIdentLispSigil | ModeFlagCommentSemi}
	case buffer.LModePascal:
		return Spec{ModeSyntaxGeneral, ModeFlagCommentPascalBrace | ModeFlagCommentPascalParen | ModeFlagNoCurlyRainbow}
	case buffer.LModeCSS:
		return Spec{ModeSyntaxGeneral, ModeFlagIdentDash | ModeFlagCommentSlashBlock | ModeFlagAtRule}
	case buffer.LModeHTML:
		return Spec{ModeSyntaxHTML, 0}
	case buffer.LModeMarkdown:
		return Spec{ModeSyntaxMarkdown, 0}
	default:
		return Spec{ModeSyntaxNone, ModeFlagCommentSlashLine | ModeFlagCommentSlashBlock}
	}
}

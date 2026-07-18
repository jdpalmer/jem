package modes

import "github.com/jdpalmer/jem/app"

func GetModeFlags(lang app.LangMode) uint32 {
	switch lang {
	case app.LModeNone:
		return app.ModeFlagCommentSlashLine | app.ModeFlagCommentSlashBlock
	case app.LModeC:
		return app.ModeFlagCommentSlashLine | app.ModeFlagCommentSlashBlock | app.ModeFlagPreprocHashAtBOL
	case app.LModeJava, app.LModeJavaScript, app.LModeActionScript, app.LModeTypeScript, app.LModeDart, app.LModeGo, app.LModeRust, app.LModeKotlin, app.LModeVerilog:
		return app.ModeFlagCommentSlashLine | app.ModeFlagCommentSlashBlock
	case app.LModeCSharp, app.LModeSwift:
		return app.ModeFlagCommentSlashLine | app.ModeFlagCommentSlashBlock | app.ModeFlagPreprocHashAtBOL
	case app.LModePython, app.LModeR, app.LModeMake:
		return app.ModeFlagCommentHash
	case app.LModeLua:
		return app.ModeFlagCommentLua
	case app.LModeLisp:
		return app.ModeFlagIdentLispExtra | app.ModeFlagIdentLispSigil | app.ModeFlagCommentSemi
	case app.LModePascal:
		return app.ModeFlagCommentPascalBrace | app.ModeFlagCommentPascalParen | app.ModeFlagNoCurlyRainbow
	case app.LModeCSS:
		return app.ModeFlagIdentDash | app.ModeFlagCommentSlashBlock | app.ModeFlagAtRule
	case app.LModeHTML, app.LModeMarkdown:
		return 0
	default:
		return 0
	}
}

package modes

import "github.com/jdpalmer/jem/session"

func GetModeFlags(lang session.LangMode) uint32 {
	switch lang {
	case session.LModeNone:
		return session.ModeFlagCommentSlashLine | session.ModeFlagCommentSlashBlock
	case session.LModeC:
		return session.ModeFlagCommentSlashLine | session.ModeFlagCommentSlashBlock | session.ModeFlagPreprocHashAtBOL
	case session.LModeJava, session.LModeJavaScript, session.LModeActionScript, session.LModeTypeScript, session.LModeDart, session.LModeGo, session.LModeRust, session.LModeKotlin, session.LModeVerilog:
		return session.ModeFlagCommentSlashLine | session.ModeFlagCommentSlashBlock
	case session.LModeCSharp, session.LModeSwift:
		return session.ModeFlagCommentSlashLine | session.ModeFlagCommentSlashBlock | session.ModeFlagPreprocHashAtBOL
	case session.LModePython, session.LModeR, session.LModeMake:
		return session.ModeFlagCommentHash
	case session.LModeLua:
		return session.ModeFlagCommentLua
	case session.LModeLisp:
		return session.ModeFlagIdentLispExtra | session.ModeFlagIdentLispSigil | session.ModeFlagCommentSemi
	case session.LModePascal:
		return session.ModeFlagCommentPascalBrace | session.ModeFlagCommentPascalParen | session.ModeFlagNoCurlyRainbow
	case session.LModeCSS:
		return session.ModeFlagIdentDash | session.ModeFlagCommentSlashBlock | session.ModeFlagAtRule
	case session.LModeHTML, session.LModeMarkdown:
		return 0
	default:
		return 0
	}
}

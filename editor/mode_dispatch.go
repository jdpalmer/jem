package editor

import "github.com/jdpalmer/jem/modes"

func CmdModeNewlineAndIndent(f bool, n int) bool {
	bp := session.App.CurrentBuffer
	if bp != nil {
		switch bp.Name {
		case grepBufferName:
			return CmdGrepVisitMatch(false, 1)
		case compileBufferName:
			return CmdCompileVisitDiag(false, 1)
		}
	}
	return modes.CmdModeNewlineAndIndent(f, n)
}

func CmdModeIndentLine(f bool, n int) bool    { return modes.CmdModeIndentLine(f, n) }
func CmdModeCloseBrace(f bool, n int) bool    { return modes.CmdModeCloseBrace(f, n) }
func CmdModeGotoMatch(f bool, n int) bool     { return modes.CmdModeGotoMatch(f, n) }
func CmdModeMakeComment(f bool, n int) bool   { return modes.CmdModeMakeComment(f, n) }
func CmdModeTopOfFunction(f bool, n int) bool { return modes.CmdModeTopOfFunction(f, n) }
func CmdModeEndOfFunction(f bool, n int) bool { return modes.CmdModeEndOfFunction(f, n) }
func CmdModeMarkFunction(f bool, n int) bool  { return modes.CmdModeMarkFunction(f, n) }
func CmdModeToggleComment(f bool, n int) bool { return modes.CmdModeToggleComment(f, n) }
func CmdCommentDwim(f bool, n int) bool       { return modes.CmdCommentDwim(f, n) }

package editor

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/modeactions"
)

func CmdModeNewlineAndIndent(f bool, n int) bool {
	bp := app.State.CurrentBuffer
	if bp != nil {
		switch bp.Name {
		case grepBufferName:
			return CmdGrepVisitMatch(false, 1)
		case compileBufferName:
			return CmdCompileVisitDiag(false, 1)
		}
	}
	return modeactions.CmdModeNewlineAndIndent(f, n)
}

func CmdModeIndentLine(f bool, n int) bool    { return modeactions.CmdModeIndentLine(f, n) }
func CmdModeCloseBrace(f bool, n int) bool    { return modeactions.CmdModeCloseBrace(f, n) }
func CmdModeGotoMatch(f bool, n int) bool     { return modeactions.CmdModeGotoMatch(f, n) }
func CmdModeMakeComment(f bool, n int) bool   { return modeactions.CmdModeMakeComment(f, n) }
func CmdModeTopOfFunction(f bool, n int) bool { return modeactions.CmdModeTopOfFunction(f, n) }
func CmdModeEndOfFunction(f bool, n int) bool { return modeactions.CmdModeEndOfFunction(f, n) }
func CmdModeMarkFunction(f bool, n int) bool  { return modeactions.CmdModeMarkFunction(f, n) }
func CmdModeToggleComment(f bool, n int) bool { return modeactions.CmdModeToggleComment(f, n) }
func CmdCommentDwim(f bool, n int) bool       { return modeactions.CmdCommentDwim(f, n) }

package modes

import "github.com/jdpalmer/jem/app"

func ModeNewlineAndIndent(f bool, n int) bool {
	_ = f
	wp := app.State.CurrentWindow
	if wp == nil || PackageHooks.WindowInsertNewline == nil {
		return false
	}
	for i := 0; i < n; i++ {
		if !PackageHooks.WindowInsertNewline(wp) {
			return false
		}
	}
	return true
}

func ModeIndentLine(f bool, n int) bool {
	_ = f
	_ = n
	return true
}

func ModeCloseBrace(f bool, n int) bool {
	_ = f
	wp := app.State.CurrentWindow
	if wp == nil || PackageHooks.WindowInsertCodepoint == nil {
		return false
	}
	for i := 0; i < n; i++ {
		if !PackageHooks.WindowInsertCodepoint(wp, '}') {
			return false
		}
	}
	return true
}

func ModeGotoMatch(f bool, n int) bool {
	_ = f
	_ = n
	if PackageHooks.DefaultGotoMatch == nil {
		return false
	}
	return PackageHooks.DefaultGotoMatch(false, 1)
}

func ModeMakeComment(f bool, n int) bool   { return false }
func ModeTopOfFunction(f bool, n int) bool { return false }
func ModeEndOfFunction(f bool, n int) bool { return false }
func ModeMarkFunction(f bool, n int) bool  { return false }

func init() {
	for i := range modeTable {
		if modeTable[i].NewlineAndIndent == nil {
			modeTable[i].NewlineAndIndent = ModeNewlineAndIndent
		}
		if modeTable[i].IndentLine == nil {
			modeTable[i].IndentLine = ModeIndentLine
		}
		if modeTable[i].CloseBrace == nil {
			modeTable[i].CloseBrace = ModeCloseBrace
		}
		if modeTable[i].GotoMatch == nil {
			modeTable[i].GotoMatch = ModeGotoMatch
		}
		if modeTable[i].MakeComment == nil {
			modeTable[i].MakeComment = ModeMakeComment
		}
		if modeTable[i].TopOfFunction == nil {
			modeTable[i].TopOfFunction = ModeTopOfFunction
		}
		if modeTable[i].EndOfFunction == nil {
			modeTable[i].EndOfFunction = ModeEndOfFunction
		}
		if modeTable[i].MarkFunction == nil {
			modeTable[i].MarkFunction = ModeMarkFunction
		}
	}
}

package runtime

import (
	"sort"
	"strings"

	"github.com/jdpalmer/jem/mode"
	"github.com/jdpalmer/jem/search"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/tools"
)

// cmd0 adapts a no-arg command for the (f, n) registry signature.
func cmd0(fn func() bool) CommandFunc {
	return func(bool, int) bool { return fn() }
}

type CommandFunc func(f bool, n int) bool
type Command struct {
	Name       string
	Fn         CommandFunc
	Doc        string
	Keys       []uint32
	AcceptsArg bool
}

var commandTable []Command
var commandNameMap map[string]CommandFunc
var keybindingsMap map[uint32]CommandFunc
var commandAcceptsArgByKey map[uint32]bool

// InitCommands populates commandNameMap and keybindingsMap from commandTable.
func InitCommands() {
	commandTable = []Command{
		{Name: "Abort", Fn: CmdAbort, Doc: "Abort the current prompt, macro, or transient operation.", AcceptsArg: false, Keys: []uint32{term.CTL | 'G'}},
		{Name: "BackToIndentationU", Fn: CmdBackToIndentation, Doc: "Move to the first non-whitespace character on the line.", AcceptsArg: false, Keys: []uint32{term.META | 'm'}},
		{Name: "BackwardChar", Fn: CmdBackwardChar, Doc: "Move backward by characters.", AcceptsArg: true, Keys: []uint32{term.KeyLeft}},
		{Name: "BackwardLine", Fn: CmdBackwardLine, Doc: "Move upward by lines.", AcceptsArg: true, Keys: []uint32{term.KeyUp}},
		{Name: "BackwardPage", Fn: CmdBackwardPage, Doc: "Scroll backward by pages.", AcceptsArg: true, Keys: []uint32{term.KeyPageUp, term.META | 'V', term.SHIFT | term.KeyUp}},
		{Name: "BackwardSexp", Fn: CmdBackwardSexp, Doc: "Move backward past a balanced expression.", AcceptsArg: true, Keys: []uint32{term.CTL | term.META | 'B'}},
		{Name: "BackwardWord", Fn: CmdBackwardWord, Doc: "Move backward by words.", AcceptsArg: true, Keys: []uint32{term.META | 'B', term.SHIFT | term.KeyLeft}},
		{Name: "CapWord", Fn: CmdCapWord, Doc: "Capitalize the next word.", AcceptsArg: true, Keys: []uint32{term.META | 'C'}},
		{Name: "CamelCase", Fn: CmdCamelCase, Doc: "Convert the identifier at point to camelCase.", AcceptsArg: false},
		{Name: "CommandPalette", Fn: CmdCommandPalette, Doc: "Open the command palette with fuzzy command search.", AcceptsArg: false, Keys: []uint32{term.META | 'X'}},
		{Name: "CommentDwim", Fn: mode.CmdCommentDwim, Doc: "Comment or uncomment region/line (DWIM).", AcceptsArg: false, Keys: []uint32{term.META | ';'}},
		{Name: "Compile", Fn: cmd0(CmdCompile), Doc: "Run a build command and capture diagnostics in *compile*.", AcceptsArg: false},
		{Name: "CompileVisitDiag", Fn: cmd0(CmdCompileVisitDiag), Doc: "Visit the source location for the selected compile diagnostic.", AcceptsArg: false},
		{Name: "CompletionAccept", Fn: CmdAccept, Doc: "Accept the pending Completion suggestion.", AcceptsArg: false, Keys: []uint32{term.SHIFT | term.KeyEnter}},
		{Name: "CompletionComplete", Fn: CmdComplete, Doc: "Request a Completion suggestion.", AcceptsArg: false, Keys: []uint32{term.SHIFT | term.KeyTab}},
		{Name: "ConstantCase", Fn: CmdConstantCase, Doc: "Convert the identifier at point to CONSTANT_CASE.", AcceptsArg: false},
		{Name: "CopyRegion", Fn: CmdCopyRegion, Doc: "Copy the active region.", AcceptsArg: false, Keys: []uint32{term.META | 'W'}},
		{Name: "CopyRegister", Fn: CmdCopyRegister, Doc: "Copy the active region to a named register.", AcceptsArg: false},
		{Name: "DeleteBackward", Fn: CmdDeleteBackward, Doc: "Delete the previous character.", AcceptsArg: true, Keys: []uint32{0x7F}},
		{Name: "DeleteBlankLines", Fn: CmdDeleteBlankLines, Doc: "Collapse surrounding blank lines.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'O'}},
		{Name: "DeleteForward", Fn: CmdDeleteForward, Doc: "Delete the next character.", AcceptsArg: true, Keys: []uint32{term.CTL | 'D'}},
		{Name: "DeleteWordBackward", Fn: CmdDeleteWordBackward, Doc: "Delete the previous word.", AcceptsArg: true, Keys: []uint32{term.META | 'H', term.META | 0x7F}},
		{Name: "DeleteWordForward", Fn: CmdDeleteWordForward, Doc: "Delete the next word.", AcceptsArg: true, Keys: []uint32{term.META | 'D'}},
		{Name: "DescribeCommand", Fn: CmdDescribeCommand, Doc: "Show one command name and its description.", AcceptsArg: false},
		{Name: "DescribeVariable", Fn: CmdDescribeVariable, Doc: "Show one variable value and description.", AcceptsArg: false},
		{Name: "FileRead", Fn: CmdFileRead, Doc: "Read a file into the current buffer.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'R'}},
		{Name: "FileSave", Fn: CmdFileSave, Doc: "Save the current buffer.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'S'}},
		{Name: "FileVisit", Fn: CmdFileVisit, Doc: "Visit a file in its own buffer.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'F', term.CTLX | term.CTL | 'F'}},
		{Name: "FileWrite", Fn: CmdFileWrite, Doc: "Write the current buffer to a new file.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'W', term.CTLX | term.CTL | 'W'}},
		{Name: "FillParagraph", Fn: CmdFillParagraph, Doc: "Fill the current paragraph.", AcceptsArg: false, Keys: []uint32{term.META | 'Q'}},
		{Name: "ForwardChar", Fn: CmdForwardChar, Doc: "Move forward by characters.", AcceptsArg: true, Keys: []uint32{term.KeyRight}},
		{Name: "ForwardLine", Fn: CmdForwardLine, Doc: "Move downward by lines.", AcceptsArg: true, Keys: []uint32{term.KeyDown}},
		{Name: "ForwardPage", Fn: CmdForwardPage, Doc: "Scroll forward by pages.", AcceptsArg: true, Keys: []uint32{term.KeyPageDown, term.CTL | 'V', term.SHIFT | term.KeyDown}},
		{Name: "ForwardSexp", Fn: CmdForwardSexp, Doc: "Move forward past a balanced expression.", AcceptsArg: true, Keys: []uint32{term.CTL | term.META | 'F'}},
		{Name: "ForwardWord", Fn: CmdForwardWord, Doc: "Move forward by words.", AcceptsArg: true, Keys: []uint32{term.META | 'F', term.SHIFT | term.KeyRight}},
		{Name: "GotoBof", Fn: CmdGotoBof, Doc: "Move to the start of the buffer.", AcceptsArg: false, Keys: []uint32{term.META | '<'}},
		{Name: "GotoBol", Fn: CmdGotoBol, Doc: "Move to the start of the line.", AcceptsArg: false, Keys: []uint32{term.CTL | 'A'}},
		{Name: "GotoEof", Fn: CmdGotoEOF, Doc: "Move to the end of the buffer.", AcceptsArg: false, Keys: []uint32{term.META | '>'}},
		{Name: "GotoEol", Fn: CmdGotoEol, Doc: "Move to the end of the line.", AcceptsArg: false, Keys: []uint32{term.CTL | 'E'}},
		{Name: "GotoLine", Fn: CmdGotoLine, Doc: "Jump to a specific line.", AcceptsArg: true, Keys: []uint32{term.META | 'G'}},
		{Name: "GotoTag", Fn: cmd0(CmdGotoTag), Doc: "Jump to a tag definition.", AcceptsArg: false, Keys: []uint32{term.META | '.'}},
		{Name: "GrepProject", Fn: cmd0(CmdGrep), Doc: "Search project files with ripgrep and open a jump buffer.", AcceptsArg: false},
		{Name: "GrepVisitMatch", Fn: cmd0(CmdGrepVisitMatch), Doc: "Visit the source location for the selected grep match.", AcceptsArg: false},
		{Name: "InsertDate", Fn: CmdInsertDate, Doc: "Insert the current date.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'D'}},
		{Name: "InsertRegister", Fn: CmdInsertRegister, Doc: "Insert the contents of a named register.", AcceptsArg: false},
		{Name: "IsearchBackward", Fn: cmd0(CmdIsearchBackward), Doc: "Incrementally search backward.", AcceptsArg: false, Keys: []uint32{term.CTL | 'R'}},
		{Name: "IsearchForward", Fn: cmd0(CmdIsearchForward), Doc: "Incrementally search forward.", AcceptsArg: false, Keys: []uint32{term.CTL | 'S'}},
		{Name: "IsearchReBackward", Fn: cmd0(CmdIsearchReBackward), Doc: "Incrementally search backward with regex.", AcceptsArg: false, Keys: []uint32{term.META | term.CTL | 'R'}},
		{Name: "IsearchReForward", Fn: cmd0(CmdIsearchReForward), Doc: "Incrementally search forward with regex.", AcceptsArg: false, Keys: []uint32{term.META | term.CTL | 'S'}},
		{Name: "Kill", Fn: CmdKill, Doc: "Kill text from point.", AcceptsArg: true, Keys: []uint32{term.CTL | 'K'}},
		{Name: "KillBuffer", Fn: CmdKillBuffer, Doc: "Kill one buffer.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'K'}},
		{Name: "KillBufferFuzzy", Fn: CmdKillBufferFuzzy, Doc: "Kill one buffer with fuzzy matching.", AcceptsArg: false},
		{Name: "KillRegion", Fn: CmdKillRegion, Doc: "Kill the active region.", AcceptsArg: false, Keys: []uint32{term.CTL | 'W'}},
		{Name: "LowerRegion", Fn: CmdLowerRegion, Doc: "Lowercase the active region.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'L'}},
		{Name: "LowerWord", Fn: CmdLowerWord, Doc: "Lowercase the next word.", AcceptsArg: true, Keys: []uint32{term.META | 'L'}},
		{Name: "MacroEnd", Fn: CmdMacroEnd, Doc: "Stop recording a keyboard macro.", AcceptsArg: false, Keys: []uint32{term.CTLX | ')'}},
		{Name: "MacroExec", Fn: CmdMacroExec, Doc: "Replay the last keyboard macro.", AcceptsArg: true, Keys: []uint32{term.CTLX | 'E'}},
		{Name: "MacroStart", Fn: CmdMacroStart, Doc: "Start recording a keyboard macro.", AcceptsArg: false, Keys: []uint32{term.CTLX | '('}},
		{Name: "MarkPop", Fn: CmdMarkPop, Doc: "Pop back to the most recently pushed mark.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | ' '}},
		{Name: "MarkPush", Fn: CmdMarkPush, Doc: "Push the current location onto the mark stack.", AcceptsArg: false, Keys: []uint32{term.CTLX | ' '}},
		{Name: "MarkWholeBuffer", Fn: CmdMarkWholeBuffer, Doc: "Mark the entire buffer as the active region.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'H'}},
		{Name: "MenuRun", Fn: CmdMenuRun, Doc: "Open the message-line menu.", AcceptsArg: false, Keys: []uint32{term.CTL | '_', term.CTL | '/'}},
		{Name: "ModeCloseBrace", Fn: mode.CmdModeCloseBrace, Doc: "Insert a close brace with mode-aware behavior.", AcceptsArg: true, Keys: []uint32{'}'}},
		{Name: "ModeEndOfFunction", Fn: mode.CmdModeEndOfFunction, Doc: "Jump to the end of the current function.", AcceptsArg: false, Keys: []uint32{term.META | term.CTL | 'E'}},
		{Name: "ModeGotoMatch", Fn: mode.CmdModeGotoMatch, Doc: "Jump to the matching delimiter.", AcceptsArg: false, Keys: []uint32{term.CTL | '\\'}},
		{Name: "ModeIndentLine", Fn: mode.CmdModeIndentLine, Doc: "Indent the current line using the active mode.", AcceptsArg: false, Keys: []uint32{term.KeyTab}},
		{Name: "ModeMarkFunction", Fn: mode.CmdModeMarkFunction, Doc: "Mark the current function.", AcceptsArg: false, Keys: []uint32{term.META | term.CTL | 'H'}},
		{Name: "ModeNewlineAndIndent", Fn: CmdModeNewlineAndIndent, Doc: "Insert a newline using the active mode.", AcceptsArg: true, Keys: []uint32{term.KeyEnter, '\r', '\n'}},
		{Name: "ModeTopOfFunction", Fn: mode.CmdModeTopOfFunction, Doc: "Jump to the start of the current function.", AcceptsArg: false, Keys: []uint32{term.META | term.CTL | 'A'}},
		{Name: "MouseDrag", Fn: CmdMouseDrag, Doc: "Extend the selection with the mouse.", AcceptsArg: false, Keys: []uint32{term.MouseDrag}},
		{Name: "MouseLeft", Fn: CmdMouseLeft, Doc: "Move point with the mouse.", AcceptsArg: false, Keys: []uint32{term.MouseLeft}},
		{Name: "MouseWheelDown", Fn: CmdMouseWheelDown, Doc: "Scroll down with the mouse wheel.", AcceptsArg: false, Keys: []uint32{term.MouseWheelDown}},
		{Name: "MouseWheelUp", Fn: CmdMouseWheelUp, Doc: "Scroll up with the mouse wheel.", AcceptsArg: false, Keys: []uint32{term.MouseWheelUp}},
		{Name: "OpenLine", Fn: CmdOpenLine, Doc: "Open a blank line after point.", AcceptsArg: true, Keys: []uint32{term.CTL | 'O'}},
		{Name: "PascalCase", Fn: CmdPascalCase, Doc: "Convert the identifier at point to PascalCase.", AcceptsArg: false},
		{Name: "QueryReReplace", Fn: cmd0(CmdQueryReReplace), Doc: "Query replace with a regular expression.", AcceptsArg: false},
		{Name: "QueryReplace", Fn: cmd0(CmdQueryReplace), Doc: "Query replace plain text.", AcceptsArg: false, Keys: []uint32{term.META | '%'}},
		{Name: "Quit", Fn: CmdQuit, Doc: "Quit Jem.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'C'}},
		{Name: "Quote", Fn: CmdQuote, Doc: "Insert the next character literally.", AcceptsArg: true, Keys: []uint32{term.CTL | 'Q'}},
		{Name: "Refresh", Fn: CmdRefresh, Doc: "Refresh the current window.", AcceptsArg: false, Keys: []uint32{term.CTL | 'L'}},
		{Name: "RevertFile", Fn: CmdRevertFile, Doc: "Revert the current buffer to the on-disk file.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'V'}},
		{Name: "SearchBackward", Fn: cmd0(CmdSearchBackward), Doc: "Search backward for a string.", AcceptsArg: false},
		{Name: "SearchForward", Fn: cmd0(CmdSearchForward), Doc: "Search forward for a string.", AcceptsArg: false},
		{Name: "SetEolMode", Fn: CmdSetEolMode, Doc: "Set the current buffer line ending mode.", AcceptsArg: false},
		{Name: "SetMark", Fn: CmdSetMark, Doc: "Set the mark.", AcceptsArg: false, Keys: []uint32{term.CTL | ' '}},
		{Name: "SetVariable", Fn: CmdSetVariable, Doc: "Set a named editor variable.", AcceptsArg: false},
		{Name: "ShowPosition", Fn: CmdShowPosition, Doc: "Show the current cursor position.", AcceptsArg: false, Keys: []uint32{term.CTLX | '='}},
		{Name: "SnakeCase", Fn: CmdSnakeCase, Doc: "Convert the identifier at point to snake_case.", AcceptsArg: false},
		{Name: "SortRegion", Fn: CmdSortRegion, Doc: "Sort the active region by lines.", AcceptsArg: false},
		{Name: "Spawn", Fn: cmd0(CmdSpawn), Doc: "Run a shell command.", AcceptsArg: false, Keys: []uint32{term.CTLX | '!'}},
		{Name: "SpawnCli", Fn: cmd0(tools.RunSpawnCLI), Doc: "Open an interactive shell.", AcceptsArg: false, Keys: []uint32{term.META | '!'}},
		{Name: "SwapMark", Fn: CmdSwapMark, Doc: "Swap point and mark.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'X'}},
		{Name: "ThemeToggle", Fn: CmdThemeToggle, Doc: "Toggle between dark and light themes.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'T'}},
		{Name: "ToggleSearchScope", Fn: cmd0(search.ToggleSearchScope), Doc: "Toggle search between one buffer and all buffers.", AcceptsArg: false},
		{Name: "TransposeChars", Fn: CmdTransposeChars, Doc: "Transpose the characters around point.", AcceptsArg: false, Keys: []uint32{term.CTL | 'T'}},
		{Name: "TransposeLines", Fn: CmdTransposeLines, Doc: "Transpose the current line with the line above it.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'T'}},
		{Name: "TransposeWords", Fn: CmdTransposeWords, Doc: "Transpose the words around point.", AcceptsArg: false, Keys: []uint32{term.META | 'T'}},
		{Name: "TrimWhitespace", Fn: CmdTrimWhitespace, Doc: "Trim surrounding whitespace.", AcceptsArg: false, Keys: []uint32{term.META | '\\'}},
		{Name: "Undo", Fn: CmdUndo, Doc: "Undo the most recent editing command.", AcceptsArg: true, Keys: []uint32{term.CTL | 'Z'}},
		{Name: "UpperRegion", Fn: CmdUpperRegion, Doc: "Uppercase the active region.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'U'}},
		{Name: "UpperWord", Fn: CmdUpperWord, Doc: "Uppercase the next word.", AcceptsArg: true, Keys: []uint32{term.META | 'U'}},
		{Name: "UseBuffer", Fn: CmdUseBuffer, Doc: "Switch to another buffer.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'B'}},
		{Name: "UseBufferFuzzy", Fn: CmdUseBuffer, Doc: "Switch to another buffer with fuzzy matching.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'B'}},
		{Name: "WindowDelete", Fn: CmdWindowDelete, Doc: "Delete the current window.", AcceptsArg: false, Keys: []uint32{term.CTLX | '0'}},
		{Name: "WindowNext", Fn: CmdWindowNext, Doc: "Select the next window.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'O'}},
		{Name: "WindowOnly", Fn: CmdWindowOnly, Doc: "Make the current window the only window.", AcceptsArg: false, Keys: []uint32{term.CTLX | '1'}},
		{Name: "WindowSplit", Fn: CmdWindowSplit, Doc: "Split the current window.", AcceptsArg: false, Keys: []uint32{term.CTLX | '2'}},
		{Name: "Yank", Fn: CmdYank, Doc: "Yank the most recently killed text.", AcceptsArg: true, Keys: []uint32{term.CTL | 'Y'}},
	}
	commandNameMap = make(map[string]CommandFunc, len(commandTable))
	keybindingsMap = make(map[uint32]CommandFunc)
	commandAcceptsArgByKey = make(map[uint32]bool)
	for i := range commandTable {
		c := &commandTable[i]
		if c.Name != "" {
			commandNameMap[strings.ToLower(c.Name)] = c.Fn
		}
		for _, key := range c.Keys {
			keybindingsMap[key] = c.Fn
			if c.AcceptsArg {
				commandAcceptsArgByKey[key] = true
			}
		}
	}
}

func commandByName(name string) *Command {
	norm := strings.ToLower(strings.TrimSpace(name))
	for i := range commandTable {
		if strings.ToLower(commandTable[i].Name) == norm {
			return &commandTable[i]
		}
	}
	return nil
}

func buildCommandList() []string {
	names := make([]string, 0, len(commandTable))
	for i := range commandTable {
		if commandTable[i].Name != "" {
			names = append(names, commandTable[i].Name)
		}
	}
	sort.Strings(names)
	return names
}

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
		{Name: "abort", Fn: CmdAbort, Doc: "Abort the current prompt, macro, or transient operation.", AcceptsArg: false, Keys: []uint32{term.CTL | 'G'}},
		{Name: "back_to_indentation", Fn: CmdBackToIndentation, Doc: "Move to the first non-whitespace character on the line.", AcceptsArg: false, Keys: []uint32{term.META | 'm'}},
		{Name: "backward_char", Fn: CmdBackwardChar, Doc: "Move backward by characters.", AcceptsArg: true, Keys: []uint32{term.KeyLeft}},
		{Name: "backward_line", Fn: CmdBackwardLine, Doc: "Move upward by lines.", AcceptsArg: true, Keys: []uint32{term.KeyUp}},
		{Name: "backward_page", Fn: CmdBackwardPage, Doc: "Scroll backward by pages.", AcceptsArg: true, Keys: []uint32{term.KeyPageUp, term.META | 'V', term.SHIFT | term.KeyUp}},
		{Name: "backward_sexp", Fn: CmdBackwardSexp, Doc: "Move backward past a balanced expression.", AcceptsArg: true, Keys: []uint32{term.CTL | term.META | 'B'}},
		{Name: "backward_word", Fn: CmdBackwardWord, Doc: "Move backward by words.", AcceptsArg: true, Keys: []uint32{term.META | 'B', term.SHIFT | term.KeyLeft}},
		{Name: "cap_word", Fn: CmdCapWord, Doc: "Capitalize the next word.", AcceptsArg: true, Keys: []uint32{term.META | 'C'}},
		{Name: "camel_case", Fn: CmdCamelCase, Doc: "Convert the identifier at point to camelCase.", AcceptsArg: false},
		{Name: "command_palette", Fn: CmdCommandPalette, Doc: "Open the command palette with fuzzy command search.", AcceptsArg: false, Keys: []uint32{term.META | 'X'}},
		{Name: "comment_dwim", Fn: mode.CmdCommentDwim, Doc: "Comment or uncomment region/line (DWIM).", AcceptsArg: false, Keys: []uint32{term.META | ';'}},
		{Name: "compile", Fn: cmd0(CmdCompile), Doc: "Run a build command and capture diagnostics in *compile*.", AcceptsArg: false},
		{Name: "compile_visit_diag", Fn: cmd0(CmdCompileVisitDiag), Doc: "Visit the source location for the selected compile diagnostic.", AcceptsArg: false},
		{Name: "completion_accept", Fn: CmdAccept, Doc: "Accept the pending Completion suggestion.", AcceptsArg: false, Keys: []uint32{term.SHIFT | term.KeyEnter}},
		{Name: "completion_complete", Fn: CmdComplete, Doc: "Request a Completion suggestion.", AcceptsArg: false, Keys: []uint32{term.SHIFT | term.KeyTab}},
		{Name: "constant_case", Fn: CmdConstantCase, Doc: "Convert the identifier at point to CONSTANT_CASE.", AcceptsArg: false},
		{Name: "copy_region", Fn: CmdCopyRegion, Doc: "Copy the active region.", AcceptsArg: false, Keys: []uint32{term.META | 'W'}},
		{Name: "copy_register", Fn: CmdCopyRegister, Doc: "Copy the active region to a named register.", AcceptsArg: false},
		{Name: "delete_backward", Fn: CmdDeleteBackward, Doc: "Delete the previous character.", AcceptsArg: true, Keys: []uint32{0x7F}},
		{Name: "delete_blank_lines", Fn: CmdDeleteBlankLines, Doc: "Collapse surrounding blank lines.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'O'}},
		{Name: "delete_forward", Fn: CmdDeleteForward, Doc: "Delete the next character.", AcceptsArg: true, Keys: []uint32{term.CTL | 'D'}},
		{Name: "delete_word_backward", Fn: CmdDeleteWordBackward, Doc: "Delete the previous word.", AcceptsArg: true, Keys: []uint32{term.META | 'H', term.META | 0x7F}},
		{Name: "delete_word_forward", Fn: CmdDeleteWordForward, Doc: "Delete the next word.", AcceptsArg: true, Keys: []uint32{term.META | 'D'}},
		{Name: "describe_command", Fn: CmdDescribeCommand, Doc: "Show one command name and its description.", AcceptsArg: false},
		{Name: "describe_variable", Fn: CmdDescribeVariable, Doc: "Show one variable value and description.", AcceptsArg: false},
		{Name: "file_read", Fn: CmdFileRead, Doc: "Read a file into the current buffer.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'R'}},
		{Name: "file_save", Fn: CmdFileSave, Doc: "Save the current buffer.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'S'}},
		{Name: "file_visit", Fn: CmdFileVisit, Doc: "Visit a file in its own buffer.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'F', term.CTLX | term.CTL | 'F'}},
		{Name: "file_write", Fn: CmdFileWrite, Doc: "Write the current buffer to a new file.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'W', term.CTLX | term.CTL | 'W'}},
		{Name: "fill_paragraph", Fn: CmdFillParagraph, Doc: "Fill the current paragraph.", AcceptsArg: false, Keys: []uint32{term.META | 'Q'}},
		{Name: "forward_char", Fn: CmdForwardChar, Doc: "Move forward by characters.", AcceptsArg: true, Keys: []uint32{term.KeyRight}},
		{Name: "forward_line", Fn: CmdForwardLine, Doc: "Move downward by lines.", AcceptsArg: true, Keys: []uint32{term.KeyDown}},
		{Name: "forward_page", Fn: CmdForwardPage, Doc: "Scroll forward by pages.", AcceptsArg: true, Keys: []uint32{term.KeyPageDown, term.CTL | 'V', term.SHIFT | term.KeyDown}},
		{Name: "forward_sexp", Fn: CmdForwardSexp, Doc: "Move forward past a balanced expression.", AcceptsArg: true, Keys: []uint32{term.CTL | term.META | 'F'}},
		{Name: "forward_word", Fn: CmdForwardWord, Doc: "Move forward by words.", AcceptsArg: true, Keys: []uint32{term.META | 'F', term.SHIFT | term.KeyRight}},
		{Name: "goto_bof", Fn: CmdGotoBof, Doc: "Move to the start of the buffer.", AcceptsArg: false, Keys: []uint32{term.META | '<'}},
		{Name: "goto_bol", Fn: CmdGotoBol, Doc: "Move to the start of the line.", AcceptsArg: false, Keys: []uint32{term.CTL | 'A'}},
		{Name: "goto_eof", Fn: CmdGotoEOF, Doc: "Move to the end of the buffer.", AcceptsArg: false, Keys: []uint32{term.META | '>'}},
		{Name: "goto_eol", Fn: CmdGotoEol, Doc: "Move to the end of the line.", AcceptsArg: false, Keys: []uint32{term.CTL | 'E'}},
		{Name: "goto_line", Fn: CmdGotoLine, Doc: "Jump to a specific line.", AcceptsArg: true, Keys: []uint32{term.META | 'G'}},
		{Name: "goto_tag", Fn: cmd0(CmdGotoTag), Doc: "Jump to a tag definition.", AcceptsArg: false, Keys: []uint32{term.META | '.'}},
		{Name: "grep_project", Fn: cmd0(CmdGrep), Doc: "Search project files with ripgrep and open a jump buffer.", AcceptsArg: false},
		{Name: "grep_visit_match", Fn: cmd0(CmdGrepVisitMatch), Doc: "Visit the source location for the selected grep match.", AcceptsArg: false},
		{Name: "insert_date", Fn: CmdInsertDate, Doc: "Insert the current date.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'D'}},
		{Name: "insert_register", Fn: CmdInsertRegister, Doc: "Insert the contents of a named register.", AcceptsArg: false},
		{Name: "isearch_backward", Fn: cmd0(CmdIsearchBackward), Doc: "Incrementally search backward.", AcceptsArg: false, Keys: []uint32{term.CTL | 'R'}},
		{Name: "isearch_forward", Fn: cmd0(CmdIsearchForward), Doc: "Incrementally search forward.", AcceptsArg: false, Keys: []uint32{term.CTL | 'S'}},
		{Name: "isearch_re_backward", Fn: cmd0(CmdIsearchReBackward), Doc: "Incrementally search backward with regex.", AcceptsArg: false, Keys: []uint32{term.META | term.CTL | 'R'}},
		{Name: "isearch_re_forward", Fn: cmd0(CmdIsearchReForward), Doc: "Incrementally search forward with regex.", AcceptsArg: false, Keys: []uint32{term.META | term.CTL | 'S'}},
		{Name: "kill", Fn: CmdKill, Doc: "Kill text from point.", AcceptsArg: true, Keys: []uint32{term.CTL | 'K'}},
		{Name: "kill_buffer", Fn: CmdKillBuffer, Doc: "Kill one buffer.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'K'}},
		{Name: "kill_buffer_fuzzy", Fn: CmdKillBufferFuzzy, Doc: "Kill one buffer with fuzzy matching.", AcceptsArg: false},
		{Name: "kill_region", Fn: CmdKillRegion, Doc: "Kill the active region.", AcceptsArg: false, Keys: []uint32{term.CTL | 'W'}},
		{Name: "lower_region", Fn: CmdLowerRegion, Doc: "Lowercase the active region.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'L'}},
		{Name: "lower_word", Fn: CmdLowerWord, Doc: "Lowercase the next word.", AcceptsArg: true, Keys: []uint32{term.META | 'L'}},
		{Name: "macro_end", Fn: CmdMacroEnd, Doc: "Stop recording a keyboard macro.", AcceptsArg: false, Keys: []uint32{term.CTLX | ')'}},
		{Name: "macro_exec", Fn: CmdMacroExec, Doc: "Replay the last keyboard macro.", AcceptsArg: true, Keys: []uint32{term.CTLX | 'E'}},
		{Name: "macro_start", Fn: CmdMacroStart, Doc: "Start recording a keyboard macro.", AcceptsArg: false, Keys: []uint32{term.CTLX | '('}},
		{Name: "mark_pop", Fn: CmdMarkPop, Doc: "Pop back to the most recently pushed mark.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | ' '}},
		{Name: "mark_push", Fn: CmdMarkPush, Doc: "Push the current location onto the mark stack.", AcceptsArg: false, Keys: []uint32{term.CTLX | ' '}},
		{Name: "mark_whole_buffer", Fn: CmdMarkWholeBuffer, Doc: "Mark the entire buffer as the active region.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'H'}},
		{Name: "menu_run", Fn: CmdMenuRun, Doc: "Open the message-line menu.", AcceptsArg: false, Keys: []uint32{term.CTL | '_', term.CTL | '/'}},
		{Name: "mode_close_brace", Fn: mode.CmdModeCloseBrace, Doc: "Insert a close brace with mode-aware behavior.", AcceptsArg: true, Keys: []uint32{'}'}},
		{Name: "mode_end_of_function", Fn: mode.CmdModeEndOfFunction, Doc: "Jump to the end of the current function.", AcceptsArg: false, Keys: []uint32{term.META | term.CTL | 'E'}},
		{Name: "mode_goto_match", Fn: mode.CmdModeGotoMatch, Doc: "Jump to the matching delimiter.", AcceptsArg: false, Keys: []uint32{term.CTL | '\\'}},
		{Name: "mode_indent_line", Fn: mode.CmdModeIndentLine, Doc: "Indent the current line using the active mode.", AcceptsArg: false, Keys: []uint32{term.KeyTab}},
		{Name: "mode_mark_function", Fn: mode.CmdModeMarkFunction, Doc: "Mark the current function.", AcceptsArg: false, Keys: []uint32{term.META | term.CTL | 'H'}},
		{Name: "mode_newline_and_indent", Fn: CmdModeNewlineAndIndent, Doc: "Insert a newline using the active mode.", AcceptsArg: true, Keys: []uint32{term.KeyEnter, '\r', '\n'}},
		{Name: "mode_top_of_function", Fn: mode.CmdModeTopOfFunction, Doc: "Jump to the start of the current function.", AcceptsArg: false, Keys: []uint32{term.META | term.CTL | 'A'}},
		{Name: "mouse_drag", Fn: CmdMouseDrag, Doc: "Extend the selection with the mouse.", AcceptsArg: false, Keys: []uint32{term.MouseDrag}},
		{Name: "mouse_left", Fn: CmdMouseLeft, Doc: "Move point with the mouse.", AcceptsArg: false, Keys: []uint32{term.MouseLeft}},
		{Name: "mouse_wheel_down", Fn: CmdMouseWheelDown, Doc: "Scroll down with the mouse wheel.", AcceptsArg: false, Keys: []uint32{term.MouseWheelDown}},
		{Name: "mouse_wheel_up", Fn: CmdMouseWheelUp, Doc: "Scroll up with the mouse wheel.", AcceptsArg: false, Keys: []uint32{term.MouseWheelUp}},
		{Name: "open_line", Fn: CmdOpenLine, Doc: "Open a blank line after point.", AcceptsArg: true, Keys: []uint32{term.CTL | 'O'}},
		{Name: "pascal_case", Fn: CmdPascalCase, Doc: "Convert the identifier at point to PascalCase.", AcceptsArg: false},
		{Name: "query_re_replace", Fn: cmd0(CmdQueryReReplace), Doc: "Query replace with a regular expression.", AcceptsArg: false},
		{Name: "query_replace", Fn: cmd0(CmdQueryReplace), Doc: "Query replace plain text.", AcceptsArg: false, Keys: []uint32{term.META | '%'}},
		{Name: "quit", Fn: CmdQuit, Doc: "Quit Jem.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'C'}},
		{Name: "quote", Fn: CmdQuote, Doc: "Insert the next character literally.", AcceptsArg: true, Keys: []uint32{term.CTL | 'Q'}},
		{Name: "refresh", Fn: CmdRefresh, Doc: "Refresh the current window.", AcceptsArg: false, Keys: []uint32{term.CTL | 'L'}},
		{Name: "revert_file", Fn: CmdRevertFile, Doc: "Revert the current buffer to the on-disk file.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'V'}},
		{Name: "search_backward", Fn: cmd0(CmdSearchBackward), Doc: "Search backward for a string.", AcceptsArg: false},
		{Name: "search_forward", Fn: cmd0(CmdSearchForward), Doc: "Search forward for a string.", AcceptsArg: false},
		{Name: "set_eol_mode", Fn: CmdSetEolMode, Doc: "Set the current buffer line ending mode.", AcceptsArg: false},
		{Name: "set_mark", Fn: CmdSetMark, Doc: "Set the mark.", AcceptsArg: false, Keys: []uint32{term.CTL | ' '}},
		{Name: "set_variable", Fn: CmdSetVariable, Doc: "Set a named editor variable.", AcceptsArg: false},
		{Name: "show_position", Fn: CmdShowPosition, Doc: "Show the current cursor position.", AcceptsArg: false, Keys: []uint32{term.CTLX | '='}},
		{Name: "snake_case", Fn: CmdSnakeCase, Doc: "Convert the identifier at point to snake_case.", AcceptsArg: false},
		{Name: "sort_region", Fn: CmdSortRegion, Doc: "Sort the active region by lines.", AcceptsArg: false},
		{Name: "spawn", Fn: cmd0(CmdSpawn), Doc: "Run a shell command.", AcceptsArg: false, Keys: []uint32{term.CTLX | '!'}},
		{Name: "spawn_cli", Fn: cmd0(tools.RunSpawnCLI), Doc: "Open an interactive shell.", AcceptsArg: false, Keys: []uint32{term.META | '!'}},
		{Name: "swap_mark", Fn: CmdSwapMark, Doc: "Swap point and mark.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'X'}},
		{Name: "theme_toggle", Fn: CmdThemeToggle, Doc: "Toggle between dark and light themes.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'T'}},
		{Name: "toggle_search_scope", Fn: cmd0(search.ToggleSearchScope), Doc: "Toggle search between one buffer and all buffers.", AcceptsArg: false},
		{Name: "transpose_chars", Fn: CmdTransposeChars, Doc: "Transpose the characters around point.", AcceptsArg: false, Keys: []uint32{term.CTL | 'T'}},
		{Name: "transpose_lines", Fn: CmdTransposeLines, Doc: "Transpose the current line with the line above it.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'T'}},
		{Name: "transpose_words", Fn: CmdTransposeWords, Doc: "Transpose the words around point.", AcceptsArg: false, Keys: []uint32{term.META | 'T'}},
		{Name: "trim_whitespace", Fn: CmdTrimWhitespace, Doc: "Trim surrounding whitespace.", AcceptsArg: false, Keys: []uint32{term.META | '\\'}},
		{Name: "undo", Fn: CmdUndo, Doc: "Undo the most recent editing command.", AcceptsArg: true, Keys: []uint32{term.CTL | 'Z'}},
		{Name: "upper_region", Fn: CmdUpperRegion, Doc: "Uppercase the active region.", AcceptsArg: false, Keys: []uint32{term.CTLX | term.CTL | 'U'}},
		{Name: "upper_word", Fn: CmdUpperWord, Doc: "Uppercase the next word.", AcceptsArg: true, Keys: []uint32{term.META | 'U'}},
		{Name: "use_buffer", Fn: CmdUseBuffer, Doc: "Switch to another buffer.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'B'}},
		{Name: "use_buffer_fuzzy", Fn: CmdUseBuffer, Doc: "Switch to another buffer with fuzzy matching.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'B'}},
		{Name: "window_delete", Fn: CmdWindowDelete, Doc: "Delete the current window.", AcceptsArg: false, Keys: []uint32{term.CTLX | '0'}},
		{Name: "window_next", Fn: CmdWindowNext, Doc: "Select the next window.", AcceptsArg: false, Keys: []uint32{term.CTLX | 'O'}},
		{Name: "window_only", Fn: CmdWindowOnly, Doc: "Make the current window the only window.", AcceptsArg: false, Keys: []uint32{term.CTLX | '1'}},
		{Name: "window_split", Fn: CmdWindowSplit, Doc: "Split the current window.", AcceptsArg: false, Keys: []uint32{term.CTLX | '2'}},
		{Name: "yank", Fn: CmdYank, Doc: "Yank the most recently killed text.", AcceptsArg: true, Keys: []uint32{term.CTL | 'Y'}},
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

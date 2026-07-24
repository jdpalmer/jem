package runtime

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/markring"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/search"
	"github.com/jdpalmer/jem/tools"
	"github.com/jdpalmer/jem/window"
)

// CmdSearchForward prompts for a pattern then searches forward.
func CmdSearchForward() bool {
	if window.Active.CurrentWindow == nil || buffer.All.Current == nil {
		return false
	}
	AskString(search.SearchPromptLabel("Search"), search.DefaultState.SearchPattern, func(pattern string, pr minibuffer.PromptResult) {
		if !search.AcceptPromptedPattern(pattern, pr) {
			return
		}
		search.SearchForward()
	})
	return true
}

// CmdSearchBackward prompts for a pattern then searches backward.
func CmdSearchBackward() bool {
	if window.Active.CurrentWindow == nil || buffer.All.Current == nil {
		return false
	}
	AskString(search.SearchPromptLabel("Reverse search"), search.DefaultState.SearchPattern, func(pattern string, pr minibuffer.PromptResult) {
		if !search.AcceptPromptedPattern(pattern, pr) {
			return
		}
		search.SearchBackward()
	})
	return true
}

// CmdQueryReplace prompts for pattern and replacement, then starts query-replace.
func CmdQueryReplace() bool {
	if window.Active.CurrentWindow == nil || buffer.All.Current == nil {
		return false
	}
	AskString(search.SearchPromptLabel("replace"), search.DefaultState.SearchPattern, func(pattern string, pr minibuffer.PromptResult) {
		if !search.AcceptPromptedPattern(pattern, pr) {
			return
		}
		if search.DefaultState.SearchPattern == "" {
			return
		}
		AskString("Replace '"+search.DefaultState.SearchPattern+"' with: ", "", func(repl string, pr minibuffer.PromptResult) {
			if pr == minibuffer.PromptResultAbort {
				return
			}
			if s := search.StartQueryReplace(repl); s != nil {
				PushKeySession(s)
			}
		})
	})
	return true
}

// CmdQueryReReplace prompts for a regex pattern and replacement, then starts query-replace.
func CmdQueryReReplace() bool {
	if window.Active.CurrentWindow == nil || buffer.All.Current == nil {
		return false
	}
	AskString(search.SearchPromptLabel("Query re-replace"), search.DefaultState.SearchPattern, func(pattern string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes || pattern == "" {
			return
		}
		AskString("Replace '"+pattern+"' with (\\0..\\9): ", "", func(replStr string, pr minibuffer.PromptResult) {
			if pr == minibuffer.PromptResultAbort {
				return
			}
			if s := search.StartQueryReReplace(pattern, replStr); s != nil {
				PushKeySession(s)
			}
		})
	})
	return true
}

// CmdGrep prompts then runs a project grep.
func CmdGrep() bool {
	AskString("grep: ", "", func(pattern string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		tools.GrepWithPattern(pattern)
	})
	return true
}

// CmdCompile prompts then runs a build command.
func CmdCompile() bool {
	AskString("Compile: ", tools.LastCompileCommand(), func(command string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		tools.CompileWithCommand(command)
	})
	return true
}

// CmdSpawn prompts then runs a one-line shell command.
func CmdSpawn() bool {
	AskStringCap(tools.SpawnPrompt(), "", tools.CommandPromptCapacity, func(command string, pr minibuffer.PromptResult) {
		_ = tools.RunSpawnAfterPrompt(command, pr)
	})
	return true
}

// CmdGotoTag jumps to a tag definition, prompting when needed.
func CmdGotoTag() bool {
	if !tools.EnsureTagsLoaded(false) {
		return false
	}
	finish := func(name string) {
		matches := tools.CollectTagMatches(name)
		if len(matches) == 0 {
			display.MBWrite("[tag not found: %s]", name)
			return
		}
		if len(matches) == 1 {
			visitTagMatch(matches, 0)
			return
		}
		count := tools.TagMatchCount(matches)
		slice := matches[:count]
		AskFuzzyEx("Tag: ", tools.TagMatchProvider, slice, count, tools.TagMatchFormatter, slice, func(selected string, r minibuffer.PromptResult) {
			if r == minibuffer.PromptResultAbort {
				CmdAbort(false, 1)
				return
			}
			if r != minibuffer.PromptResultYes {
				return
			}
			if i := tools.IndexOfTagName(slice, selected); i >= 0 {
				visitTagMatch(slice, i)
			}
		})
	}
	if name, ok := tools.SymbolAtPoint(); ok {
		finish(name)
		return true
	}
	AskString("Goto tag: ", "", func(symbol string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		finish(symbol)
	})
	return true
}

func visitTagMatch(matches []*tools.TagEntry, choice int) {
	path, line, ok := tools.TagMatchLocation(matches, choice)
	if !ok {
		return
	}
	markring.PushCurrent()
	_ = fileVisitLocation(path, line, 1)
}

func pushKeySessionCmd(s search.KeySession) bool {
	if s == nil {
		return false
	}
	PushKeySession(s)
	return true
}

func CmdIsearchForward() bool  { return pushKeySessionCmd(search.IsearchForward()) }
func CmdIsearchBackward() bool { return pushKeySessionCmd(search.IsearchBackward()) }
func CmdIsearchReForward() bool {
	return pushKeySessionCmd(search.IsearchReForward())
}
func CmdIsearchReBackward() bool {
	return pushKeySessionCmd(search.IsearchReBackward())
}

func CmdGrepVisitMatch() bool {
	path, line, col, ok := tools.GrepMatchAtPoint()
	if !ok {
		return false
	}
	markring.PushCurrent()
	return fileVisitLocation(path, line, col)
}

func CmdCompileVisitDiag() bool {
	path, line, col, ok := tools.CompileDiagAtPoint()
	if !ok {
		return false
	}
	markring.PushCurrent()
	return fileVisitLocation(path, line, col)
}

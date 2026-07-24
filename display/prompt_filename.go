package display

import (
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
	"sort"
	"strings"

	"github.com/jdpalmer/jem/file"
	"github.com/jdpalmer/jem/term"
)

// FilenamePrompt is a filename picker with tab completion / fuzzy matches.
type FilenamePrompt struct {
	prompt         string
	state          minibuffer.MinibufferState
	filePaths      []string
	matchRoot      string
	currentDirPart string
	matchIndices   []int
	lastQuery      string
	sel            int
	programmatic   bool
}

// NewFilenamePrompt builds a filename prompt. initial may be empty.
func NewFilenamePrompt(prompt, initial string, capacity int) *FilenamePrompt {
	if capacity <= 0 {
		capacity = file.PromptPathCapacity
	}
	p := &FilenamePrompt{
		prompt: prompt,
		state: minibuffer.MinibufferState{
			Text:       make([]byte, 0, capacity),
			Nbuf:       capacity,
			HistoryPos: -1,
		},
	}
	if initial != "" {
		p.state.SetText([]byte(initial))
	}
	p.refreshList(".")
	p.syncMatches()
	return p
}

// Open shows the prompt for listener-driven use.
func (p *FilenamePrompt) Open() {
	minibuffer.Active = &p.state
	p.redraw()
}

// Close tears down the prompt UI.
func (p *FilenamePrompt) Close() {
	minibuffer.Active = nil
	window.HideMatchWindow()
	DisplayUpdate()
}

func (p *FilenamePrompt) refreshList(dir string) {
	if dir == p.matchRoot {
		return
	}
	fp := collectFuzzyPaths(dir, "")
	if len(fp) > 0 && fp[0] == "../" {
		sort.Strings(fp[1:])
	} else {
		sort.Strings(fp)
	}
	p.filePaths = fp
	p.matchRoot = dir
}

func (p *FilenamePrompt) syncMatches() {
	query := string(p.state.Text)
	queryChanged := !p.programmatic && query != p.lastQuery
	p.lastQuery = query

	if query == "~" {
		p.currentDirPart = ""
		p.matchIndices = nil
		if queryChanged {
			p.sel = 0
		}
		return
	}
	dirPart, pattern := file.PromptSplit(query)
	p.currentDirPart = dirPart
	p.refreshList(file.OpenDirFromPrompt(dirPart))

	const maxMatches = 16
	if pattern == "" {
		n := len(p.filePaths)
		if n > maxMatches {
			n = maxMatches
		}
		p.matchIndices = make([]int, n)
		for i := range p.matchIndices {
			p.matchIndices[i] = i
		}
	} else {
		p.matchIndices = filenameFuzzyMatches(p.filePaths, pattern, maxMatches)
	}
	if queryChanged {
		p.sel = 0
	} else if p.sel >= len(p.matchIndices) {
		p.sel = 0
	}
}

func (p *FilenamePrompt) applyMatchSelection() string {
	if len(p.matchIndices) == 0 || p.sel >= len(p.matchIndices) {
		return string(p.state.Text)
	}
	selected := p.filePaths[p.matchIndices[p.sel]]
	return file.ApplyFilenameSelection(p.currentDirPart, selected)
}

func (p *FilenamePrompt) setPromptText(text string) {
	p.programmatic = true
	p.state.SetText([]byte(text))
	p.programmatic = false
	p.lastQuery = text
}

func (p *FilenamePrompt) fpProvider(ctx any, idx int) []byte {
	paths := ctx.([]string)
	if int(idx) >= len(paths) {
		return nil
	}
	return []byte(paths[int(idx)])
}

func (p *FilenamePrompt) redraw() {
	if len(p.matchIndices) > 0 {
		fctx := &fuzzyMatchCtx{
			provider:    p.fpProvider,
			providerCtx: p.filePaths,
		}
		fuzzyListRedraw(p.prompt, &p.state, fctx, p.matchIndices, p.sel)
		return
	}
	window.HideMatchWindow()
	DisplayUpdate()
	MBWritePrompt(promptFormatWithCount(p.prompt, p.sel, len(p.matchIndices)), p.state.Text, p.state.CursorPos)
}

// HandleKey applies one key. On success, text is the chosen path.
func (p *FilenamePrompt) HandleKey(k uint32) (done bool, text string, pr minibuffer.PromptResult) {
	changed := false

	switch {
	case k == term.KeyEnter || k == '\r' || k == '\n' || k == (term.CTL|'M') || k == (term.CTL|'J'):
		full := p.applyMatchSelection()
		if len(p.matchIndices) > 0 && p.sel < len(p.matchIndices) {
			selected := p.filePaths[p.matchIndices[p.sel]]
			if selected == "../" || strings.HasSuffix(selected, "/") {
				p.setPromptText(full)
				p.syncMatches()
				p.redraw()
				return false, "", 0
			}
			p.setPromptText(full)
		}
		MBHistoryAdd(string(p.state.Text))
		maybeRecordMinibufferResult(p.state.Text)
		MBClear()
		return true, string(p.state.Text), minibuffer.PromptResultYes

	case k == (term.CTL|'G') || k == 0x07 || k == 0x1B:
		MBClear()
		return true, "", minibuffer.PromptResultAbort

	case k == term.KeyTab:
		if len(p.matchIndices) > 0 && p.sel < len(p.matchIndices) {
			p.setPromptText(p.applyMatchSelection())
			p.syncMatches()
			changed = true
		} else if completePromptFilename(&p.state) {
			p.lastQuery = string(p.state.Text)
			p.syncMatches()
			changed = true
		} else {
			term.Beep()
		}

	case k == term.KeyUp || k == (term.CTL|'P'):
		if len(p.matchIndices) == 0 {
			term.Beep()
		} else {
			p.sel = (p.sel + len(p.matchIndices) - 1) % len(p.matchIndices)
			p.setPromptText(p.applyMatchSelection())
			p.syncMatches()
			changed = true
		}

	case k == term.KeyDown || k == (term.CTL|'N'):
		if len(p.matchIndices) == 0 {
			term.Beep()
		} else {
			p.sel = (p.sel + 1) % len(p.matchIndices)
			p.setPromptText(p.applyMatchSelection())
			p.syncMatches()
			changed = true
		}

	default:
		var handled bool
		handled, changed = promptLineEditKey(&p.state, k)
		if !handled {
			term.Beep()
		}
	}

	if changed {
		p.syncMatches()
	}
	p.redraw()
	return false, "", 0
}

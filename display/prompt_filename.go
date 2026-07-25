package display

import (
	"sort"
	"strings"

	"github.com/jdpalmer/jem/file"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/term"
	"github.com/jdpalmer/jem/window"
)

// FilenamePrompt is a filename picker: typed text filters the match list
// (like the command palette); Tab completes, Enter accepts the selection.
type FilenamePrompt struct {
	prompt         string
	state          minibuffer.MinibufferState
	matchCtx       *filenameMatchCtx
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
		matchCtx: &filenameMatchCtx{},
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
	window.DiscardMatchBuffer()
	DisplayUpdate()
}

func (p *FilenamePrompt) refreshList(dir string) {
	if dir == p.matchRoot {
		return
	}
	entries := collectFuzzyPaths(dir, "")
	if len(entries) > 0 && entries[0].Name == "../" {
		sort.Slice(entries[1:], func(i, j int) bool {
			return entries[i+1].Name < entries[j+1].Name
		})
	} else {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name < entries[j].Name
		})
	}
	p.matchCtx = newFilenameMatchCtx(entries)
	p.matchRoot = dir
}

func (p *FilenamePrompt) entryName(idx int) string {
	if p.matchCtx == nil || idx < 0 || idx >= len(p.matchCtx.entries) {
		return ""
	}
	return p.matchCtx.entries[idx].Name
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

	entries := p.matchCtx.entries
	const maxMatches = fuzzyMaxMatches
	if pattern == "" {
		n := len(entries)
		if n > maxMatches {
			n = maxMatches
		}
		p.matchIndices = make([]int, n)
		for i := range p.matchIndices {
			p.matchIndices[i] = i
		}
	} else {
		p.matchIndices = filenameFuzzyMatches(entries, pattern, maxMatches)
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
	selected := p.entryName(p.matchIndices[p.sel])
	return file.ApplyFilenameSelection(p.currentDirPart, selected)
}

func (p *FilenamePrompt) setPromptText(text string) {
	p.programmatic = true
	p.state.SetText([]byte(text))
	p.programmatic = false
}

func (p *FilenamePrompt) redraw() {
	if len(p.matchIndices) > 0 {
		fuzzyMatchRefresh(p.matchIndices, p.sel, &fuzzyMatchCtx{
			provider:         filenameProvider,
			providerCtx:      p.matchCtx,
			displayFormatter: filenameMatchFormatter,
			displayCtx:       p.matchCtx,
		})
	} else {
		window.DiscardMatchBuffer()
		DisplayUpdate()
	}
	MBWritePrompt(p.prompt, p.state.Text, p.state.CursorPos)
}

// HandleKey applies one key. On success, text is the chosen path.
func (p *FilenamePrompt) HandleKey(k uint32) (done bool, text string, pr minibuffer.PromptResult) {
	changed := false

	switch {
	case k == term.KeyEnter || k == '\r' || k == '\n' || k == (term.CTL|'M') || k == (term.CTL|'J'):
		full := p.applyMatchSelection()
		if len(p.matchIndices) > 0 && p.sel < len(p.matchIndices) {
			selected := p.entryName(p.matchIndices[p.sel])
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
		}

	case k == term.KeyDown || k == (term.CTL|'N'):
		if len(p.matchIndices) == 0 {
			term.Beep()
		} else {
			p.sel = (p.sel + 1) % len(p.matchIndices)
		}

	case k == (term.SHIFT|term.KeyUp) || k == term.KeyPageUp ||
		k == (term.SHIFT|term.KeyDown) || k == term.KeyPageDown:
		if len(p.matchIndices) == 0 {
			term.Beep()
		} else {
			delta := matchListPageSize()
			if k == (term.SHIFT|term.KeyUp) || k == term.KeyPageUp {
				delta = -delta
			}
			prev := p.sel
			p.sel = matchListMoveSel(p.sel, len(p.matchIndices), delta)
			if p.sel == prev {
				term.Beep()
			}
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

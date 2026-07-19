package view

import (
	"sort"
	"strings"

	"github.com/jdpalmer/jem/fileio"
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/term"
)

// FilenamePrompt is a filename picker with tab completion / fuzzy matches.
type FilenamePrompt struct {
	prompt         string
	state          model.MinibufferState
	filePaths      []string
	matchRoot      string
	currentDirPart string
	matchIndices   []uint
	lastQuery      string
	sel            int
	programmatic   bool
	async          bool
}

// NewFilenamePrompt builds a filename prompt. initial may be empty.
func NewFilenamePrompt(prompt, initial string, capacity int) *FilenamePrompt {
	if capacity <= 0 {
		capacity = fileio.PromptPathCapacity
	}
	p := &FilenamePrompt{
		prompt: prompt,
		state: model.MinibufferState{
			Prompt:     prompt,
			Text:       make([]byte, 0, capacity),
			Nbuf:       uint(capacity),
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

// OpenAsync shows the prompt for listener-driven use.
func (p *FilenamePrompt) OpenAsync() {
	p.async = true
	ShowMinibuffer(&p.state)
	p.redraw()
}

// OpenBlocking shows the prompt with nested key capture.
func (p *FilenamePrompt) OpenBlocking() {
	p.async = false
	ActivateMinibuffer(&p.state)
	p.redraw()
}

// Close tears down the prompt UI.
func (p *FilenamePrompt) Close() {
	if p.async {
		HideMinibuffer()
	} else {
		DeactivateMinibuffer()
	}
	model.HideMatchWindow()
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
	dirPart, pattern := fileio.PromptSplit(query)
	p.currentDirPart = dirPart
	p.refreshList(fileio.OpenDirFromPrompt(dirPart))

	const maxMatches = 16
	if pattern == "" {
		n := len(p.filePaths)
		if n > maxMatches {
			n = maxMatches
		}
		p.matchIndices = make([]uint, n)
		for i := range p.matchIndices {
			p.matchIndices[i] = uint(i)
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
	return fileio.ApplyFilenameSelection(p.currentDirPart, selected)
}

func (p *FilenamePrompt) setPromptText(text string) {
	p.programmatic = true
	p.state.SetText([]byte(text))
	p.programmatic = false
	p.lastQuery = text
}

func (p *FilenamePrompt) fpProvider(ctx any, idx uint) []byte {
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
	model.HideMatchWindow()
	DisplayUpdate()
	MBWritePrompt(promptFormatWithCount(p.prompt, p.sel, len(p.matchIndices)), p.state.Text, int(p.state.CursorPos))
}

// HandleKey applies one key. On success, text is the chosen path.
func (p *FilenamePrompt) HandleKey(k uint32) (done bool, text string, pr model.PromptResult) {
	if IsPasteRedrawKey(k) {
		DisplayUpdate()
		p.redraw()
		return false, "", 0
	}

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
		if PackageHooks.MacroRecordMinibufferResult != nil {
			PackageHooks.MacroRecordMinibufferResult(p.state.Text)
		}
		MBClear()
		return true, string(p.state.Text), model.PromptResultYes

	case k == (term.CTL|'G') || k == 0x07 || k == 0x1B:
		MBClear()
		return true, "", model.PromptResultAbort

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

	case k == (term.CTL|'A') || k == term.KeyHome:
		if !p.state.GotoBol() {
			term.Beep()
		}
	case k == (term.CTL|'E') || k == term.KeyEnd:
		if !p.state.GotoEol() {
			term.Beep()
		}
	case k == (term.CTL|'B') || k == term.KeyLeft:
		if !p.state.BackwardChar() {
			term.Beep()
		}
	case k == (term.CTL|'F') || k == term.KeyRight:
		if !p.state.ForwardChar() {
			term.Beep()
		}
	case k == (term.META|'B') || k == (term.SHIFT|term.KeyLeft):
		if !p.state.BackwardWord() {
			term.Beep()
		}
	case k == (term.META|'F') || k == (term.SHIFT|term.KeyRight):
		if !p.state.ForwardWord() {
			term.Beep()
		}

	case k == 0x7F || k == (term.CTL|'H'):
		changed = p.state.DeleteBackward()
		if !changed {
			term.Beep()
		}
	case k == (term.CTL|'D') || k == term.KeyDelete:
		changed = p.state.DeleteForward()
		if !changed {
			term.Beep()
		}
	case k == (term.CTL | 'U'):
		changed = p.state.ClearText()
		if !changed {
			term.Beep()
		}
	case k == (term.CTL | 'K'):
		changed = p.state.Kill()
		if !changed {
			term.Beep()
		}
	case k == (term.META | 'D'):
		changed = p.state.DeleteWordForward()
		if !changed {
			term.Beep()
		}
	case k == (term.META|'H') || k == (term.META|0x7F):
		changed = p.state.DeleteWordBackward()
		if !changed {
			term.Beep()
		}

	default:
		if k < term.UnicodeLimit && k >= 0x20 && (k&term.KeyMask) == 0 {
			if p.state.InsertChar(rune(k)) {
				changed = true
			} else {
				term.Beep()
			}
		} else {
			term.Beep()
		}
	}

	if changed {
		p.syncMatches()
	}
	p.redraw()
	return false, "", 0
}

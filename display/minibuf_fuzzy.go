package display

// Fuzzy-list prompt matching and redraw.

import (
	"bytes"
	"sort"
	"strings"

	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
)

// fuzzyMaxMatches caps ranked fuzzy results shown in the *match* window.
// Large enough to list the full command palette; the window scrolls as needed.
const fuzzyMaxMatches = 256

type fuzzyMatchCtx struct {
	provider         minibuffer.MbNameProviderFn
	providerCtx      any
	displayFormatter minibuffer.MbMatchFormatter
	displayCtx       any
}

func fuzzyMatchFormatLine(ctx *fuzzyMatchCtx, matches []int, out []byte, outSize int, listIdx int) {
	if listIdx >= len(matches) {
		return
	}
	provIdx := matches[listIdx]
	if ctx.displayFormatter != nil {
		ctx.displayFormatter(out, outSize, provIdx, ctx.displayCtx)
		return
	}
	if ctx.provider == nil {
		return
	}
	name := ctx.provider(ctx.providerCtx, provIdx)
	if name == nil {
		return
	}
	n := len(name)
	if outSize <= 0 {
		return
	}
	if n >= outSize {
		n = outSize - 1
	}
	copy(out, name[:n])
	out[n] = 0
}

func writeMatchBufferGeneric(formatter minibuffer.MbMatchFormatter, ctx any, count int, selected int) {
	if count == 0 {
		window.SetMatchBufferText(nil, 0)
		DisplayUpdate()
		return
	}

	var out strings.Builder
	for i := 0; i < count; i++ {
		line := make([]byte, 1024)
		formatter(line, len(line), i, ctx)
		end := bytes.IndexByte(line, 0)
		if end < 0 {
			end = len(line)
		}
		if i > 0 {
			out.WriteByte('\n')
		}
		if i == selected {
			out.WriteString("→ ")
		} else {
			out.WriteString("  ")
		}
		out.Write(line[:end])
	}

	window.SetMatchBufferText([]byte(out.String()), selected)
	DisplayUpdate()
}

func fuzzyMatchRefresh(matches []int, sel int, ctx *fuzzyMatchCtx) {
	count := len(matches)
	if count > fuzzyMaxMatches {
		count = fuzzyMaxMatches
	}
	if count == 0 {
		window.SetMatchBufferText(nil, 0)
		DisplayUpdate()
		return
	}
	if sel < 0 {
		sel = 0
	}
	if sel >= count {
		sel = count - 1
	}
	writeMatchBufferGeneric(func(out []byte, outSize int, idx int, c any) {
		fuzzyMatchFormatLine(c.(*fuzzyMatchCtx), matches, out, outSize, idx)
	}, ctx, count, sel)
}

func fuzzyListRedraw(prompt string, state *minibuffer.MinibufferState, ctx *fuzzyMatchCtx, matches []int, sel int) {
	fuzzyMatchRefresh(matches, sel, ctx)
	MBWritePrompt(promptFormatWithCount(prompt, sel, len(matches)), state.Text, state.CursorPos)
}

// matchListPageSize is how many rows Shift-Up/Down moves in the *match* list.
func matchListPageSize() int {
	if mw := window.MatchWindow(); mw != nil && mw.Height > 1 {
		return mw.Height
	}
	return 10
}

// matchListMoveSel moves sel by delta within [0, count). Clamps at the ends.
// Returns the new selection index.
func matchListMoveSel(sel, count, delta int) int {
	if count <= 0 {
		return 0
	}
	sel += delta
	if sel < 0 {
		return 0
	}
	if sel >= count {
		return count - 1
	}
	return sel
}

// ---- Fuzzy list prompt (generic) --------------------------------------------

type fuzzyEntry struct {
	idx   int
	score int
}

// fuzzyTopN ranks items by score descending and returns up to maxMatches indices.
// scoreAt returns (matched, score); unmatched items are skipped.
// tieLess breaks equal scores (true if index a should rank before b); nil uses lower index.
func fuzzyTopN(count, maxMatches int, scoreAt func(i int) (bool, int), tieLess func(a, b int) bool) []int {
	if count == 0 || maxMatches <= 0 {
		return nil
	}
	matches := make([]fuzzyEntry, 0, maxMatches)
	for i := 0; i < count; i++ {
		ok, sc := scoreAt(i)
		if !ok {
			continue
		}
		matches = append(matches, fuzzyEntry{idx: i, score: sc})
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(a, b int) bool {
		if matches[a].score != matches[b].score {
			return matches[a].score > matches[b].score
		}
		if tieLess != nil {
			return tieLess(matches[a].idx, matches[b].idx)
		}
		return matches[a].idx < matches[b].idx
	})
	n := len(matches)
	if n > maxMatches {
		n = maxMatches
	}
	out := make([]int, n)
	for i := 0; i < n; i++ {
		out[i] = matches[i].idx
	}
	return out
}

// fuzzyScore computes a fuzzy match score for name against query.
// Returns (matched, score); higher score is better.
func fuzzyScore(name, query []byte) (bool, int) {
	if len(query) == 0 {
		return true, 1
	}
	n := len(name)
	q := len(query)
	ni := 0
	prev := -1
	totalGap := 0
	consecBonus := 0
	matched := 0
	for qi := 0; qi < q; qi++ {
		qc := query[qi]
		found := -1
		for ni < n {
			nc := name[ni]
			if nc >= 'A' && nc <= 'Z' {
				nc = nc - 'A' + 'a'
			}
			cc := qc
			if cc >= 'A' && cc <= 'Z' {
				cc = cc - 'A' + 'a'
			}
			if nc == cc {
				found = ni
				ni++
				break
			}
			ni++
		}
		if found == -1 {
			return false, 0
		}
		if prev != -1 {
			gap := found - prev - 1
			totalGap += gap
			if gap == 0 {
				consecBonus += 5
			}
		}
		prev = found
		matched++
	}
	score := matched*100 - totalGap*5 + consecBonus
	if prev >= 0 && prev < 3 {
		score += 20
	}
	return true, score
}

// fuzzyMatches returns up to maxMatches indices from provider that best match
// query, ordered by score descending.
func fuzzyMatches(provider minibuffer.MbNameProviderFn, ctx any, count int, query []byte, maxMatches int) []int {
	return fuzzyTopN(count, maxMatches, func(i int) (bool, int) {
		name := provider(ctx, i)
		if name == nil {
			return false, 0
		}
		return fuzzyScore(name, query)
	}, nil)
}

package view

// minibuf.go - Minibuffer input prompts and feedback (Go port of src/minibuffer.c)

import (
	"sort"
	"strings"

	"github.com/jdpalmer/jem/model"
)

const fuzzyMaxMatches = 16

type fuzzyMatchCtx struct {
	provider         model.MbNameProviderFn
	providerCtx      any
	displayFormatter model.MbMatchFormatter
	displayCtx       any
	indices          []uint
}

func fuzzyMatchFormatLine(ctx *fuzzyMatchCtx, out []byte, outSize uint, listIdx uint) {
	if int(listIdx) >= len(ctx.indices) {
		return
	}
	provIdx := ctx.indices[listIdx]
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
	if uint(n) >= outSize {
		n = int(outSize) - 1
	}
	if n < 0 {
		n = 0
	}
	copy(out, name[:n])
	out[n] = 0
}

func writeMatchBufferGeneric(formatter model.MbMatchFormatter, ctx any, count uint, selected uint) {
	if count == 0 {
		model.SetMatchBufferText(nil, 0)
		DisplayUpdate()
		return
	}

	var out strings.Builder
	for i := uint(0); i < count; i++ {
		line := make([]byte, 512)
		formatter(line, uint(len(line)), i, ctx)
		end := 0
		for end < len(line) && line[end] != 0 {
			end++
		}
		if i == selected {
			out.WriteString("> ")
		} else {
			out.WriteString("  ")
		}
		out.Write(line[:end])
		out.WriteByte('\n')
	}

	model.SetMatchBufferText([]byte(out.String()), selected)
	DisplayUpdate()
}

func fuzzyMatchRefresh(matches []uint, sel int, ctx *fuzzyMatchCtx) {
	ctx.indices = matches
	count := uint(len(matches))
	if count > fuzzyMaxMatches {
		count = fuzzyMaxMatches
	}
	if count == 0 {
		writeMatchBufferGeneric(func([]byte, uint, uint, any) {}, ctx, 0, 0)
		return
	}
	if sel < 0 {
		sel = 0
	}
	if uint(sel) >= count {
		sel = int(count) - 1
	}
	writeMatchBufferGeneric(func(out []byte, outSize uint, idx uint, c any) {
		fuzzyMatchFormatLine(c.(*fuzzyMatchCtx), out, outSize, idx)
	}, ctx, count, uint(sel))
}

func fuzzyListRedraw(prompt string, state *model.MinibufferState, ctx *fuzzyMatchCtx, matches []uint, sel int) {
	fuzzyMatchRefresh(matches, sel, ctx)
	MBWritePrompt(promptFormatWithCount(prompt, sel, len(matches)), state.Text, int(state.CursorPos))
}

// ---- Fuzzy list prompt (generic) --------------------------------------------

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
func fuzzyMatches(provider model.MbNameProviderFn, ctx any, count uint, query []byte, maxMatches int) []uint {
	if count == 0 || maxMatches <= 0 {
		return nil
	}
	type entry struct {
		idx   uint
		score int
	}
	matches := make([]entry, 0, maxMatches)
	for i := uint(0); i < count; i++ {
		name := provider(ctx, i)
		if name == nil {
			continue
		}
		ok, sc := fuzzyScore(name, query)
		if !ok {
			continue
		}
		matches = append(matches, entry{idx: i, score: sc})
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(a, b int) bool {
		if matches[a].score != matches[b].score {
			return matches[a].score > matches[b].score
		}
		return matches[a].idx < matches[b].idx
	})
	n := len(matches)
	if n > maxMatches {
		n = maxMatches
	}
	out := make([]uint, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, matches[i].idx)
	}
	return out
}

// mbReadFuzzyListEx prompts the user with a live-filtering fuzzy list (blocking).
func mbReadFuzzyListEx(prompt string, provider model.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter model.MbMatchFormatter, displayCtx any, buf []byte, nbuf int) model.PromptResult {
	if pr, played := macroPlayPrompt(buf); played {
		return pr
	}
	p := NewFuzzyPrompt(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx, nbuf)
	p.OpenBlocking()
	defer p.Close()
	for {
		k, ok := WaitKey()
		if !ok {
			return model.PromptResultAbort
		}
		done, text, pr := p.HandleKey(k)
		if done {
			if pr == model.PromptResultYes {
				n := copy(buf, text)
				if n < len(buf) {
					buf[n] = 0
				}
			}
			return pr
		}
	}
}

// mbReadFuzzyList is a convenience wrapper around mbReadFuzzyListEx with no
// custom display formatter.
func mbReadFuzzyList(prompt string, provider model.MbNameProviderFn, providerCtx any, providerCount uint, buf []byte, nbuf int) model.PromptResult {
	return mbReadFuzzyListEx(prompt, provider, providerCtx, providerCount, nil, nil, buf, nbuf)
}

package view

// minibuf.go - Minibuffer input prompts and feedback (Go port of src/minibuffer.c)

import (
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/term"
)

func macroPlayPrompt(buf []byte) (model.PromptResult, bool) {
	text, pr, playing := model.TakeMacroPromptReply()
	if !playing {
		return model.PromptResultAbort, false
	}
	if pr == model.PromptResultYes {
		n := copy(buf, text)
		if n < len(buf) {
			buf[n] = 0
		}
	}
	return pr, true
}

// ---- Horizontal choice menu (mb_choose) --------------------------------------

const (
	mlChoiceLeftWidth      = 2 // "… "
	mlChoiceRightWidth     = 3 // " …"  (space + ellipsis)
	mlChoiceSeparatorWidth = 2 // "  "
)

// mlChoiceVisibleWidth returns the total column width if choices [start..end]
// are displayed (including overflow indicators but not the leading prompt).
func mlChoiceVisibleWidth(ctx any, labelFn model.MLChoiceLabelFn, count, start, end int) int {
	w := 0
	if start > 0 {
		w += mlChoiceLeftWidth
	}
	for i := start; i <= end; i++ {
		if i > start {
			w += mlChoiceSeparatorWidth
		}
		w += len(labelFn(ctx, uint8(i)))
	}
	if end < count-1 {
		w += mlChoiceRightWidth
	}
	return w
}

// mlChoiceWindow computes the widest visible window of choices around selected
// that fits within avail columns, alternating right/left expansion.
func mlChoiceWindow(ctx any, labelFn model.MLChoiceLabelFn, count, selected, avail int) (start, end int) {
	start = selected
	end = selected
	chooseRight := true
	for {
		expanded := false
		r := end + 1
		l := start - 1
		if chooseRight && r < count {
			if mlChoiceVisibleWidth(ctx, labelFn, count, start, r) <= avail {
				end = r
				expanded = true
			}
		}
		if l >= 0 {
			if mlChoiceVisibleWidth(ctx, labelFn, count, l, end) <= avail {
				start = l
				expanded = true
			}
		}
		if !chooseRight && r < count {
			if mlChoiceVisibleWidth(ctx, labelFn, count, start, r) <= avail {
				end = r
				expanded = true
			}
		}
		if !expanded {
			break
		}
		chooseRight = !chooseRight
	}
	return
}

// mlChoiceRender renders the visible choice window on the message line and
// positions the cursor on the selected item.
func mlChoiceRender(prompt string, ctx any, labelFn model.MLChoiceLabelFn, count, start, end, selected int) {
	normalStyle := model.State.Theme.NormalStyle
	selStyle := model.State.Theme.PickerSelectionStyle
	maxcol := term.Cols() - 1
	col := 0
	selectedCol := 0

	mlBegin(normalStyle)

	if prompt != "" {
		pb := []byte(prompt)
		screenPutBytes(pb)
		col += displayWidthBytes(pb, len(pb))
	}

	if start > 0 && maxcol-col >= mlChoiceLeftWidth {
		screenPutBytes([]byte("\xe2\x80\xa6 ")) // "… "
		col += mlChoiceLeftWidth
	}

	for i := start; i <= end; i++ {
		label := labelFn(ctx, uint8(i))
		if i > start && maxcol-col >= mlChoiceSeparatorWidth {
			screenPutBytes([]byte("  "))
			col += mlChoiceSeparatorWidth
		}
		if i == selected {
			selectedCol = col
			screenSetStyle(selStyle)
		}
		screenPutBytes(label)
		col += displayWidthBytes(label, len(label))
		if i == selected {
			screenSetStyle(normalStyle)
		}
	}

	if end < count-1 && maxcol-col >= mlChoiceRightWidth {
		screenPutBytes([]byte("  \xe2\x80\xa6")) // "  …"
	}

	mlFinish(selectedCol, true)
}

// MBChoose presents a horizontal menu of count choices at the message line (blocking).
// Returns the selected index (≥0), -1 on Escape/cancel, or -2 on Ctrl-G abort.
func MBChoose(prompt string, ctx any, labelFn model.MLChoiceLabelFn, count uint8, defaultIdx uint8) int16 {
	p := NewChoosePrompt(prompt, ctx, labelFn, count, defaultIdx)
	if p == nil {
		return -1
	}
	p.OpenBlocking()
	defer p.Close()
	for {
		k, ok := WaitKey()
		if !ok {
			MBClear()
			return -1
		}
		done, sel := p.HandleKey(k)
		if done {
			return sel
		}
	}
}

// MBYesNo prompts the user for a yes/no answer using the horizontal choice menu.
func MBYesNo(prompt string) model.PromptResult {
	choices := [][]byte{[]byte("yes"), []byte("no")}
	labelFn := func(ctx any, idx uint8) []byte {
		sl := ctx.([][]byte)
		if int(idx) < len(sl) {
			return sl[int(idx)]
		}
		return nil
	}
	question := prompt
	if len(prompt) > 0 && prompt[len(prompt)-1] != ' ' {
		question = prompt + " "
	}
	choice := MBChoose(question, choices, labelFn, 2, 0)
	switch choice {
	case 0:
		return model.PromptResultYes
	case 1:
		return model.PromptResultNo
	default:
		return model.PromptResultAbort
	}
}

// ---- Filename prompt with tab completion and fuzzy matching ------------------

// shouldSkipFuzzyFile returns true for binary/derived files that clutter the

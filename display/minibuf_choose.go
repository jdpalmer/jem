package display

// Horizontal choice menu rendering for the message line.

import (
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/term"
)

// ---- Horizontal choice menu (mb_choose) --------------------------------------

const (
	mlChoiceLeftWidth      = 2 // "… "
	mlChoiceRightWidth     = 3 // " …"  (space + ellipsis)
	mlChoiceSeparatorWidth = 2 // "  "
)

// mlChoiceVisibleWidth returns the total column width if choices [start..end]
// are displayed (including overflow indicators but not the leading prompt).
func mlChoiceVisibleWidth(ctx any, labelFn minibuffer.MLChoiceLabelFn, count, start, end int) int {
	w := 0
	if start > 0 {
		w += mlChoiceLeftWidth
	}
	for i := start; i <= end; i++ {
		if i > start {
			w += mlChoiceSeparatorWidth
		}
		w += len(labelFn(ctx, i))
	}
	if end < count-1 {
		w += mlChoiceRightWidth
	}
	return w
}

// mlChoiceWindow computes the widest visible window of choices around selected
// that fits within avail columns, alternating right/left expansion.
func mlChoiceWindow(ctx any, labelFn minibuffer.MLChoiceLabelFn, count, selected, avail int) (start, end int) {
	start = selected
	end = selected
	chooseRight := true
	for {
		expanded := false
		r := end + 1
		l := start - 1
		for _, right := range [2]bool{chooseRight, !chooseRight} {
			if right {
				if r < count && mlChoiceVisibleWidth(ctx, labelFn, count, start, r) <= avail {
					end = r
					r = end + 1
					expanded = true
				}
			} else if l >= 0 && mlChoiceVisibleWidth(ctx, labelFn, count, l, end) <= avail {
				start = l
				l = start - 1
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
func mlChoiceRender(prompt string, ctx any, labelFn minibuffer.MLChoiceLabelFn, count, start, end, selected int) {
	normalStyle := Active.Theme.NormalStyle
	selStyle := Active.Theme.PickerSelectionStyle
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
		label := labelFn(ctx, i)
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

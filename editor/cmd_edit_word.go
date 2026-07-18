package editor

import (
	"strings"
	"unicode"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
)

// cmd_edit_word.go — word edits and character insert/delete

// delete forward word
func CmdDeleteWordForward(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	UndoBeginCommand()
	defer UndoEndCommand()
	for i := 0; i < n; i++ {
		start := wp.Cursor
		end := forwardWordLoc(bp, start)
		var newEnd buffer.Location
		if !bufferSetText(bp, start, end, nil, &newEnd, false) {
			return false
		}
		wp.Cursor = newEnd
	}
	wp.DidEdit = true
	return true
}

// delete backward word
// delete backward word
func CmdDeleteWordBackward(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	UndoBeginCommand()
	defer UndoEndCommand()
	for i := 0; i < n; i++ {
		end := wp.Cursor
		start := backwardWordLoc(bp, end)
		var newEnd buffer.Location
		if !bufferSetText(bp, start, end, nil, &newEnd, false) {
			return false
		}
		wp.Cursor = newEnd
	}
	wp.DidEdit = true
	return true
}

// helper: find start of next word from loc (skip non-word then return start)
// helper: find start of next word from loc (skip non-word then return start)
func nextWordStart(bp *buffer.Buffer, loc buffer.Location) buffer.Location {
	if bp == nil || loc.Line == 0 {
		return loc
	}
	line := bp.Line(loc.Line)
	if line == nil {
		return loc
	}
	off := int(loc.Offset)
	for ln := loc.Line; ln <= bp.LineCount; ln++ {
		line = bp.Line(ln)
		if line == nil {
			continue
		}
		for off < len(line.Data) && !isWordChar(line.Data[off]) {
			off++
		}
		if off < len(line.Data) {
			return buffer.Location{Line: ln, Offset: uint(off)}
		}
		off = 0
	}
	return buffer.Location{Line: bp.LineCount, Offset: 0}
}

// Case transformations on a word at point
// Case transformations on a word at point
func CmdLowerWord(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	// operate on word at point: find start and end
	start := backwardWordLoc(bp, wp.Cursor)
	end := forwardWordLoc(bp, start)
	text := bp.GetText(start, end)
	length := uint(len(text))
	if length == 0 || text == nil {
		return false
	}
	newText := []byte(strings.ToLower(string(text)))
	UndoBeginCommand()
	defer UndoEndCommand()
	var newEnd buffer.Location
	ok := bufferSetText(bp, start, end, newText, &newEnd, false)
	if ok {
		wp.Cursor = newEnd
		wp.DidEdit = true
	}
	return ok
}

func CmdUpperWord(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	start := backwardWordLoc(bp, wp.Cursor)
	end := forwardWordLoc(bp, start)
	text := bp.GetText(start, end)
	length := uint(len(text))
	if length == 0 || text == nil {
		return false
	}
	newText := []byte(strings.ToUpper(string(text)))
	UndoBeginCommand()
	defer UndoEndCommand()
	var newEnd buffer.Location
	ok := bufferSetText(bp, start, end, newText, &newEnd, false)
	if ok {
		wp.Cursor = newEnd
		wp.DidEdit = true
	}
	return ok
}

func CmdCapWord(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	start := backwardWordLoc(bp, wp.Cursor)
	end := forwardWordLoc(bp, start)
	text := bp.GetText(start, end)
	length := uint(len(text))
	if length == 0 || text == nil {
		return false
	}
	runes := []rune(string(text))
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
		for i := 1; i < len(runes); i++ {
			runes[i] = unicode.ToLower(runes[i])
		}
	}
	newText := []byte(string(runes))
	UndoBeginCommand()
	defer UndoEndCommand()
	var newEnd buffer.Location
	ok := bufferSetText(bp, start, end, newText, &newEnd, false)
	if ok {
		wp.Cursor = newEnd
		wp.DidEdit = true
	}
	return ok
}

// Transpose adjacent words around point: left word and next word
// Transpose adjacent words around point: left word and next word
func CmdTransposeWords(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	// Find left word
	leftStart := backwardWordLoc(bp, wp.Cursor)
	leftEnd := forwardWordLoc(bp, leftStart)
	// Find right word start
	rightStart := nextWordStart(bp, leftEnd)
	if rightStart.Line == leftEnd.Line && rightStart.Offset == leftEnd.Offset {
		// No right word
		return false
	}
	rightEnd := forwardWordLoc(bp, rightStart)
	// Extract texts
	leftText := bp.GetText(leftStart, leftEnd)
	rightText := bp.GetText(rightStart, rightEnd)
	if leftText == nil || rightText == nil {
		return false
	}
	// Replace right then left to avoid offset shifts
	UndoBeginCommand()
	defer UndoEndCommand()
	// replace right with leftText
	var tmpEnd buffer.Location
	if !bufferSetText(bp, rightStart, rightEnd, leftText, &tmpEnd, false) {
		return false
	}
	// After replacing right, left region unchanged; replace left with rightText
	if !bufferSetText(bp, leftStart, leftEnd, rightText, &tmpEnd, false) {
		return false
	}
	wp.DidEdit = true
	return true
}

// Fill paragraph at point to buffer.FillCol or 72
// Fill paragraph at point to buffer.FillCol or 72
func CmdFillParagraph(f bool, n int) bool {
	_ = f
	_ = n
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}
	lineNum := wp.Cursor.Line
	// find paragraph start
	start := lineNum
	for start > 1 {
		lp := bp.Line(start-1)
		if lp == nil || lp.IsBlank() {
			break
		}
		start--
	}
	end := lineNum
	for end < bp.LineCount {
		nl := bp.Line(end+1)
		if nl == nil || nl.IsBlank() {
			break
		}
		end++
	}
	// collect words from lines start..end
	words := make([]string, 0)
	for ln := start; ln <= end; ln++ {
		lp := bp.Line(ln)
		if lp == nil {
			continue
		}
		s := string(lp.Data)
		// split on whitespace
		for _, w := range strings.Fields(s) {
			words = append(words, w)
		}
	}
	if len(words) == 0 {
		return true
	}
	fillCol := int(bp.FillCol)
	if fillCol == 0 {
		fillCol = 72
	}
	// build lines
	outLines := make([]string, 0)
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
			continue
		}
		if len(cur)+1+len(w) <= fillCol {
			cur = cur + " " + w
		} else {
			outLines = append(outLines, cur)
			cur = w
		}
	}
	if cur != "" {
		outLines = append(outLines, cur)
	}
	newText := strings.Join(outLines, "\n")
	begin := buffer.MakeLocation(start, 0)
	endLoc := bp.Line(end)
	endOff := uint(0)
	if endLoc != nil {
		endOff = endLoc.Len()
	}
	endLocation := buffer.MakeLocation(end, endOff)
	UndoBeginCommand()
	defer UndoEndCommand()
	var newEnd buffer.Location
	ok := bufferSetText(bp, begin, endLocation, []byte(newText), &newEnd, false)
	if ok {
		wp.DidEdit = true
	}
	return ok
}

// Page-wise movement
func CmdDeleteBackward(f bool, n int) bool {
	if n < 0 {
		return CmdDeleteForward(f, -n)
	}
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly || n == 0 {
		return false
	}

	UndoBeginCommand()
	defer UndoEndCommand()

	end := wp.Cursor
	CmdBackwardChar(f, n)
	begin := wp.Cursor
	if begin == end {
		return false
	}
	var newEnd buffer.Location
	if !bufferSetText(bp, begin, end, nil, &newEnd, false) {
		return false
	}
	wp.Cursor = begin
	wp.DidEdit = true
	return true
}

func CmdDeleteForward(f bool, n int) bool {
	if n < 0 {
		return CmdDeleteBackward(f, -n)
	}
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly || n == 0 {
		return false
	}

	UndoBeginCommand()
	defer UndoEndCommand()

	begin := wp.Cursor
	CmdForwardChar(f, n)
	end := wp.Cursor
	if begin == end {
		return false
	}
	wp.Cursor = begin
	if !bufferSetText(bp, begin, end, nil, nil, false) {
		return false
	}
	wp.DidEdit = true
	return true
}

func CmdInsertChar(c byte) bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.IsReadonly {
		return false
	}

	UndoBeginCommand()
	defer UndoEndCommand()

	begin := wp.Cursor
	var newEnd buffer.Location
	if bufferSetText(bp, begin, begin, []byte{c}, &newEnd, false) {
		wp.Cursor = newEnd
		wp.DidEdit = true
		return true
	}
	return false
}

// commandsProvider returns the command name label for the given index. ctx is a []string.

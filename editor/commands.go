package editor

import (
	"github.com/jdpalmer/jem/buffer"
	"strings"
	"unicode"

	"github.com/jdpalmer/jem/app"
)

// commands.go - Editor text commands and movement (translation of cmd_move.c and cmd_edit.c)

// helper: ASCII word char
func isWordChar(b byte) bool {
	if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' {
		return true
	}
	return false
}

// move forward by one word: skip non-word then skip word
func forwardWordLoc(bp *Buffer, loc Location) Location {
	if bp == nil || loc.Line == 0 {
		return loc
	}
	line := bp.Line(loc.Line)
	if line == nil {
		return loc
	}
	off := int(loc.Offset)
	// within line: if at or beyond used, move to next line start
	for {
		if off < len(line.Data) {
			b := line.Data[off]
			if !isWordChar(b) {
				// skip non-word
				off++
				continue
			}
			break
		} else {
			// move to next line
			if loc.Line >= bp.LineCount {
				return Location{Line: bp.LineCount, Offset: line.Len()}
			}
			loc.Line++
			line = bp.Line(loc.Line)
			off = 0
		}
		break
	}
	// now skip non-word starting at original pos
	for loc.Line <= bp.LineCount {
		line = bp.Line(loc.Line)
		if line == nil {
			return loc
		}
		for off < len(line.Data) && !isWordChar(line.Data[off]) {
			off++
		}
		if off < len(line.Data) {
			// found start of word; now advance to end of word
			for off < len(line.Data) && isWordChar(line.Data[off]) {
				off++
			}
			return Location{Line: loc.Line, Offset: uint(off)}
		}
		// continue to next line
		if loc.Line >= bp.LineCount {
			return Location{Line: bp.LineCount, Offset: 0}
		}
		loc.Line++
		off = 0
	}
	return Location{Line: bp.LineCount, Offset: 0}
}

// move backward by one word: go left, skip non-word, then skip word backwards
func backwardWordLoc(bp *Buffer, loc Location) Location {
	if bp == nil || loc.Line == 0 {
		return loc
	}
	// If at start of buffer, return same
	if loc.Line == 1 && loc.Offset == 0 {
		return loc
	}
	line := bp.Line(loc.Line)
	if line == nil {
		return loc
	}
	off := int(loc.Offset)
	// start by stepping left one position (if at offset 0, move to end of prev line)
	if off == 0 {
		// move to end of previous line
		if loc.Line > 1 {
			loc.Line--
			line = bp.Line(loc.Line)
			if line != nil {
				off = len(line.Data)
			} else {
				off = 0
			}
		} else {
			return loc
		}
	}
	// now off > 0 or at some position; step left over non-word then word
	// step back to previous codepoint boundary and inspect bytes
	for {
		for off > 0 {
			// move to previous UTF-8 rune start
			offPrev := utf8PrevOffset(line.Data, uint(off))
			if offPrev == uint(off) { // can't move
				off--
			} else {
				off = int(offPrev)
			}
			b := byte(0)
			if off < len(line.Data) {
				b = line.Data[off]
			}
			if isWordChar(b) {
				break
			}
			if off == 0 {
				break
			}
		}
		// skip non-word backwards
		for off > 0 && !isWordChar(line.Data[off-1]) {
			off--
		}
		// now skip word backwards
		for off > 0 && isWordChar(line.Data[off-1]) {
			off--
		}
		return Location{Line: loc.Line, Offset: uint(off)}
	}
	return loc
}

// Move forward by a single codepoint, preserving UTF-8 boundaries.
func CmdForwardChar(f bool, n int) bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}

	for i := 0; i < n; i++ {
		line := bp.Line(wp.Cursor.Line)
		if line != nil && wp.Cursor.Offset < line.Len() {
			wp.Cursor.Offset = utf8NextOffset(line.Data, wp.Cursor.Offset)
		} else if wp.Cursor.Line < bp.LineCount {
			wp.Cursor.Line++
			wp.Cursor.Offset = 0
		} else {
			break
		}
	}
	wp.DidMove = true
	return true
}

func CmdBackwardChar(f bool, n int) bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}

	for i := 0; i < n; i++ {
		line := bp.Line(wp.Cursor.Line)
		if line != nil && wp.Cursor.Offset > 0 {
			wp.Cursor.Offset = utf8PrevOffset(line.Data, wp.Cursor.Offset)
		} else if wp.Cursor.Line > 1 {
			wp.Cursor.Line--
			prevLine := bp.Line(wp.Cursor.Line)
			if prevLine != nil {
				wp.Cursor.Offset = prevLine.Len()
			} else {
				wp.Cursor.Offset = 0
			}
		} else {
			break
		}
	}
	wp.DidMove = true
	return true
}

// CmdForwardWord moves forward by words (ASCII words: letters, digits, underscore)
func CmdForwardWord(f bool, n int) bool {
	_ = f
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	for i := 0; i < n; i++ {
		wp.Cursor = forwardWordLoc(bp, wp.Cursor)
	}
	wp.DidMove = true
	return true
}

// CmdBackwardWord moves backward by words
func CmdBackwardWord(f bool, n int) bool {
	_ = f
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}
	for i := 0; i < n; i++ {
		wp.Cursor = backwardWordLoc(bp, wp.Cursor)
	}
	wp.DidMove = true
	return true
}

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
		var newEnd Location
		if !bufferSetText(bp, start, end, nil, &newEnd, false) {
			return false
		}
		wp.Cursor = newEnd
	}
	wp.DidEdit = true
	return true
}

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
		var newEnd Location
		if !bufferSetText(bp, start, end, nil, &newEnd, false) {
			return false
		}
		wp.Cursor = newEnd
	}
	wp.DidEdit = true
	return true
}

// helper: find start of next word from loc (skip non-word then return start)
func nextWordStart(bp *Buffer, loc Location) Location {
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
			return Location{Line: ln, Offset: uint(off)}
		}
		off = 0
	}
	return Location{Line: bp.LineCount, Offset: 0}
}

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
	var newEnd Location
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
	var newEnd Location
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
	var newEnd Location
	ok := bufferSetText(bp, start, end, newText, &newEnd, false)
	if ok {
		wp.Cursor = newEnd
		wp.DidEdit = true
	}
	return ok
}

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
	var tmpEnd Location
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
	var newEnd Location
	ok := bufferSetText(bp, begin, endLocation, []byte(newText), &newEnd, false)
	if ok {
		wp.DidEdit = true
	}
	return ok
}

// Page-wise movement
func CmdForwardPage(f bool, n int) bool {
	wp := app.State.CurrentWindow
	if wp == nil {
		return false
	}
	pageLines := int(wp.Height)
	if pageLines > 2 {
		pageLines = int(wp.Height - 2)
	} else {
		pageLines = 1
	}
	return CmdForwardLine(f, pageLines*n)
}

func CmdBackwardPage(f bool, n int) bool {
	wp := app.State.CurrentWindow
	if wp == nil {
		return false
	}
	pageLines := int(wp.Height)
	if pageLines > 2 {
		pageLines = int(wp.Height - 2)
	} else {
		pageLines = 1
	}
	return CmdBackwardLine(f, pageLines*n)
}

func CmdForwardLine(f bool, n int) bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}

	if wp.Cursor.Line+uint(n) <= bp.LineCount {
		wp.Cursor.Line += uint(n)
	} else {
		wp.Cursor.Line = bp.LineCount
	}

	line := bp.Line(wp.Cursor.Line)
	if line != nil && wp.Cursor.Offset > line.Len() {
		wp.Cursor.Offset = line.Len()
	}
	wp.DidMove = true
	return true
}

func CmdBackwardLine(f bool, n int) bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		return false
	}

	if wp.Cursor.Line > uint(n) {
		wp.Cursor.Line -= uint(n)
	} else {
		wp.Cursor.Line = 1
	}

	line := bp.Line(wp.Cursor.Line)
	if line != nil && wp.Cursor.Offset > line.Len() {
		wp.Cursor.Offset = line.Len()
	}
	wp.DidMove = true
	return true
}

func CmdGotoBol(f bool, n int) bool {
	wp := app.State.CurrentWindow
	if wp != nil {
		wp.Cursor.Offset = 0
		wp.DidMove = true
	}
	return true
}

func CmdGotoEol(f bool, n int) bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp != nil && bp != nil {
		line := bp.Line(wp.Cursor.Line)
		if line != nil {
			wp.Cursor.Offset = line.Len()
		} else {
			wp.Cursor.Offset = 0
		}
		wp.DidMove = true
	}
	return true
}

func CmdGotoBof(f bool, n int) bool {
	wp := app.State.CurrentWindow
	if wp != nil {
		wp.Cursor.Line = 1
		wp.Cursor.Offset = 0
		wp.DidMove = true
	}
	return true
}

func CmdGotoEof(f bool, n int) bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp != nil && bp != nil {
		wp.Cursor.Line = bp.LineCount
		line := bp.Line(wp.Cursor.Line)
		if line != nil {
			wp.Cursor.Offset = line.Len()
		} else {
			wp.Cursor.Offset = 0
		}
		wp.DidMove = true
	}
	return true
}

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
	var newEnd Location
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
	var newEnd Location
	if bufferSetText(bp, begin, begin, []byte{c}, &newEnd, false) {
		wp.Cursor = newEnd
		wp.DidEdit = true
		return true
	}
	return false
}

// commandsProvider returns the command name label for the given index. ctx is a []string.
func commandsProvider(ctx any, idx uint) []byte {
	if ctx == nil {
		return nil
	}
	names, ok := ctx.([]string)
	if !ok {
		return nil
	}
	if int(idx) >= len(names) {
		return nil
	}
	return []byte(names[idx])
}

// CmdCommandPalette opens the command palette (M-x) and executes the chosen command.
func CmdCommandPalette(f bool, n int) bool {
	_ = f
	_ = n
	// Build provider list
	names := buildCommandList()
	if len(names) == 0 {
		mbWrite("[no commands]")
		return false
	}
	label, pr := mbReadFuzzyListString("M-x: ", commandsProvider, names, uint(len(names)))
	if pr != PromptResultYes {
		return false
	}
	if label == "" {
		return false
	}
	cmdName := strings.ToLower(label)
	if cmdFn, ok := commandNameMap[cmdName]; ok {
		cmdFn(false, 1)
		return true
	}
	mbWrite("[unknown command: %s]", label)
	return false
}

// CmdDescribeCommand shows the name and description of a selected command.
func CmdDescribeCommand(f bool, n int) bool {
	_ = f
	_ = n
	names := buildCommandList()
	if len(names) == 0 {
		mbWrite("[no commands]")
		return false
	}
	label, pr := mbReadFuzzyListString("Describe: ", commandsProvider, names, uint(len(names)))
	if pr != PromptResultYes {
		return false
	}
	if label == "" {
		return false
	}
	if cmd := commandByName(label); cmd != nil && cmd.Doc != "" {
		mbWrite("%s: %s", cmd.Name, cmd.Doc)
		return true
	}
	mbWrite("Command: %s", label)
	return true
}

// CmdKillBuffer kills/releases the current buffer.
func CmdKillBuffer(f bool, n int) bool {
	_ = f
	// If numeric argument provided, kill that buffer (1-based index)
	if n > 0 {
		if n <= int(app.State.BufferCount) {
			bp := app.State.Buffers[n-1]
			if bp == nil {
				mbWrite("[no such buffer]")
				return false
			}
			// confirm
			if mbYesNo("Kill buffer?") != PromptResultYes {
				mbWrite("[aborted]")
				return false
			}
			app.BufferRelease(bp)
			mbWrite("[buffer killed]")
			return true
		}
		mbWrite("[no such buffer]")
		return false
	}

	// default: kill current buffer with confirmation
	bp := app.State.CurrentBuffer
	if bp == nil {
		mbWrite("[no buffer to kill]")
		return false
	}
	if mbYesNo("Kill current buffer?") != PromptResultYes {
		mbWrite("[aborted]")
		return false
	}
	app.BufferRelease(bp)
	mbWrite("[buffer killed]")
	return true
}

// CmdKillBufferFuzzy prompts the user with a fuzzy list of buffers and kills the
// chosen buffer after confirmation.
func CmdKillBufferFuzzy(f bool, n int) bool {
	_ = f
	_ = n
	names := make([]string, 0, app.State.BufferCount)
	for i := 0; i < int(app.State.BufferCount); i++ {
		bp := app.State.Buffers[i]
		if bp == nil {
			continue
		}
		names = append(names, bp.Name)
	}
	if len(names) == 0 {
		mbWrite("[no buffers]")
		return false
	}
	label, pr := mbReadFuzzyListString("Kill buffer: ", commandsProvider, names, uint(len(names)))
	if pr != PromptResultYes {
		return false
	}
	if label == "" {
		return false
	}
	// find buffer by name
	for i := 0; i < int(app.State.BufferCount); i++ {
		bp := app.State.Buffers[i]
		if bp == nil {
			continue
		}
		if strings.EqualFold(bp.Name, label) {
			if mbYesNo("Kill buffer?") != PromptResultYes {
				mbWrite("[aborted]")
				return false
			}
			app.BufferRelease(bp)
			mbWrite("[buffer killed]")
			return true
		}
	}
	mbWrite("[buffer not found: %s]", label)
	return false
}

// pickBufferList returns the active buffers in editor order.
func pickBufferList() []*Buffer {
	list := make([]*Buffer, 0, app.State.BufferCount)
	for i := 0; i < int(app.State.BufferCount); i++ {
		if bp := app.State.Buffers[i]; bp != nil {
			list = append(list, bp)
		}
	}
	return list
}

func bufferChoiceLabel(ctx any, idx uint8) []byte {
	list, _ := ctx.([]*Buffer)
	if int(idx) >= len(list) || list[idx] == nil {
		return nil
	}
	return []byte(list[idx].Name)
}

func findBufferByLabel(label string) *Buffer {
	for i := 0; i < int(app.State.BufferCount); i++ {
		bp := app.State.Buffers[i]
		if bp == nil {
			continue
		}
		if strings.EqualFold(bp.Name, label) {
			return bp
		}
	}
	return nil
}

// CmdUseBuffer switches to a buffer. With a universal argument (f true, n > 0),
// select the nth buffer (1-based) directly. Otherwise show a horizontal picker (C-x b).
func CmdUseBuffer(f bool, n int) bool {
	if f && n > 0 {
		if n <= int(app.State.BufferCount) {
			bp := app.State.Buffers[n-1]
			if bp != nil {
				editorSwitchBuffer(bp)
				return true
			}
		}
		return false
	}

	buffers := pickBufferList()
	if len(buffers) == 0 {
		mbWrite("[no buffers]")
		return false
	}

	var bp *Buffer
	if app.State.IsPlaying() {
		label, pr := mbReadStringCap("Buffer: ", "", BufferNameCapacity)
		if pr != PromptResultYes {
			return false
		}
		if label == "" {
			return false
		}
		bp = findBufferByLabel(label)
		if bp == nil {
			mbWrite("[no such buffer]")
			return false
		}
	} else {
		defaultIdx := uint8(0)
		if len(buffers) > 1 {
			defaultIdx = 1
		}
		sel := mbChoose("Buffer: ", buffers, bufferChoiceLabel, uint8(len(buffers)), defaultIdx)
		if sel == -2 {
			CmdAbort(false, 1)
			return false
		}
		if sel < 0 {
			return false
		}
		bp = buffers[sel]
	}

	macroRecordBufferName(bp)
	editorSwitchBuffer(bp)
	DisplayUpdate()
	return true
}

// CmdBackToIndentation moves point to the first non-blank character on the line.
func CmdBackToIndentation(f bool, n int) bool {
	_ = f
	_ = n
	wp := app.State.CurrentWindow
	if wp == nil {
		return false
	}
	lp := wp.Buffer.Line(wp.Cursor.Line)
	if lp != nil {
		wp.Cursor.Offset = lp.FirstNonblank()
	} else {
		wp.Cursor.Offset = 0
	}
	wp.DidMove = true
	return true
}

// CmdGotoLine jumps to a specific line number.
func CmdGotoLine(f bool, n int) bool {
	bp := app.State.CurrentBuffer
	wp := app.State.CurrentWindow
	if bp == nil || wp == nil {
		return false
	}
	var target uint
	if f {
		if n <= 0 {
			mbWrite("[line number out of range]")
			return false
		}
		target = uint(n)
	} else {
		lineStr, pr := mbReadStringCap("Goto line: ", "", 32)
		if pr != PromptResultYes {
			return false
		}
		parsed, ok := parsePositiveLineNumber(lineStr)
		if !ok {
			mbWrite("[invalid line number]")
			return false
		}
		target = parsed
	}
	if target > bp.LineCount {
		mbWrite("[line number out of range]")
		return false
	}
	if wp.Cursor.Line != target || wp.Cursor.Offset != 0 {
		app.MarkPushCurrent()
	}
	wp.SetCursor(buffer.MakeLocation(target, 0))
	wp.ShouldRedraw = true
	return true
}

func parsePositiveLineNumber(s string) (uint, bool) {
	if s == "" {
		return 0, false
	}
	var n uint
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + uint(c-'0')
		if n == 0 {
			return 0, false
		}
	}
	return n, true
}

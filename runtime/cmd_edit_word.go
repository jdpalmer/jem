package runtime

import (
	"github.com/jdpalmer/jem/window"
	"strings"
	"unicode"

	"github.com/jdpalmer/jem/buffer"
)

// cmd_edit_word.go — word edits and character insert/delete

// delete forward word
func CmdDeleteWordForward(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}
	BeginCommand()
	defer EndCommand()
	for i := 0; i < n; i++ {
		start := win.Cursor
		end := forwardWordLoc(buf, start)
		var newEnd buffer.Location
		if !bufferSetText(buf, start, end, nil, &newEnd, false) {
			return false
		}
		win.Cursor = newEnd
	}
	win.DidEdit = true
	return true
}

// delete backward word
// delete backward word
func CmdDeleteWordBackward(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}
	BeginCommand()
	defer EndCommand()
	for i := 0; i < n; i++ {
		end := win.Cursor
		start := backwardWordLoc(buf, end)
		var newEnd buffer.Location
		if !bufferSetText(buf, start, end, nil, &newEnd, false) {
			return false
		}
		win.Cursor = newEnd
	}
	win.DidEdit = true
	return true
}

// helper: find start of next word from loc (skip non-word then return start)
// helper: find start of next word from loc (skip non-word then return start)
func nextWordStart(buf *buffer.Buffer, loc buffer.Location) buffer.Location {
	if loc.Line == 0 {
		return loc
	}
	line := buf.Line(loc.Line)
	if line == nil {
		return loc
	}
	off := int(loc.Offset)
	for ln := loc.Line; ln <= len(buf.Lines); ln++ {
		line = buf.Line(ln)
		if line == nil {
			continue
		}
		for off < len(line.Data) && !isWordChar(line.Data[off]) {
			off++
		}
		if off < len(line.Data) {
			return buffer.Location{Line: ln, Offset: off}
		}
		off = 0
	}
	return buffer.Location{Line: len(buf.Lines), Offset: 0}
}

// Case transformations on a word at point
// Case transformations on a word at point
func CmdLowerWord(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}
	// operate on word at point: find start and end
	start := backwardWordLoc(buf, win.Cursor)
	end := forwardWordLoc(buf, start)
	text := buf.GetText(start, end)
	length := len(text)
	if length == 0 || text == nil {
		return false
	}
	newText := []byte(strings.ToLower(string(text)))
	BeginCommand()
	defer EndCommand()
	var newEnd buffer.Location
	ok := bufferSetText(buf, start, end, newText, &newEnd, false)
	if ok {
		win.Cursor = newEnd
		win.DidEdit = true
	}
	return ok
}

func CmdUpperWord(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}
	start := backwardWordLoc(buf, win.Cursor)
	end := forwardWordLoc(buf, start)
	text := buf.GetText(start, end)
	length := len(text)
	if length == 0 || text == nil {
		return false
	}
	newText := []byte(strings.ToUpper(string(text)))
	BeginCommand()
	defer EndCommand()
	var newEnd buffer.Location
	ok := bufferSetText(buf, start, end, newText, &newEnd, false)
	if ok {
		win.Cursor = newEnd
		win.DidEdit = true
	}
	return ok
}

func CmdCapWord(f bool, n int) bool {
	_ = f
	if n <= 0 {
		return false
	}
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}
	start := backwardWordLoc(buf, win.Cursor)
	end := forwardWordLoc(buf, start)
	text := buf.GetText(start, end)
	length := len(text)
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
	BeginCommand()
	defer EndCommand()
	var newEnd buffer.Location
	ok := bufferSetText(buf, start, end, newText, &newEnd, false)
	if ok {
		win.Cursor = newEnd
		win.DidEdit = true
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
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}
	// Find left word
	leftStart := backwardWordLoc(buf, win.Cursor)
	leftEnd := forwardWordLoc(buf, leftStart)
	// Find right word start
	rightStart := nextWordStart(buf, leftEnd)
	if rightStart.Line == leftEnd.Line && rightStart.Offset == leftEnd.Offset {
		// No right word
		return false
	}
	rightEnd := forwardWordLoc(buf, rightStart)
	// Extract texts
	leftText := buf.GetText(leftStart, leftEnd)
	rightText := buf.GetText(rightStart, rightEnd)
	if leftText == nil || rightText == nil {
		return false
	}
	// Replace right then left to avoid offset shifts
	BeginCommand()
	defer EndCommand()
	// replace right with leftText
	var tmpEnd buffer.Location
	if !bufferSetText(buf, rightStart, rightEnd, leftText, &tmpEnd, false) {
		return false
	}
	// After replacing right, left region unchanged; replace left with rightText
	if !bufferSetText(buf, leftStart, leftEnd, rightText, &tmpEnd, false) {
		return false
	}
	win.DidEdit = true
	return true
}

// Fill paragraph at point to buffer.FillCol or 72
// Fill paragraph at point to buffer.FillCol or 72
func CmdFillParagraph(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}
	lineNum := win.Cursor.Line
	// find paragraph start
	start := lineNum
	for start > 1 {
		line := buf.Line(start - 1)
		if line == nil || line.IsBlank() {
			break
		}
		start--
	}
	end := lineNum
	for end < len(buf.Lines) {
		nl := buf.Line(end + 1)
		if nl == nil || nl.IsBlank() {
			break
		}
		end++
	}
	// collect words from lines start..end
	words := make([]string, 0)
	for ln := start; ln <= end; ln++ {
		line := buf.Line(ln)
		if line == nil {
			continue
		}
		s := string(line.Data)
		words = append(words, strings.Fields(s)...)
	}
	if len(words) == 0 {
		return true
	}
	fillCol := buf.FillCol
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
	endLoc := buf.Line(end)
	endOff := 0
	if endLoc != nil {
		endOff = endLoc.Len()
	}
	endLocation := buffer.MakeLocation(end, endOff)
	BeginCommand()
	defer EndCommand()
	var newEnd buffer.Location
	ok := bufferSetText(buf, begin, endLocation, []byte(newText), &newEnd, false)
	if ok {
		win.DidEdit = true
	}
	return ok
}

// Page-wise movement
func CmdDeleteBackward(f bool, n int) bool {
	if n < 0 {
		return CmdDeleteForward(f, -n)
	}
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly || n == 0 {
		return false
	}

	BeginCommand()
	defer EndCommand()

	end := win.Cursor
	CmdBackwardChar(f, n)
	begin := win.Cursor
	if begin == end {
		return false
	}
	var newEnd buffer.Location
	if !bufferSetText(buf, begin, end, nil, &newEnd, false) {
		return false
	}
	win.Cursor = begin
	win.DidEdit = true
	return true
}

func CmdDeleteForward(f bool, n int) bool {
	if n < 0 {
		return CmdDeleteBackward(f, -n)
	}
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly || n == 0 {
		return false
	}

	BeginCommand()
	defer EndCommand()

	begin := win.Cursor
	CmdForwardChar(f, n)
	end := win.Cursor
	if begin == end {
		return false
	}
	win.Cursor = begin
	if !bufferSetText(buf, begin, end, nil, nil, false) {
		return false
	}
	win.DidEdit = true
	return true
}

func CmdInsertChar(c byte) bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.IsReadonly {
		return false
	}

	BeginCommand()
	defer EndCommand()

	begin := win.Cursor
	var newEnd buffer.Location
	if bufferSetText(buf, begin, begin, []byte{c}, &newEnd, false) {
		win.Cursor = newEnd
		win.DidEdit = true
		return true
	}
	return false
}

// commandsProvider returns the command name label for the given index. ctx is a []string.

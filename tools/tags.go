package tools

// tags.go - Tag database and navigation (translation of src/tags.c)

import (
	"encoding/json"
	"fmt"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/files"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
	"os"
	"path/filepath"
	"unicode"
)

const (
	tagsFileName      = "tags.json"
	tagHintContextMax = 512
	tagHintLinesMax   = 8
)

type TagEntry struct {
	Name      string
	Path      string
	Kind      string
	Signature string
	Line      uint32
}

type tagDbState struct {
	path    string
	mtime   os.FileInfo
	entries []TagEntry
}

var tagDb tagDbState

func tagDbClear() {
	tagDb.path = ""
	tagDb.mtime = nil
	tagDb.entries = nil
}

func tagFindTagsFile() (string, bool) {
	tagName := os.Getenv("JEM_TAGS_FILE")
	if tagName == "" {
		tagName = tagsFileName
	}

	if filepath.IsAbs(tagName) {
		if _, err := os.Stat(tagName); err == nil {
			return files.NormalizePath(tagName), true
		}
		return "", false
	}

	dir := ""
	if bp := buffer.All.Current; bp != nil {
		if fname := bp.FileName; fname != "" {
			dir = filepath.Dir(files.NormalizePath(fname))
		}
	}
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", false
		}
		dir = cwd
	}

	if path, ok := files.FindFileWalkUp(dir, tagName); ok {
		return path, true
	}
	return "", false
}

func tagEntryParse(line []byte, tagsDir string) (TagEntry, bool) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return TagEntry{}, false
	}

	var typ string
	if err := json.Unmarshal(raw["_type"], &typ); err != nil || typ != "tag" {
		return TagEntry{}, false
	}

	var name string
	if err := json.Unmarshal(raw["name"], &name); err != nil || name == "" {
		return TagEntry{}, false
	}

	var rawPath string
	if err := json.Unmarshal(raw["path"], &rawPath); err != nil || rawPath == "" {
		return TagEntry{}, false
	}

	var lineNum float64
	if err := json.Unmarshal(raw["line"], &lineNum); err != nil {
		return TagEntry{}, false
	}
	if lineNum < 1 || lineNum > float64(^uint32(0)) || lineNum != float64(uint32(lineNum)) {
		return TagEntry{}, false
	}

	entry := TagEntry{
		Name: name,
		Line: uint32(lineNum),
	}
	if filepath.IsAbs(rawPath) {
		entry.Path = rawPath
	} else {
		entry.Path = filepath.Join(tagsDir, rawPath)
	}

	if field, ok := raw["kind"]; ok {
		var kind string
		if json.Unmarshal(field, &kind) == nil {
			entry.Kind = kind
		}
	}
	if field, ok := raw["signature"]; ok {
		var sig string
		if json.Unmarshal(field, &sig) == nil {
			entry.Signature = sig
		}
	}
	return entry, true
}

func tagDbLoad(path string) (string, bool) {
	tagDbClear()

	text, err := os.ReadFile(path)
	if err != nil {
		return "cannot read " + path, false
	}

	tagsDir := filepath.Dir(path)
	lineNumber := uint64(1)
	start := 0
	for start <= len(text) {
		end := start
		for end < len(text) && text[end] != '\n' {
			end++
		}
		line := text[start:end]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if len(line) > 0 {
			if !json.Valid(line) {
				tagDbClear()
				return fmt.Sprintf("invalid %s line %d: parse error", path, lineNumber), false
			}
			entry, ok := tagEntryParse(line, tagsDir)
			if ok {
				tagDb.entries = append(tagDb.entries, entry)
			}
		}
		if end >= len(text) {
			break
		}
		start = end + 1
		lineNumber++
	}

	st, err := os.Stat(path)
	if err != nil {
		tagDbClear()
		return "cannot stat " + path, false
	}

	tagDb.path = path
	tagDb.mtime = st
	return "", true
}

func EnsureTagsLoaded(quiet bool) bool {
	path, ok := tagFindTagsFile()
	if !ok {
		if !quiet {
			mbWrite("[no %s found; run make tags]", tagsFileName)
		}
		return false
	}

	st, err := os.Stat(path)
	if err != nil {
		if !quiet {
			mbWrite("[cannot stat %s]", path)
		}
		return false
	}

	if tagDb.path == path && tagDb.mtime != nil &&
		tagDb.mtime.ModTime().Equal(st.ModTime()) && tagDb.entries != nil {
		return true
	}

	if msg, ok := tagDbLoad(path); !ok {
		if !quiet && msg != "" {
			mbWrite("[%s]", msg)
		}
		return false
	}
	return true
}

func tagIsSymbolChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '_'
}

func tagSymbolAtPoint(wp *window.Window, symbol []byte) bool {
	if wp == nil || wp.Buffer == nil {
		return false
	}
	line := wp.Buffer.Line(wp.Cursor.Line)
	if line == nil {
		return false
	}

	start := int(wp.Cursor.Offset)
	var end int
	if start < len(line.Data) && tagIsSymbolChar(line.Data[start]) {
		end = start
	} else if start > 0 && tagIsSymbolChar(line.Data[start-1]) {
		start--
		end = start
	} else {
		return false
	}

	for start > 0 && tagIsSymbolChar(line.Data[start-1]) {
		start--
	}
	for end < len(line.Data) && tagIsSymbolChar(line.Data[end]) {
		end++
	}
	if end <= start || end-start >= len(symbol) {
		return false
	}
	copy(symbol, line.Data[start:end])
	symbol[end-start] = 0
	return true
}

func tagVisitLocation(path string, line, offset uint32) bool {
	if line == 0 {
		return false
	}
	return fileVisitLocation(path, line, offset+1)
}

func tagMatchCount(name string, requireSignature bool) int {
	count := 0
	for i := range tagDb.entries {
		entry := &tagDb.entries[i]
		if entry.Name != name {
			continue
		}
		if requireSignature && entry.Signature == "" {
			continue
		}
		count++
	}
	return count
}

func tagCollectMatches(name string, requireSignature bool) []*TagEntry {
	count := tagMatchCount(name, requireSignature)
	if count == 0 {
		return nil
	}
	matches := make([]*TagEntry, 0, count)
	for i := range tagDb.entries {
		entry := &tagDb.entries[i]
		if entry.Name != name {
			continue
		}
		if requireSignature && entry.Signature == "" {
			continue
		}
		matches = append(matches, entry)
	}
	return matches
}

func tagSignatureScore(bp *buffer.Buffer, entry *TagEntry) int {
	score := 0
	if entry.Signature != "" {
		score += 8
	}
	switch entry.Kind {
	case "function", "method":
		score += 4
	case "prototype":
		score += 3
	case "macro":
		score += 2
	}
	if bp != nil && bp.FileName != "" && files.PathsEqual(bp.FileName, entry.Path) {
		score += 6
	}
	return score
}

func tagBestSignature(bp *buffer.Buffer, name string) *TagEntry {
	var best *TagEntry
	bestScore := int(^uint(0)>>1) * -1
	for i := range tagDb.entries {
		entry := &tagDb.entries[i]
		if entry.Name != name || entry.Signature == "" {
			continue
		}
		score := tagSignatureScore(bp, entry)
		if best == nil || score > bestScore {
			best = entry
			bestScore = score
		}
	}
	return best
}

type tagMatchList struct {
	matches []*TagEntry
}

func (l *tagMatchList) provider(idx uint) []byte {
	if l == nil || int(idx) >= len(l.matches) {
		return nil
	}
	return []byte(l.matches[idx].Name)
}

func tagDisplayFormatter(out []byte, outSize uint, idx uint, ctx any) {
	list, _ := ctx.(*tagMatchList)
	if list == nil || int(idx) >= len(list.matches) {
		if len(out) > 0 {
			out[0] = 0
		}
		return
	}
	entry := list.matches[idx]
	kind := entry.Kind
	if kind == "" {
		kind = "tag"
	}
	text := fmt.Sprintf("%s  %s  %s:%d", entry.Name, kind, entry.Path, entry.Line)
	if entry.Signature != "" {
		text += " " + entry.Signature
	}
	n := copy(out, []byte(text))
	if uint(n) < outSize {
		out[n] = 0
	}
}

func tagChooseMatch(matches []*TagEntry, onDone func(choice int)) {
	if len(matches) == 0 {
		onDone(-1)
		return
	}
	count := len(matches)
	if count > 255 {
		count = 255
	}

	list := &tagMatchList{matches: matches[:count]}
	askFuzzyEx("Tag: ", func(ctx any, idx uint) []byte {
		return ctx.(*tagMatchList).provider(idx)
	}, list, uint(count), tagDisplayFormatter, list, func(selected string, r minibuffer.PromptResult) {
		if r == minibuffer.PromptResultAbort {
			CmdAbort(false, 1)
			onDone(-1)
			return
		}
		if r != minibuffer.PromptResultYes {
			onDone(-1)
			return
		}
		for i := 0; i < count; i++ {
			if matches[i].Name == selected {
				onDone(i)
				return
			}
		}
		onDone(-1)
	})
}

func tagVisitMatch(matches []*TagEntry, choice int) {
	if choice < 0 || choice >= len(matches) {
		return
	}
	markPushCurrent()
	entry := matches[choice]
	_ = tagVisitLocation(entry.Path, entry.Line, 0)
}

// RunGotoTag jumps to the definition of the symbol at point (M-.).
func RunGotoTag() bool {
	wp := window.Active.CurrentWindow

	if !EnsureTagsLoaded(false) {
		return false
	}

	finish := func(name string) {
		matches := tagCollectMatches(name, false)
		if len(matches) == 0 {
			mbWrite("[tag not found: %s]", name)
			return
		}
		if len(matches) == 1 {
			tagVisitMatch(matches, 0)
			return
		}
		tagChooseMatch(matches, func(choice int) {
			tagVisitMatch(matches, choice)
		})
	}

	var buf [display.PatternCapacity]byte
	if tagSymbolAtPoint(wp, buf[:]) {
		finish(promptStringFromBuf(buf[:]))
		return true
	}
	askString("Goto tag: ", "", func(symbol string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		finish(symbol)
	})
	return true
}

func tagCollectHintContext(wp *window.Window, out []byte) int {
	if wp == nil || wp.Buffer == nil || len(out) == 0 {
		return 0
	}
	bp := wp.Buffer
	reversed := make([]byte, 0, tagHintContextMax)
	used := 0
	lines := 1
	lineNumber := wp.Cursor.Line
	offset := int(wp.Cursor.Offset)
	line := bp.Line(lineNumber)
	if line == nil {
		out[0] = 0
		return 0
	}

	for used < tagHintContextMax {
		if offset == 0 {
			if lineNumber <= 1 || lines >= tagHintLinesMax {
				break
			}
			reversed = append(reversed, '\n')
			used++
			lineNumber--
			line = bp.Line(lineNumber)
			offset = int(line.Len())
			lines++
			continue
		}
		reversed = append(reversed, line.Data[offset-1])
		used++
		offset--
	}

	if used+1 > len(out) {
		used = len(out) - 1
	}
	for i := 0; i < used; i++ {
		out[i] = reversed[used-i-1]
	}
	out[used] = 0
	return used
}

func tagFindCallHint(wp *window.Window, name []byte, argIndexOut *uint32) bool {
	if len(name) == 0 || argIndexOut == nil {
		return false
	}
	context := make([]byte, tagHintContextMax+1)
	length := tagCollectHintContext(wp, context)
	if length == 0 {
		return false
	}

	openIndex := 0
	foundOpen := false
	depth := 0
	for i := length; i > 0; {
		i--
		c := context[i]
		switch c {
		case ')', ']', '}':
			depth++
		case '(', '[', '{':
			if depth > 0 {
				depth--
			} else if c == '(' {
				openIndex = i
				foundOpen = true
			}
		}
		if foundOpen {
			break
		}
	}
	if !foundOpen {
		return false
	}

	end := openIndex
	for end > 0 && unicode.IsSpace(rune(context[end-1])) {
		end--
	}
	start := end
	for start > 0 && tagIsSymbolChar(context[start-1]) {
		start--
	}
	if start == end || end-start >= len(name) {
		return false
	}
	copy(name, context[start:end])
	name[end-start] = 0

	depth = 0
	argIndex := uint32(1)
	sawNonspace := false
	for i := openIndex + 1; i < length; i++ {
		c := context[i]
		if !unicode.IsSpace(rune(c)) {
			sawNonspace = true
		}
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				argIndex++
			}
		}
	}
	if sawNonspace {
		*argIndexOut = argIndex
	} else {
		*argIndexOut = 1
	}
	return true
}

// MaybeShowCallHint displays a signature hint for the function call at point.
func MaybeShowCallHint() {
	if display.Active.MacroRecording || display.Active.MacroPlaying {
		return
	}
	if !EnsureTagsLoaded(true) {
		return
	}

	bp := buffer.All.Current
	wp := window.Active.CurrentWindow
	if bp == nil || wp == nil {
		return
	}
	var name [display.PatternCapacity]byte
	var argIndex uint32
	if !tagFindCallHint(wp, name[:], &argIndex) {
		return
	}

	symbolName := promptStringFromBuf(name[:])
	entry := tagBestSignature(bp, symbolName)
	if entry == nil || entry.Signature == "" {
		return
	}
	mbWrite("%s%s  [arg %d]", symbolName, entry.Signature, argIndex)
}

package tools

// Tag database and navigation (tags.json).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unicode"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/file"
	"github.com/jdpalmer/jem/window"
)

const (
	tagsFileName      = "tags.json"
	tagHintContextMax = 512
	tagHintLinesMax   = 8
)

func promptStringFromBuf(data []byte) string {
	n := bytes.IndexByte(data, 0)
	if n < 0 {
		n = len(data)
	}
	return string(data[:n])
}

type TagEntry struct {
	Name      string
	Path      string
	Kind      string
	Signature string
	Line      int
}

type tagDBState struct {
	path    string
	mtime   time.Time
	entries []TagEntry
}

var tagDB tagDBState

func tagFindTagsFile() (string, bool) {
	tagName := os.Getenv("JEM_TAGS_FILE")
	if tagName == "" {
		tagName = tagsFileName
	}

	if filepath.IsAbs(tagName) {
		if _, err := os.Stat(tagName); err == nil {
			return file.NormalizePath(tagName), true
		}
		return "", false
	}

	dir := ""
	if buf := buffer.All.Current; buf != nil {
		if fname := buf.FileName; fname != "" {
			dir = filepath.Dir(file.NormalizePath(fname))
		}
	}
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", false
		}
		dir = cwd
	}

	if path, ok := file.FindFileWalkUp(dir, tagName); ok {
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
		Line: int(lineNum),
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

func tagDBLoad(path string) (string, bool) {
	tagDB = tagDBState{}

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
				tagDB = tagDBState{}
				return fmt.Sprintf("invalid %s line %d: parse error", path, lineNumber), false
			}
			entry, ok := tagEntryParse(line, tagsDir)
			if ok {
				tagDB.entries = append(tagDB.entries, entry)
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
		tagDB = tagDBState{}
		return "cannot stat " + path, false
	}

	tagDB.path = path
	tagDB.mtime = st.ModTime()
	return "", true
}

func EnsureTagsLoaded(quiet bool) bool {
	path, ok := tagFindTagsFile()
	if !ok {
		if !quiet {
			display.MBWrite("[no %s found; run make tags]", tagsFileName)
		}
		return false
	}

	st, err := os.Stat(path)
	if err != nil {
		if !quiet {
			display.MBWrite("[cannot stat %s]", path)
		}
		return false
	}

	if tagDB.path == path && !tagDB.mtime.IsZero() &&
		tagDB.mtime.Equal(st.ModTime()) && tagDB.entries != nil {
		return true
	}

	if msg, ok := tagDBLoad(path); !ok {
		if !quiet && msg != "" {
			display.MBWrite("[%s]", msg)
		}
		return false
	}
	return true
}

func tagIsSymbolChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '_'
}

func tagSymbolAtPoint(win *window.Window, symbol []byte) bool {
	line := win.Buffer.Line(win.Cursor.Line)

	start := int(win.Cursor.Offset)
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

func tagVisitMatch(matches []*TagEntry, choice int) (path string, line int, ok bool) {
	if choice < 0 || choice >= len(matches) {
		return "", 0, false
	}
	entry := matches[choice]
	if entry.Line == 0 {
		return "", 0, false
	}
	return entry.Path, entry.Line, true
}

func tagCollectMatches(name string, requireSignature bool) []*TagEntry {
	var matches []*TagEntry
	for i := range tagDB.entries {
		entry := &tagDB.entries[i]
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

func tagSignatureScore(buf *buffer.Buffer, entry *TagEntry) int {
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
	if buf != nil && buf.FileName != "" && file.PathsEqual(buf.FileName, entry.Path) {
		score += 6
	}
	return score
}

func tagBestSignature(buf *buffer.Buffer, name string) *TagEntry {
	var best *TagEntry
	var bestScore int
	for i := range tagDB.entries {
		entry := &tagDB.entries[i]
		if entry.Name != name || entry.Signature == "" {
			continue
		}
		score := tagSignatureScore(buf, entry)
		if best == nil || score > bestScore {
			best = entry
			bestScore = score
		}
	}
	return best
}

// SymbolAtPoint returns the tag symbol under the cursor, if any.
func SymbolAtPoint() (string, bool) {
	win := window.Active.CurrentWindow
	if win == nil {
		return "", false
	}
	var scratch [display.PatternCapacity]byte
	if !tagSymbolAtPoint(win, scratch[:]) {
		return "", false
	}
	return promptStringFromBuf(scratch[:]), true
}

// CollectTagMatches returns tag entries matching name.
func CollectTagMatches(name string) []*TagEntry {
	return tagCollectMatches(name, false)
}

// TagMatchLocation returns the file location for matches[choice].
func TagMatchLocation(matches []*TagEntry, choice int) (path string, line int, ok bool) {
	return tagVisitMatch(matches, choice)
}

// TagMatchCount returns the fuzzy-list size (capped) for matches.
func TagMatchCount(matches []*TagEntry) int {
	count := len(matches)
	if count > 255 {
		count = 255
	}
	return count
}

// TagMatchProvider is an MbNameProviderFn over a []*TagEntry ctx.
func TagMatchProvider(ctx any, idx int) []byte {
	matches, _ := ctx.([]*TagEntry)
	if idx < 0 || idx >= len(matches) {
		return nil
	}
	return []byte(matches[idx].Name)
}

// TagMatchFormatter formats a tag match line for the fuzzy list.
func TagMatchFormatter(out []byte, outSize int, idx int, ctx any) {
	matches, _ := ctx.([]*TagEntry)
	if idx < 0 || idx >= len(matches) {
		if len(out) > 0 {
			out[0] = 0
		}
		return
	}
	entry := matches[idx]
	kind := entry.Kind
	if kind == "" {
		kind = "tag"
	}
	text := fmt.Sprintf("%s  %s  %s:%d", entry.Name, kind, entry.Path, entry.Line)
	if entry.Signature == "" {
		n := copy(out, []byte(text))
		if n < outSize {
			out[n] = 0
		}
		return
	}
	text += " " + entry.Signature
	n := copy(out, []byte(text))
	if n < outSize {
		out[n] = 0
	}
}

// IndexOfTagName returns the index of name in matches, or -1.
func IndexOfTagName(matches []*TagEntry, name string) int {
	for i, m := range matches {
		if m.Name == name {
			return i
		}
	}
	return -1
}

func tagCollectHintContext(win *window.Window, out []byte) int {
	buf := win.Buffer
	reversed := make([]byte, 0, tagHintContextMax)
	used := 0
	lines := 1
	lineNumber := win.Cursor.Line
	offset := int(win.Cursor.Offset)
	line := buf.Line(lineNumber)
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
			line = buf.Line(lineNumber)
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

func tagFindCallHint(win *window.Window, name []byte, argIndexOut *uint32) bool {
	context := make([]byte, tagHintContextMax+1)
	length := tagCollectHintContext(win, context)
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

	buf := buffer.All.Current
	win := window.Active.CurrentWindow
	if buf == nil || win == nil {
		return
	}
	var name [display.PatternCapacity]byte
	var argIndex uint32
	if !tagFindCallHint(win, name[:], &argIndex) {
		return
	}

	symbolName := promptStringFromBuf(name[:])
	entry := tagBestSignature(buf, symbolName)
	if entry == nil || entry.Signature == "" {
		return
	}
	display.MBWrite("%s%s  [arg %d]", symbolName, entry.Signature, argIndex)
}

package display

// Filename prompt helpers (path list / fuzzy matching).

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jdpalmer/jem/file"
	"github.com/jdpalmer/jem/minibuffer"
)

func shouldSkipFuzzyFile(name string) bool {
	return strings.HasSuffix(name, ".o") ||
		strings.HasSuffix(name, ".exe") ||
		strings.HasSuffix(name, ".pyc")
}

// fuzzyFileEntry is one row in the find-file match list.
type fuzzyFileEntry struct {
	Name    string
	Size    int64 // -1 when not applicable (directories)
	ModTime time.Time
}

// filenameMatchCtx holds entries plus padded-column widths for the match window.
type filenameMatchCtx struct {
	entries   []fuzzyFileEntry
	nameWidth int
	sizeWidth int
	timeWidth int
	now       time.Time
}

func newFilenameMatchCtx(entries []fuzzyFileEntry) *filenameMatchCtx {
	c := &filenameMatchCtx{entries: entries, now: time.Now()}
	for i := range entries {
		e := &entries[i]
		if n := len(e.Name); n > c.nameWidth {
			c.nameWidth = n
		}
		if n := len(formatFileSize(e.Size)); n > c.sizeWidth {
			c.sizeWidth = n
		}
		if n := len(formatModTime(e.ModTime, c.now)); n > c.timeWidth {
			c.timeWidth = n
		}
	}
	return c
}

func filenameProvider(ctx any, idx int) []byte {
	c, ok := ctx.(*filenameMatchCtx)
	if !ok || idx < 0 || idx >= len(c.entries) {
		return nil
	}
	return []byte(c.entries[idx].Name)
}

// filenameMatchFormatter writes "name  size  mtime" with padded columns.
func filenameMatchFormatter(out []byte, outSize int, idx int, ctx any) {
	c, ok := ctx.(*filenameMatchCtx)
	if !ok || idx < 0 || idx >= len(c.entries) {
		if outSize > 0 {
			out[0] = 0
		}
		return
	}
	e := c.entries[idx]
	size := formatFileSize(e.Size)
	mtime := formatModTime(e.ModTime, c.now)
	var b strings.Builder
	b.Grow(c.nameWidth + c.sizeWidth + c.timeWidth + 4)
	b.WriteString(e.Name)
	for i := len(e.Name); i < c.nameWidth; i++ {
		b.WriteByte(' ')
	}
	b.WriteString("  ")
	for i := len(size); i < c.sizeWidth; i++ {
		b.WriteByte(' ') // right-align size
	}
	b.WriteString(size)
	if mtime != "" {
		b.WriteString("  ")
		b.WriteString(mtime)
		for i := len(mtime); i < c.timeWidth; i++ {
			b.WriteByte(' ')
		}
	}
	n := copy(out, b.String())
	if n < outSize {
		out[n] = 0
	} else if outSize > 0 {
		out[outSize-1] = 0
	}
}

// formatFileSize returns a compact size like "0B", "4.2k", or "1.5M".
// Size < 0 means n/a (directories) and yields "".
func formatFileSize(n int64) string {
	if n < 0 {
		return ""
	}
	if n < 1024 {
		return strconv.FormatInt(n, 10) + "B"
	}
	f := float64(n)
	switch {
	case f < 1024*1024:
		return trimOneDecimal(f/1024) + "k"
	case f < 1024*1024*1024:
		return trimOneDecimal(f/(1024*1024)) + "M"
	default:
		return trimOneDecimal(f/(1024*1024*1024)) + "G"
	}
}

func trimOneDecimal(v float64) string {
	s := fmt.Sprintf("%.1f", v)
	return strings.TrimSuffix(s, ".0")
}

// formatModTime prefers relative labels for the last 30 days, else "Jan 2 2006".
func formatModTime(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h ago"
	case d < 30*24*time.Hour:
		return strconv.Itoa(int(d.Hours()/24)) + "d ago"
	default:
		return t.Format("Jan 2 2006")
	}
}

// collectFuzzyPaths lists the immediate children of dirpath for use in the
// fuzzy file picker.  Symlinks, hidden files/dirs, and binary artefacts are
// skipped.  Directories are returned with a trailing separator.
func collectFuzzyPaths(dirpath, prefix string) []fuzzyFileEntry {
	openDir := file.OpenDirFromPrompt(dirpath)
	absDir, err := filepath.Abs(openDir)
	if err != nil {
		absDir = filepath.Clean(openDir)
	}

	var paths []fuzzyFileEntry
	if filepath.Dir(absDir) != absDir {
		name := "../"
		if prefix != "" {
			name = filepath.Join(prefix, "..") + string(filepath.Separator)
		}
		ent := fuzzyFileEntry{Name: name, Size: -1}
		if info, err := os.Stat(filepath.Dir(absDir)); err == nil {
			ent.ModTime = info.ModTime()
		}
		paths = append(paths, ent)
	}

	entries, err := os.ReadDir(openDir)
	if err != nil {
		return paths
	}
	sep := string(filepath.Separator)
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		rel := name
		if prefix != "" {
			rel = filepath.Join(prefix, name)
		}
		if e.IsDir() {
			if name == ".git" || name == "__pycache__" || name == "node_modules" {
				continue
			}
			paths = append(paths, fuzzyFileEntry{Name: rel + sep, Size: -1, ModTime: info.ModTime()})
		} else if e.Type().IsRegular() {
			if shouldSkipFuzzyFile(name) {
				continue
			}
			paths = append(paths, fuzzyFileEntry{Name: rel, Size: info.Size(), ModTime: info.ModTime()})
		}
	}
	return paths
}

// lowerByte is a fast ASCII tolower helper.
func lowerByte(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c - 'A' + 'a'
	}
	return c
}

// filenameFuzzyScore scores name against query using the same algorithm as the
// C fuzzy_score function. Returns (matched, score); higher score is better.
func filenameFuzzyScore(name, query string) (bool, int) {
	score := 0
	prev := -1
	nameLen := len(name)
	for qi := 0; qi < len(query); qi++ {
		qc := lowerByte(query[qi])
		pos := prev + 1
		for pos < nameLen && lowerByte(name[pos]) != qc {
			pos++
		}
		if pos >= nameLen {
			return false, 0
		}
		score += 10
		if pos == 0 || name[pos-1] == '/' || name[pos-1] == '_' ||
			name[pos-1] == '-' || name[pos-1] == '.' {
			score += 12
		}
		if prev >= 0 {
			if pos == prev+1 {
				score += 15
			} else {
				score -= pos - prev - 1
			}
		} else {
			score -= pos
		}
		prev = pos
	}
	score -= nameLen / 4
	return true, score
}

// filenameFuzzyMatches returns the indices (into entries) of up to maxMatches
// entries that best match query, ordered by score descending.
func filenameFuzzyMatches(entries []fuzzyFileEntry, query string, maxMatches int) []int {
	return fuzzyTopN(len(entries), maxMatches, func(i int) (bool, int) {
		return filenameFuzzyScore(entries[i].Name, query)
	}, func(a, b int) bool {
		return entries[a].Name < entries[b].Name
	})
}

// completePromptFilename performs tab-completion on the current text in state:
// it opens the directory implied by the typed path, finds all entries with the
// matching prefix, replaces the typed portion with the longest common prefix,
// and appends "/" when exactly one match is a directory.
// Returns true if the text was changed.
func completePromptFilename(state *minibuffer.MinibufferState) bool {
	typed := string(state.Text)
	expanded := file.ExpandPath(typed)

	if typed == "~" {
		state.SetText([]byte("~/"))
		return true
	}

	tdir, _ := file.PromptSplit(typed)
	edir, eprefix := file.PromptSplit(expanded)
	openDir := file.OpenDirFromPrompt(edir)

	entries, err := os.ReadDir(openDir)
	if err != nil {
		return false
	}

	prefixLen := len(eprefix)
	common := ""
	matchCount := 0
	matchIsDir := false

	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		if prefixLen == 0 && strings.HasPrefix(name, ".") {
			continue
		}
		if !strings.HasPrefix(name, eprefix) {
			continue
		}
		isDir := e.IsDir()
		if matchCount == 0 {
			common = name
			matchIsDir = isDir
		} else {
			i := 0
			for i < len(common) && i < len(name) && common[i] == name[i] {
				i++
			}
			common = common[:i]
			matchIsDir = false
		}
		matchCount++
	}

	if matchCount == 0 {
		return false
	}

	newText := filepath.Join(tdir, common)
	if matchCount == 1 && matchIsDir {
		newText += string(filepath.Separator)
	}
	if len(newText) >= state.Nbuf {
		return false
	}
	state.SetText([]byte(newText))
	return true
}

// promptFormatWithCount formats a prompt string, inserting a "[sel+1/count]: "
// counter when the prompt ends with ": ".  It mirrors prompt_format_with_count
// in C and is used by the filename and command-palette prompts.
func promptFormatWithCount(prompt string, sel, count int) string {
	if count <= 0 {
		return prompt
	}
	plen := len(prompt)
	if plen >= 2 && prompt[plen-2] == ':' && prompt[plen-1] == ' ' {
		return fmt.Sprintf("%s [%d/%d]: ", prompt[:plen-2], sel+1, count)
	}
	return prompt
}

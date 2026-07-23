package tools

// Project-wide search (grep).

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/files"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	ignore "github.com/Sriram-PR/go-ignore"
)

const (
	GrepBufferName   = "*grep*"
	grepMaxFileSize  = 2 * 1024 * 1024
	grepMaxMatches   = 100_000
	grepWorkerCount  = 8
	grepBinarySample = 1024
)

// GrepLineData is stored in grep result line Metadata for jump-to-match.
type GrepLineData struct {
	Path   string
	Line   int
	Column int
}

type grepMatch struct {
	path   string // absolute
	line   int
	column int // 1-based byte offset
	text   string
}

type grepSearchResult struct {
	matches   []grepMatch
	truncated bool
	err       error
}

func grepSearchRoot() (string, error) {
	start := ""
	if buf := buffer.All.Current; buf != nil {
		if fname := buf.FileName; fname != "" {
			start = filepath.Dir(files.NormalizePath(fname))
		}
	}
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		start = cwd
	}
	if root, ok := files.FindDirWalkUp(start, ".git"); ok {
		return root, nil
	}
	return start, nil
}

func grepCompilePattern(pattern string) (*regexp.Regexp, error) {
	smartCase := true
	for _, r := range pattern {
		if unicode.IsUpper(r) {
			smartCase = false
			break
		}
	}
	if smartCase {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

func grepIsBinary(data []byte) bool {
	limit := len(data)
	if limit > grepBinarySample {
		limit = grepBinarySample
	}
	return bytes.IndexByte(data[:limit], 0) >= 0
}

func grepSearchFile(path string, re *regexp.Regexp, out chan<- grepMatch, limit *int, limitMu *sync.Mutex) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > grepMaxFileSize {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil || grepIsBinary(data) {
		return
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		locs := re.FindAllIndex(line, -1)
		for _, loc := range locs {
			limitMu.Lock()
			if *limit >= grepMaxMatches {
				limitMu.Unlock()
				return
			}
			*limit++
			limitMu.Unlock()

			text := string(line)
			if len(text) > 512 {
				text = text[:512]
			}
			out <- grepMatch{
				path:   abs,
				line:   lineNum,
				column: loc[0] + 1,
				text:   text,
			}
		}
	}
}

func grepProjectSearch(ctx context.Context, root, pattern string) grepSearchResult {
	re, err := grepCompilePattern(pattern)
	if err != nil {
		return grepSearchResult{err: err}
	}

	fileCh := make(chan string, 256)
	matchCh := make(chan grepMatch, 256)
	var wg sync.WaitGroup
	var matchCount int
	var limitMu sync.Mutex
	truncated := false

	for i := 0; i < grepWorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range fileCh {
				select {
				case <-ctx.Done():
					return
				default:
				}
				grepSearchFile(path, re, matchCh, &matchCount, &limitMu)
			}
		}()
	}

	go func() {
		defer close(fileCh)
		for path, walkErr := range ignore.RepoFiles(root, ignore.MatcherOptions{}) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if walkErr != nil {
				continue
			}
			fileCh <- path
		}
	}()

	go func() {
		wg.Wait()
		close(matchCh)
	}()

	matches := make([]grepMatch, 0, 64)
collecting:
	for {
		select {
		case <-ctx.Done():
			return grepSearchResult{matches: matches, truncated: truncated, err: ctx.Err()}
		case m, ok := <-matchCh:
			if !ok {
				break collecting
			}
			matches = append(matches, m)
			if len(matches) >= grepMaxMatches {
				truncated = true
				break collecting
			}
		}
	}
	if matchCount >= grepMaxMatches {
		truncated = true
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].path != matches[j].path {
			return matches[i].path < matches[j].path
		}
		if matches[i].line != matches[j].line {
			return matches[i].line < matches[j].line
		}
		return matches[i].column < matches[j].column
	})

	return grepSearchResult{matches: matches, truncated: truncated}
}

func grepDisplayPath(root, abs string) string {
	if rel, err := filepath.Rel(root, abs); err == nil && rel != "" && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return abs
}

func grepFillBuffer(buf *buffer.Buffer, root string, matches []grepMatch, pattern string, truncated bool) (int, bool) {
	buf.IsChanged = false
	buf.Clear()
	buf.FileName = ""
	buf.FileModTime = time.Time{}
	buf.LangMode = buffer.LModeMarkdown

	var matchCount int
	var fileCount int
	currentFile := ""
	var lastLine int
	haveLastLine := false

	for _, m := range matches {
		displayPath := grepDisplayPath(root, m.path)
		if displayPath != currentFile {
			currentFile = displayPath
			haveLastLine = false
			if matchCount > 0 {
				if buf.AppendLineBytes(nil) == nil {
					return 0, false
				}
			}
			header := []byte("## " + displayPath)
			if buf.AppendLineBytes(header) == nil {
				return 0, false
			}
			fileCount++
		}
		if haveLastLine && lastLine == m.line {
			continue
		}

		lineText := fmt.Sprintf("L%d: %s", m.line, m.text)
		line := buf.AppendLineBytes([]byte(lineText))
		if line == nil {
			return 0, false
		}
		line.Metadata = &GrepLineData{
			Path:   m.path,
			Line:   m.line,
			Column: m.column,
		}
		lastLine = m.line
		haveLastLine = true
		matchCount++
	}

	summary := fmt.Sprintf("# %d matches across %d files for `%s`", matchCount, fileCount, pattern)
	if summaryLine := buf.Line(1); summaryLine != nil {
		begin := buffer.MakeLocation(1, 0)
		end := buffer.MakeLocation(1, summaryLine.Len())
		if err := buf.SetText(nil, begin, end, []byte(summary), nil); err != nil {
			return 0, false
		}
	}

	if matchCount == 0 {
		msg := fmt.Sprintf("[no matches for: %s]", pattern)
		if buf.AppendLineBytes([]byte(msg)) == nil {
			return 0, false
		}
	}
	if truncated {
		msg := "[results truncated]"
		if buf.AppendLineBytes([]byte(msg)) == nil {
			return 0, false
		}
	}

	buf.IsChanged = false
	buf.Cursor = buffer.Location{Line: 1, Offset: 0}
	buf.Mark = buffer.Location{Line: 0, Offset: 0}
	return matchCount, true
}

func grepEnsureBuffer() *buffer.Buffer {
	if buf := buffer.Find(GrepBufferName); buf != nil {
		return buf
	}
	buf := buffer.Create()
	if buf == nil {
		return nil
	}
	buf.Name = GrepBufferName
	return buf
}

// RunGrep searches the project and opens the *grep* results buffer.
func RunGrep() bool {
	askString("grep: ", "", func(pattern string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes {
			return
		}
		if pattern == "" {
			mbWrite("[empty pattern]")
			return
		}
		mbHistoryAdd(pattern)

		root, err := grepSearchRoot()
		if err != nil {
			mbWrite("[grep failed]")
			return
		}
		_ = StartBackgroundGrep(root, pattern)
	})
	return true
}

// VisitGrepMatch jumps to the match at the current line in the *grep* buffer.
func VisitGrepMatch() bool {
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil || buf.Name != GrepBufferName {
		return false
	}
	line := buf.Line(win.Cursor.Line)
	if line == nil || line.Len() == 0 || line.Metadata == nil {
		return false
	}
	data, ok := line.Metadata.(*GrepLineData)
	if !ok || data == nil {
		return false
	}

	markPushCurrent()
	return fileVisitLocation(data.Path, data.Line, data.Column)
}

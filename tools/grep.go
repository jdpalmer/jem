package tools

// grep.go - Project-wide search (translation of src/cmd_grep.c)

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/fileio"
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
	Line   uint
	Column uint
}

type grepMatch struct {
	path   string // absolute
	line   uint
	column uint // 1-based byte offset
	text   string
}

type grepSearchResult struct {
	matches   []grepMatch
	truncated bool
	err       error
}

func grepSearchRoot() (string, error) {
	start := ""
	if bp := app.State.CurrentBuffer; bp != nil {
		if fname := bp.FileName; fname != "" {
			start = filepath.Dir(fileio.NormalizePath(fname))
		}
	}
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		start = cwd
	}
	if root, ok := fileio.FindDirWalkUp(start, ".git"); ok {
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
	lineNum := uint(0)
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
				column: uint(loc[0]) + 1,
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

func grepFillBuffer(bp *Buffer, root string, matches []grepMatch, pattern string, truncated bool) (uint, bool) {
	if bp == nil {
		return 0, false
	}

	bp.IsChanged = false
	buffer.Clear(bp)
	bp.FileName = ""
	bp.FileMtime = time.Time{}
	bp.LangMode = LModeMarkdown
	if buffer.AppendLineBytes(bp, nil, 0) == nil {
		return 0, false
	}

	var matchCount uint
	var fileCount uint
	currentFile := ""
	var lastLine uint
	haveLastLine := false

	for _, m := range matches {
		displayPath := grepDisplayPath(root, m.path)
		if displayPath != currentFile {
			currentFile = displayPath
			haveLastLine = false
			if matchCount > 0 {
				if buffer.AppendLineBytes(bp, nil, 0) == nil {
					return 0, false
				}
			}
			header := []byte("## " + displayPath)
			if buffer.AppendLineBytes(bp, header, uint(len(header))) == nil {
				return 0, false
			}
			fileCount++
		}
		if haveLastLine && lastLine == m.line {
			continue
		}

		lineText := fmt.Sprintf("L%d: %s", m.line, m.text)
		lp := buffer.AppendLineBytes(bp, []byte(lineText), uint(len(lineText)))
		if lp == nil {
			return 0, false
		}
		lp.Metadata = &GrepLineData{
			Path:   m.path,
			Line:   m.line,
			Column: m.column,
		}
		lastLine = m.line
		haveLastLine = true
		matchCount++
	}

	summary := fmt.Sprintf("# %d matches across %d files for `%s`", matchCount, fileCount, pattern)
	if summaryLine := buffer.GetLine(bp, 1); summaryLine != nil {
		begin := buffer.MakeLocation(1, 0)
		end := buffer.MakeLocation(1, buffer.LineLength(summaryLine))
		if !buffer.SetText(bp, nil, begin, end, []byte(summary), uint(len(summary)), nil) {
			return 0, false
		}
	}

	if matchCount == 0 {
		msg := fmt.Sprintf("[no matches for: %s]", pattern)
		if buffer.AppendLineBytes(bp, []byte(msg), uint(len(msg))) == nil {
			return 0, false
		}
	}
	if truncated {
		msg := "[results truncated]"
		if buffer.AppendLineBytes(bp, []byte(msg), uint(len(msg))) == nil {
			return 0, false
		}
	}

	bp.IsChanged = false
	bp.Cursor = Location{Line: 1, Offset: 0}
	bp.Mark = Location{Line: 0, Offset: 0}
	return matchCount, true
}

func grepEnsureBuffer() *Buffer {
	if bp := app.BufferFind(GrepBufferName); bp != nil {
		return bp
	}
	bp := app.BufferCreate(&app.State.EditorRuntimeState)
	if bp == nil {
		return nil
	}
	bp.Name = GrepBufferName
	return bp
}

// RunGrep searches the project and opens the *grep* results buffer.
func RunGrep() bool {
	pattern, pr := mbReadString("grep: ", "")
	if pr != PromptResultYes {
		return false
	}
	if pattern == "" {
		mbWrite("[empty pattern]")
		return false
	}
	mbHistoryAdd(pattern)

	root, err := grepSearchRoot()
	if err != nil {
		mbWrite("[grep failed]")
		return false
	}

	return StartBackgroundGrep(root, pattern)
}

// VisitGrepMatch jumps to the match at the current line in the *grep* buffer.
func VisitGrepMatch() bool {
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil || bp.Name != GrepBufferName {
		return false
	}
	lp := buffer.GetLine(bp, wp.Cursor.Line)
	if lp == nil || buffer.LineLength(lp) == 0 || lp.Metadata == nil {
		return false
	}
	data, ok := lp.Metadata.(*GrepLineData)
	if !ok || data == nil {
		return false
	}

	markPushCurrent()
	return fileVisitLocation(data.Path, uint32(data.Line), uint32(data.Column))
}

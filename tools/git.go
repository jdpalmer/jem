package tools

// Git modeline text and gutter diff markers.
//
// Uses the installed git binary via os/exec, matching the C implementation.
// This preserves user git config, hooks, and porcelain output semantics.

import (
	"bytes"
	"fmt"
	"github.com/jdpalmer/jem/buffer"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type gitModelineCache struct {
	buffer      *buffer.Buffer
	valid       bool
	hasRepo     bool
	refreshedAt int64
	fileName    string
	text        string
	lineDiffs   []uint8
	diffCount   uint
}

var gitModelineCaches []gitModelineCache

func gitNextLine(data []byte, start int) (int, []byte) {
	if start >= len(data) {
		return len(data), nil
	}
	lineEnd := start
	for lineEnd < len(data) && data[lineEnd] != '\n' && data[lineEnd] != '\r' {
		lineEnd++
	}
	line := data[start:lineEnd]
	for lineEnd < len(data) && (data[lineEnd] == '\n' || data[lineEnd] == '\r') {
		lineEnd++
	}
	return lineEnd, line
}

func gitSetLineDiff(cache *gitModelineCache, lineNumber uint, marker GitLineDiff) {
	if cache == nil || lineNumber == 0 || lineNumber > cache.diffCount {
		return
	}
	slot := &cache.lineDiffs[lineNumber-1]
	if uint8(marker) > *slot {
		*slot = uint8(marker)
	}
}

func gitRun(argv []string, maxOut int) (stdout string, exitCode int, ran bool) {
	if len(argv) == 0 {
		return "", -1, false
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if maxOut > 0 && out.Len() > maxOut {
				return string(out.Bytes()[:maxOut]), exitErr.ExitCode(), true
			}
			return out.String(), exitErr.ExitCode(), true
		}
		return "", -1, false
	}
	if maxOut > 0 && out.Len() > maxOut {
		return string(out.Bytes()[:maxOut]), 0, true
	}
	return out.String(), 0, true
}

func gitModelineCacheForBuffer(buf *buffer.Buffer) *gitModelineCache {
	if buf == nil || buf.FileName == "" {
		return nil
	}

	var cache *gitModelineCache
	for i := range gitModelineCaches {
		if gitModelineCaches[i].buffer == buf {
			cache = &gitModelineCaches[i]
			break
		}
		if cache == nil && gitModelineCaches[i].buffer == nil {
			cache = &gitModelineCaches[i]
		}
	}
	if cache == nil {
		if len(gitModelineCaches) >= buffer.MaxBuffers {
			return nil
		}
		gitModelineCaches = append(gitModelineCaches, gitModelineCache{})
		cache = &gitModelineCaches[len(gitModelineCaches)-1]
	}

	if cache.buffer == nil {
		cache.buffer = buf
		cache.valid = false
		cache.hasRepo = false
		cache.refreshedAt = 0
		cache.fileName = ""
		cache.text = ""
		cache.lineDiffs = nil
		cache.diffCount = 0
	}

	now := time.Now().Unix()
	fname := buf.FileName
	if !cache.valid || cache.fileName != fname || cache.refreshedAt != now {
		gitRefreshCache(cache, buf, fname, now)
	}
	return cache
}

func gitRefreshCache(cache *gitModelineCache, buf *buffer.Buffer, fname string, now int64) {
	cache.valid = true
	cache.hasRepo = false
	cache.refreshedAt = now
	cache.fileName = fname
	cache.text = ""
	cache.lineDiffs = nil
	cache.diffCount = buf.LineCount
	if cache.diffCount > 0 {
		cache.lineDiffs = make([]uint8, cache.diffCount)
	}

	dir := filepath.Dir(fname)
	if dir == "" {
		dir = "."
	}
	basename := filepath.Base(fname)

	statusOut, statusExit, ran := gitRun([]string{
		"git", "-C", dir,
		"status", "--branch", "--porcelain=2", "--untracked-files=all",
		"--", basename,
	}, 16384)
	if !ran || statusExit != 0 {
		return
	}

	branch := ""
	oid := ""
	ahead := 0
	behind := 0
	fileUntracked := false
	fileIndexStatus := byte('.')
	fileWorktreeStatus := byte('.')

	pos := 0
	status := []byte(statusOut)
	for pos < len(status) {
		var line []byte
		pos, line = gitNextLine(status, pos)
		if len(line) == 0 {
			continue
		}
		text := string(line)
		switch {
		case strings.HasPrefix(text, "# branch.head "):
			branch = text[len("# branch.head "):]
		case strings.HasPrefix(text, "# branch.oid "):
			oid = text[len("# branch.oid "):]
		case strings.HasPrefix(text, "# branch.ab "):
			rest := text[len("# branch.ab "):]
			var a, b int
			if n, _ := fmt.Sscanf(rest, "+%d -%d", &a, &b); n == 2 {
				ahead = a
				behind = b
			}
		case (text[0] == '1' || text[0] == '2') && len(text) > 3 && text[1] == ' ':
			fileIndexStatus = text[2]
			fileWorktreeStatus = text[3]
		case text[0] == '?':
			fileUntracked = true
		}
	}

	cache.hasRepo = true

	shortOID := oid
	if len(shortOID) > 7 {
		shortOID = shortOID[:7]
	}

	head := "git"
	switch {
	case branch == "(detached)":
		head = "@" + shortOID
	case branch != "":
		head = branch
	case oid != "":
		head = "@" + shortOID
	}

	dirty := ""
	if fileUntracked {
		dirty = "[?]"
		for i := uint(1); i <= cache.diffCount; i++ {
			gitSetLineDiff(cache, i, GitLineDiffAdded)
		}
	} else if fileIndexStatus != '.' || fileWorktreeStatus != '.' {
		dirty = fmt.Sprintf("[%c%c]", fileIndexStatus, fileWorktreeStatus)
	}

	aheadPart := ""
	if ahead > 0 {
		aheadPart = "^" + strconv.Itoa(ahead)
	}
	behindPart := ""
	if behind > 0 {
		behindPart = "v" + strconv.Itoa(behind)
	}
	cache.text = head + dirty + aheadPart + behindPart

	if fileUntracked {
		return
	}

	diffCap := int(cache.diffCount)*48 + 4096
	if diffCap < 16384 {
		diffCap = 16384
	} else if diffCap > 1048576 {
		diffCap = 1048576
	}

	diffOut, diffExit, ran := gitRun([]string{
		"git", "-C", dir,
		"diff", "--no-color", "--no-ext-diff", "--unified=0", "HEAD",
		"--", basename,
	}, diffCap)
	if !ran || diffExit != 0 {
		return
	}

	pos = 0
	diff := []byte(diffOut)
	for pos < len(diff) {
		var line []byte
		pos, line = gitNextLine(diff, pos)
		if len(line) == 0 {
			continue
		}
		text := string(line)
		if !strings.HasPrefix(text, "@@ ") {
			continue
		}
		_, oldCount, newStart, newCount, ok := gitParseHunkHeader(text)
		if !ok {
			continue
		}
		if oldCount == 0 && newCount > 0 {
			for i := 0; i < newCount; i++ {
				lineNumber := newStart + i
				if lineNumber > 0 {
					gitSetLineDiff(cache, uint(lineNumber), GitLineDiffAdded)
				}
			}
			continue
		}
		if newCount == 0 && oldCount > 0 {
			target := newStart
			if target <= 0 {
				target = 1
			}
			if uint(target) > cache.diffCount && cache.diffCount > 0 {
				target = int(cache.diffCount)
			}
			gitSetLineDiff(cache, uint(target), GitLineDiffDeleted)
			continue
		}
		for i := 0; i < newCount; i++ {
			lineNumber := newStart + i
			if lineNumber > 0 {
				gitSetLineDiff(cache, uint(lineNumber), GitLineDiffModified)
			}
		}
		if oldCount > newCount {
			lineNumber := newStart + newCount
			if lineNumber > 0 {
				gitSetLineDiff(cache, uint(lineNumber), GitLineDiffDeleted)
			}
		}
	}
}

func gitParseHunkHeader(line string) (oldStart, oldCount, newStart, newCount int, ok bool) {
	oldCount = 1
	newCount = 1
	if n, _ := fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@", &oldStart, &oldCount, &newStart, &newCount); n == 4 {
		return oldStart, oldCount, newStart, newCount, true
	}
	if n, _ := fmt.Sscanf(line, "@@ -%d +%d,%d @@", &oldStart, &newStart, &newCount); n == 3 {
		return oldStart, 1, newStart, newCount, true
	}
	if n, _ := fmt.Sscanf(line, "@@ -%d,%d +%d @@", &oldStart, &oldCount, &newStart); n == 3 {
		return oldStart, oldCount, newStart, 1, true
	}
	if n, _ := fmt.Sscanf(line, "@@ -%d +%d @@", &oldStart, &newStart); n == 2 {
		return oldStart, 1, newStart, 1, true
	}
	return 0, 0, 0, 0, false
}

// GitModelineText returns branch/status text for the modeline, or "" when unavailable.
func GitModelineText(buf *buffer.Buffer) string {
	cache := gitModelineCacheForBuffer(buf)
	if cache == nil || !cache.hasRepo || cache.text == "" {
		return ""
	}
	return cache.text
}

// GitLineDiffAt returns the gutter diff marker for a buffer line.
func GitLineDiffAt(buf *buffer.Buffer, lineNumber uint) GitLineDiff {
	cache := gitModelineCacheForBuffer(buf)
	if cache == nil || !cache.hasRepo || lineNumber == 0 || lineNumber > cache.diffCount {
		return GitLineDiffNone
	}
	return GitLineDiff(cache.lineDiffs[lineNumber-1])
}

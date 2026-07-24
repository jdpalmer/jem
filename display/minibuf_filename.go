package display

// Filename prompt helpers (path list / fuzzy matching).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jdpalmer/jem/file"
	"github.com/jdpalmer/jem/minibuffer"
)

func shouldSkipFuzzyFile(name string) bool {
	return strings.HasSuffix(name, ".o") ||
		strings.HasSuffix(name, ".exe") ||
		strings.HasSuffix(name, ".pyc")
}

// collectFuzzyPaths lists the immediate children of dirpath for use in the
// fuzzy file picker.  Symlinks, hidden files/dirs, and binary artefacts are
// skipped.  Directories are returned with a trailing separator.
func collectFuzzyPaths(dirpath, prefix string) []string {
	openDir := file.OpenDirFromPrompt(dirpath)
	absDir, err := filepath.Abs(openDir)
	if err != nil {
		absDir = filepath.Clean(openDir)
	}

	var paths []string
	if filepath.Dir(absDir) != absDir {
		if prefix == "" {
			paths = append(paths, "../")
		} else {
			paths = append(paths, filepath.Join(prefix, "..")+string(filepath.Separator))
		}
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
			paths = append(paths, rel+sep)
		} else if e.Type().IsRegular() {
			if shouldSkipFuzzyFile(name) {
				continue
			}
			paths = append(paths, rel)
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

// filenameFuzzyMatches returns the indices (into paths) of up to maxMatches
// entries that best match query, ordered by score descending.
func filenameFuzzyMatches(paths []string, query string, maxMatches int) []int {
	return fuzzyTopN(len(paths), maxMatches, func(i int) (bool, int) {
		return filenameFuzzyScore(paths[i], query)
	}, func(a, b int) bool {
		return paths[a] < paths[b]
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

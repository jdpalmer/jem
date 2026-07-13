package fileio

import (
	"os"
	"path/filepath"
	"strings"
)

// PromptPathCapacity is the maximum path length accepted from filename prompts.
const PromptPathCapacity = 4096

func ExpandPath(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		if len(path) == 1 || path[1] == '/' || path[1] == '\\' {
			home, err := os.UserHomeDir()
			if err == nil {
				suffix := ""
				if len(path) > 1 {
					suffix = path[1:]
				}
				return filepath.Clean(filepath.Join(home, suffix))
			}
		}
	}
	return filepath.Clean(os.ExpandEnv(path))
}

// NormalizePath expands ~/$VAR, cleans, and resolves to an absolute path when possible.
func NormalizePath(path string) string {
	if path == "" {
		return ""
	}
	expanded := ExpandPath(path)
	if abs, err := filepath.Abs(expanded); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(expanded)
}

func PathsEqual(a, b string) bool {
	if a == b {
		return true
	}
	return NormalizePath(a) == NormalizePath(b)
}

// FindFileWalkUp searches upward from start for a file named marker in each directory.
func FindFileWalkUp(start, marker string) (string, bool) {
	dir := start
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", false
		}
		dir = cwd
	}
	dir = filepath.Clean(dir)
	for {
		candidate := filepath.Join(dir, marker)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// FindDirWalkUp searches upward from start for a directory named marker.
// Returns the directory that contains marker (not the marker path itself).
func FindDirWalkUp(start, marker string) (string, bool) {
	dir := start
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", false
		}
		dir = cwd
	}
	dir = filepath.Clean(dir)
	for {
		candidate := filepath.Join(dir, marker)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// PromptSplit splits a prompt path into directory prefix (with trailing separator) and basename.
func PromptSplit(path string) (dirPart, namePart string) {
	if path == "" {
		return "", ""
	}
	return filepath.Split(filepath.FromSlash(path))
}

// OpenDirFromPrompt resolves a prompt directory prefix to a filesystem path for os.ReadDir.
func OpenDirFromPrompt(dirPart string) string {
	if dirPart == "" {
		return "."
	}
	expanded := ExpandPath(dirPart)
	if expanded == "" {
		return string(filepath.Separator)
	}
	return expanded
}

// ContractHome rewrites absPath under the user home directory as ~/... when possible.
func ContractHome(absPath string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return absPath
	}
	absPath = filepath.Clean(absPath)
	home = filepath.Clean(home)
	if absPath == home {
		return "~/"
	}
	rel, err := filepath.Rel(home, absPath)
	if err != nil || rel == "" || strings.HasPrefix(rel, "..") {
		return absPath
	}
	return "~/" + filepath.ToSlash(rel) + string(filepath.Separator)
}

// PromptParentDir returns the directory prefix one level above dirPart in prompt notation.
func PromptParentDir(dirPart string) string {
	if dirPart == "" {
		return ""
	}
	openDir := OpenDirFromPrompt(dirPart)
	parent := filepath.Dir(openDir)
	if filepath.Clean(parent) == filepath.Clean(openDir) {
		return dirPart
	}
	if strings.HasPrefix(dirPart, "~/") || dirPart == "~/" {
		return ContractHome(parent)
	}
	if filepath.IsAbs(filepath.FromSlash(dirPart)) || (len(dirPart) > 1 && dirPart[1] == ':') {
		return parent + string(filepath.Separator)
	}
	trimmed := strings.TrimRight(dirPart, `/\`)
	if idx := strings.LastIndexAny(trimmed, `/\`); idx >= 0 {
		return trimmed[:idx+1]
	}
	return ""
}

func ApplyFilenameSelection(dirPart, selected string) string {
	if selected == "../" {
		return PromptParentDir(dirPart)
	}
	if dirPart == "" {
		return selected
	}
	return filepath.Join(dirPart, selected)
}

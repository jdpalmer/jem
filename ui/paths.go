package ui

import "github.com/jdpalmer/jem/fileio"

// PromptPathCapacity is the maximum path length accepted from filename prompts.
const PromptPathCapacity = fileio.PromptPathCapacity

func fileExpandPath(path string) string {
	return fileio.ExpandPath(path)
}

// fileNormalizePath expands ~/$VAR, cleans, and resolves to an absolute path when possible.
func fileNormalizePath(path string) string {
	return fileio.NormalizePath(path)
}

func filePathsEqual(a, b string) bool {
	return fileio.PathsEqual(a, b)
}

// findFileWalkUp searches upward from start for a file named marker in each directory.
func findFileWalkUp(start, marker string) (string, bool) {
	return fileio.FindFileWalkUp(start, marker)
}

// findDirWalkUp searches upward from start for a directory named marker.
// Returns the directory that contains marker (not the marker path itself).
func findDirWalkUp(start, marker string) (string, bool) {
	return fileio.FindDirWalkUp(start, marker)
}

// pathPromptSplit splits a prompt path into directory prefix (with trailing separator) and basename.
func pathPromptSplit(path string) (dirPart, namePart string) {
	return fileio.PromptSplit(path)
}

// pathOpenDirFromPrompt resolves a prompt directory prefix to a filesystem path for os.ReadDir.
func pathOpenDirFromPrompt(dirPart string) string {
	return fileio.OpenDirFromPrompt(dirPart)
}

// pathContractHome rewrites absPath under the user home directory as ~/… when possible.
func pathContractHome(absPath string) string {
	return fileio.ContractHome(absPath)
}

// pathPromptParentDir returns the directory prefix one level above dirPart in prompt notation.
func pathPromptParentDir(dirPart string) string {
	return fileio.PromptParentDir(dirPart)
}

func applyFilenameSelection(dirPart, selected string) string {
	return fileio.ApplyFilenameSelection(dirPart, selected)
}

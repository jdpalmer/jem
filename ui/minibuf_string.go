package ui

import (
	"bytes"
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/fileio"
)

func promptStringFromBuf(buf []byte) string {
	n := bytes.IndexByte(buf, 0)
	if n < 0 {
		n = len(buf)
	}
	return string(buf[:n])
}

// mbReadString prompts for a line of input and returns it as a string.
func mbReadString(prompt, initial string) (string, app.PromptResult) {
	return mbReadStringCap(prompt, initial, app.PatternCapacity)
}

// mbReadStringCap is like mbReadString but allows a larger capacity (e.g. filenames).
func mbReadStringCap(prompt, initial string, capacity int) (string, app.PromptResult) {
	buf := make([]byte, capacity)
	var initialBuf []byte
	if initial != "" {
		initialBuf = []byte(initial)
	}
	pr := mbReadInitial(prompt, buf, capacity, initialBuf)
	if pr != app.PromptResultYes {
		return "", pr
	}
	return promptStringFromBuf(buf), pr
}

// mbReadFilenameString prompts for a filename with tab completion.
func mbReadFilenameString(prompt, initial string) (string, app.PromptResult) {
	buf := make([]byte, fileio.PromptPathCapacity)
	if initial != "" {
		copy(buf, initial)
	}
	pr := mbReadFilename(prompt, buf, fileio.PromptPathCapacity)
	if pr != app.PromptResultYes {
		return "", pr
	}
	return promptStringFromBuf(buf), pr
}

// mbReadFuzzyListString runs a fuzzy list picker and returns the selected label.
func mbReadFuzzyListString(prompt string, provider app.MbNameProviderFn, providerCtx any, providerCount uint) (string, app.PromptResult) {
	buf := make([]byte, app.PatternCapacity)
	pr := mbReadFuzzyList(prompt, provider, providerCtx, providerCount, buf, len(buf))
	if pr != app.PromptResultYes {
		return "", pr
	}
	return promptStringFromBuf(buf), pr
}

// mbReadFuzzyListExString is like mbReadFuzzyListString with a custom formatter.
func mbReadFuzzyListExString(prompt string, provider app.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter app.MbMatchFormatter, displayCtx any) (string, app.PromptResult) {
	buf := make([]byte, app.PatternCapacity)
	pr := mbReadFuzzyListEx(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx, buf, len(buf))
	if pr != app.PromptResultYes {
		return "", pr
	}
	return promptStringFromBuf(buf), pr
}

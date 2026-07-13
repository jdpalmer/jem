package ui

import "bytes"

func promptStringFromBuf(buf []byte) string {
	n := bytes.IndexByte(buf, 0)
	if n < 0 {
		n = len(buf)
	}
	return string(buf[:n])
}

// mbReadString prompts for a line of input and returns it as a string.
func mbReadString(prompt, initial string) (string, PromptResult) {
	return mbReadStringCap(prompt, initial, PatternCapacity)
}

// mbReadStringCap is like mbReadString but allows a larger capacity (e.g. filenames).
func mbReadStringCap(prompt, initial string, capacity int) (string, PromptResult) {
	buf := make([]byte, capacity)
	var initialBuf []byte
	if initial != "" {
		initialBuf = []byte(initial)
	}
	pr := mbReadInitial(prompt, buf, capacity, initialBuf)
	if pr != PromptResultYes {
		return "", pr
	}
	return promptStringFromBuf(buf), pr
}

// mbReadFilenameString prompts for a filename with tab completion.
func mbReadFilenameString(prompt, initial string) (string, PromptResult) {
	buf := make([]byte, PromptPathCapacity)
	if initial != "" {
		copy(buf, initial)
	}
	pr := mbReadFilename(prompt, buf, PromptPathCapacity)
	if pr != PromptResultYes {
		return "", pr
	}
	return promptStringFromBuf(buf), pr
}

// mbReadFuzzyListString runs a fuzzy list picker and returns the selected label.
func mbReadFuzzyListString(prompt string, provider MbNameProviderFn, providerCtx any, providerCount uint) (string, PromptResult) {
	buf := make([]byte, PatternCapacity)
	pr := mbReadFuzzyList(prompt, provider, providerCtx, providerCount, buf, len(buf))
	if pr != PromptResultYes {
		return "", pr
	}
	return promptStringFromBuf(buf), pr
}

// mbReadFuzzyListExString is like mbReadFuzzyListString with a custom formatter.
func mbReadFuzzyListExString(prompt string, provider MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter MbMatchFormatter, displayCtx any) (string, PromptResult) {
	buf := make([]byte, PatternCapacity)
	pr := mbReadFuzzyListEx(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx, buf, len(buf))
	if pr != PromptResultYes {
		return "", pr
	}
	return promptStringFromBuf(buf), pr
}

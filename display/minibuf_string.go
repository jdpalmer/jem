package display

import (
	"bytes"
	"github.com/jdpalmer/jem/minibuffer"

	"github.com/jdpalmer/jem/files"
)

func promptStringFromBuf(buf []byte) string {
	n := bytes.IndexByte(buf, 0)
	if n < 0 {
		n = len(buf)
	}
	return string(buf[:n])
}

// MBReadString prompts for a line of input and returns it as a string (blocking).
func MBReadString(prompt, initial string) (string, minibuffer.PromptResult) {
	return MBReadStringCap(prompt, initial, PatternCapacity)
}

// MBReadStringCap is like MBReadString but allows a larger capacity (blocking).
func MBReadStringCap(prompt, initial string, capacity int) (string, minibuffer.PromptResult) {
	buf := make([]byte, capacity)
	if pr, played := TryMacroPlayPrompt(buf); played {
		return promptStringFromBuf(buf), pr
	}
	p := NewStringPrompt(prompt, initial, capacity)
	p.OpenBlocking()
	defer p.Close()
	for {
		k, ok := WaitKey()
		if !ok {
			return "", minibuffer.PromptResultAbort
		}
		done, text, pr := p.HandleKey(k)
		if done {
			return text, pr
		}
	}
}

// MBReadFilenameString prompts for a filename with tab completion (blocking).
func MBReadFilenameString(prompt, initial string) (string, minibuffer.PromptResult) {
	p := NewFilenamePrompt(prompt, initial, files.PromptPathCapacity)
	p.OpenBlocking()
	defer p.Close()
	for {
		k, ok := WaitKey()
		if !ok {
			return "", minibuffer.PromptResultAbort
		}
		done, text, pr := p.HandleKey(k)
		if done {
			return text, pr
		}
	}
}

// MBReadFuzzyListString runs a fuzzy list picker (blocking).
func MBReadFuzzyListString(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount uint) (string, minibuffer.PromptResult) {
	return MBReadFuzzyListExString(prompt, provider, providerCtx, providerCount, nil, nil)
}

// MBReadFuzzyListExString is like MBReadFuzzyListString with a custom formatter (blocking).
func MBReadFuzzyListExString(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter minibuffer.MbMatchFormatter, displayCtx any) (string, minibuffer.PromptResult) {
	buf := make([]byte, PatternCapacity)
	if pr, played := TryMacroPlayPrompt(buf); played {
		return promptStringFromBuf(buf), pr
	}
	p := NewFuzzyPrompt(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx, PatternCapacity)
	p.OpenBlocking()
	defer p.Close()
	for {
		k, ok := WaitKey()
		if !ok {
			return "", minibuffer.PromptResultAbort
		}
		done, text, pr := p.HandleKey(k)
		if done {
			return text, pr
		}
	}
}

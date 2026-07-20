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

// AskString runs an async string prompt via the editor hook when installed.
func AskString(prompt, initial string, onDone func(string, minibuffer.PromptResult)) {
	if PackageHooks.AskString != nil {
		PackageHooks.AskString(prompt, initial, onDone)
		return
	}
	text, pr := MBReadString(prompt, initial)
	if onDone != nil {
		onDone(text, pr)
	}
}

// AskStringCap runs an async capped string prompt via the editor hook.
func AskStringCap(prompt, initial string, capacity int, onDone func(string, minibuffer.PromptResult)) {
	if PackageHooks.AskStringCap != nil {
		PackageHooks.AskStringCap(prompt, initial, capacity, onDone)
		return
	}
	text, pr := MBReadStringCap(prompt, initial, capacity)
	if onDone != nil {
		onDone(text, pr)
	}
}

// AskFuzzy runs an async fuzzy prompt via the editor hook.
func AskFuzzy(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount uint, onDone func(string, minibuffer.PromptResult)) {
	if PackageHooks.AskFuzzy != nil {
		PackageHooks.AskFuzzy(prompt, provider, providerCtx, providerCount, onDone)
		return
	}
	text, pr := MBReadFuzzyListString(prompt, provider, providerCtx, providerCount)
	if onDone != nil {
		onDone(text, pr)
	}
}

// AskFuzzyEx runs an async fuzzy prompt with formatter via the editor hook.
func AskFuzzyEx(prompt string, provider minibuffer.MbNameProviderFn, providerCtx any, providerCount uint, displayFormatter minibuffer.MbMatchFormatter, displayCtx any, onDone func(string, minibuffer.PromptResult)) {
	if PackageHooks.AskFuzzyEx != nil {
		PackageHooks.AskFuzzyEx(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx, onDone)
		return
	}
	text, pr := MBReadFuzzyListExString(prompt, provider, providerCtx, providerCount, displayFormatter, displayCtx)
	if onDone != nil {
		onDone(text, pr)
	}
}

// AskFilename runs an async filename prompt via the editor hook.
func AskFilename(prompt, initial string, onDone func(string, minibuffer.PromptResult)) {
	if PackageHooks.AskFilename != nil {
		PackageHooks.AskFilename(prompt, initial, onDone)
		return
	}
	text, pr := MBReadFilenameString(prompt, initial)
	if onDone != nil {
		onDone(text, pr)
	}
}

// AskChoose runs an async choose menu via the editor hook.
func AskChoose(prompt string, ctx any, labelFn minibuffer.MLChoiceLabelFn, count uint8, defaultIdx uint8, onDone func(int16)) {
	if PackageHooks.AskChoose != nil {
		PackageHooks.AskChoose(prompt, ctx, labelFn, count, defaultIdx, onDone)
		return
	}
	sel := MBChoose(prompt, ctx, labelFn, count, defaultIdx)
	if onDone != nil {
		onDone(sel)
	}
}

package editor

// completion.go — Go-native identifier completion at point

import (
	"bytes"
	"github.com/jdpalmer/jem/app"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"unicode"

	"github.com/jdpalmer/jem/syntax"
)

const completionResultMax = 512

var completionPending string

func completionIsIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func completionPrefixAtPoint(wp *Window) string {
	if wp == nil || wp.Buffer == nil {
		return ""
	}
	lp := wp.Buffer.Line(wp.Cursor.Line)
	if lp == nil || wp.Cursor.Offset == 0 {
		return ""
	}
	start := wp.Cursor.Offset
	for start > 0 && completionIsIdentByte(lp.Data[start-1]) {
		start--
	}
	if start == wp.Cursor.Offset {
		return ""
	}
	return string(lp.Data[start:wp.Cursor.Offset])
}

func bufferTextBytes(bp *Buffer) []byte {
	if bp == nil {
		return nil
	}
	var buf bytes.Buffer
	for lineNum := uint(1); lineNum <= bp.LineCount; lineNum++ {
		if lineNum > 1 {
			buf.WriteByte('\n')
		}
		lp := bp.Line(lineNum)
		if lp != nil && lp.Len() > 0 {
			buf.Write(lp.Data)
		}
	}
	return buf.Bytes()
}

func completionKeywordsForLang(lang LangMode) []string {
	switch lang {
	case LModeC:
		return append(append([]string{}, syntax.CKeywords...), syntax.CTypes...)
	case LModeJava:
		return append(append([]string{}, syntax.JavaKeywords...), syntax.JavaTypes...)
	case LModeGo:
		return append(append([]string{}, syntax.GoKeywords...), syntax.GoTypes...)
	case LModeJavaScript, LModeActionScript:
		return append(append([]string{}, syntax.JSKeywords...), syntax.JSTypes...)
	case LModeTypeScript:
		return append(append([]string{}, syntax.TSKeywords...), syntax.TSTypes...)
	case LModeDart:
		return append(append([]string{}, syntax.DartKeywords...), syntax.DartTypes...)
	case LModePython:
		return append(append([]string{}, syntax.PyKeywords...), syntax.PyTypes...)
	case LModeCSharp:
		return append(append([]string{}, syntax.CSKeywords...), syntax.CSTypes...)
	case LModeRust:
		return append(append([]string{}, syntax.RustKeywords...), syntax.RustTypes...)
	case LModeSwift:
		return append(append([]string{}, syntax.SwiftKeywords...), syntax.SwiftTypes...)
	case LModeKotlin:
		return append(append([]string{}, syntax.KTKeywords...), syntax.KTTypes...)
	case LModeLua:
		return syntax.LuaKeywords
	case LModeLisp:
		return syntax.LispKeywords
	case LModePascal:
		return append(append([]string{}, syntax.PasKeywords...), syntax.PasTypes...)
	case LModeVerilog:
		return append(append([]string{}, syntax.VlgKeywords...), syntax.VlgTypes...)
	case LModeHTML:
		return append(append([]string{}, syntax.HTMLKeywords...), syntax.HTMLAttrs...)
	case LModeCSS:
		return append(append([]string{}, syntax.CSSKeywords...), syntax.CSSTypes...)
	case LModeR:
		return append(append([]string{}, syntax.RKeywords...), syntax.RTypes...)
	default:
		return syntax.CommonKeywords
	}
}

func completionScanLineWords(lp *Line, add func(string)) {
	if lp == nil {
		return
	}
	i := 0
	n := len(lp.Data)
	for i < n {
		if !completionIsIdentByte(lp.Data[i]) {
			i++
			continue
		}
		start := i
		for i < n && completionIsIdentByte(lp.Data[i]) {
			i++
		}
		word := string(lp.Data[start:i])
		if len(word) > 0 && (unicode.IsLetter(rune(word[0])) || word[0] == '_') {
			add(word)
		}
	}
}

func completionGoIdents(bp *Buffer) []string {
	src := bufferTextBytes(bp)
	if len(src) == 0 {
		return nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok || ident.Name == "" || seen[ident.Name] {
			return true
		}
		seen[ident.Name] = true
		out = append(out, ident.Name)
		return true
	})
	return out
}

func completionCollectCandidates(bp *Buffer, prefix string) []string {
	if prefix == "" {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	add := func(word string) {
		if word == "" || word == prefix || seen[word] {
			return
		}
		if strings.HasPrefix(word, prefix) {
			seen[word] = true
			out = append(out, word)
		}
	}
	for _, word := range completionKeywordsForLang(bp.LangMode) {
		add(word)
	}
	for lineNum := uint(1); lineNum <= bp.LineCount; lineNum++ {
		completionScanLineWords(bp.Line(lineNum), add)
	}
	if bp.LangMode == LModeGo {
		for _, word := range completionGoIdents(bp) {
			add(word)
		}
	}
	sort.Strings(out)
	return out
}

func completionSetPending(fullWord, prefix string) {
	suffix := strings.TrimPrefix(fullWord, prefix)
	if len(suffix) > completionResultMax {
		suffix = suffix[:completionResultMax]
	}
	completionPending = suffix
}

func completionPickMatch(candidates []string, prefix string) (string, bool) {
	if len(candidates) == 0 {
		return "", false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	label, pr := mbReadFuzzyListString("Complete: ", commandsProvider, candidates, uint(len(candidates)))
	if pr != PromptResultYes {
		return "", false
	}
	if label == "" {
		return "", false
	}
	if strings.HasPrefix(label, prefix) {
		return label, true
	}
	return "", false
}

// CmdCompletionComplete finds identifier completions at point (Shift-Tab).
func CmdCompletionComplete(f bool, n int) bool {
	_ = f
	_ = n
	completionPending = ""

	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		mbWrite("[no buffer]")
		return false
	}
	if bp.IsReadonly {
		mbWrite("[read-only buffer]")
		return false
	}

	prefix := completionPrefixAtPoint(wp)
	if prefix == "" {
		mbWrite("[completion: no prefix at point]")
		return false
	}

	candidates := completionCollectCandidates(bp, prefix)
	match, ok := completionPickMatch(candidates, prefix)
	if !ok {
		if len(candidates) == 0 {
			mbWrite("[completion: no matches]")
		}
		return false
	}

	completionSetPending(match, prefix)
	if strings.Contains(completionPending, "\n") {
		first := strings.SplitN(completionPending, "\n", 2)[0]
		mbWrite("[completion] %s...  (Shift+Ret to accept)", first)
	} else {
		mbWrite("[completion] %s  (Shift+Ret to accept)", completionPending)
	}
	return true
}

// CmdCompletionAccept inserts the pending completion (Shift-Enter).
func CmdCompletionAccept(f bool, n int) bool {
	_ = f
	_ = n
	if completionPending == "" {
		mbWrite("[completion: no pending suggestion]")
		return false
	}
	wp := app.State.CurrentWindow
	if wp == nil || wp.Buffer == nil || wp.Buffer.IsReadonly {
		return false
	}

	text := completionPending
	completionPending = ""
	for len(text) > 0 {
		nl := strings.IndexByte(text, '\n')
		seg := text
		if nl >= 0 {
			seg = text[:nl]
		}
		if len(seg) > 0 {
			if !windowInsertText(wp, []byte(seg), len(seg)) {
				return false
			}
		}
		if nl < 0 {
			break
		}
		if !windowInsertNewline(wp) {
			return false
		}
		text = text[nl+1:]
	}
	mbClear()
	return true
}

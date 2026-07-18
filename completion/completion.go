// Package completion provides identifier completion at point.
package completion

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"unicode"

	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/edit"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/ui"
)

const resultMax = 512

var pending string

// ClearPending clears any pending completion suggestion.
func ClearPending() {
	pending = ""
}

func isIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// PrefixAtPoint returns the identifier prefix ending at the window cursor.
func PrefixAtPoint(wp *app.Window) string {
	if wp == nil || wp.Buffer == nil {
		return ""
	}
	lp := wp.Buffer.Line(wp.Cursor.Line)
	if lp == nil || wp.Cursor.Offset == 0 {
		return ""
	}
	start := wp.Cursor.Offset
	for start > 0 && isIdentByte(lp.Data[start-1]) {
		start--
	}
	if start == wp.Cursor.Offset {
		return ""
	}
	return string(lp.Data[start:wp.Cursor.Offset])
}

func bufferTextBytes(bp *buffer.Buffer) []byte {
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

func keywordsForLang(lang buffer.LangMode) []string {
	switch lang {
	case buffer.LModeC:
		return append(append([]string{}, syntax.CKeywords...), syntax.CTypes...)
	case buffer.LModeJava:
		return append(append([]string{}, syntax.JavaKeywords...), syntax.JavaTypes...)
	case buffer.LModeGo:
		return append(append([]string{}, syntax.GoKeywords...), syntax.GoTypes...)
	case buffer.LModeJavaScript, buffer.LModeActionScript:
		return append(append([]string{}, syntax.JSKeywords...), syntax.JSTypes...)
	case buffer.LModeTypeScript:
		return append(append([]string{}, syntax.TSKeywords...), syntax.TSTypes...)
	case buffer.LModeDart:
		return append(append([]string{}, syntax.DartKeywords...), syntax.DartTypes...)
	case buffer.LModePython:
		return append(append([]string{}, syntax.PyKeywords...), syntax.PyTypes...)
	case buffer.LModeCSharp:
		return append(append([]string{}, syntax.CSKeywords...), syntax.CSTypes...)
	case buffer.LModeRust:
		return append(append([]string{}, syntax.RustKeywords...), syntax.RustTypes...)
	case buffer.LModeSwift:
		return append(append([]string{}, syntax.SwiftKeywords...), syntax.SwiftTypes...)
	case buffer.LModeKotlin:
		return append(append([]string{}, syntax.KTKeywords...), syntax.KTTypes...)
	case buffer.LModeLua:
		return syntax.LuaKeywords
	case buffer.LModeLisp:
		return syntax.LispKeywords
	case buffer.LModePascal:
		return append(append([]string{}, syntax.PasKeywords...), syntax.PasTypes...)
	case buffer.LModeVerilog:
		return append(append([]string{}, syntax.VlgKeywords...), syntax.VlgTypes...)
	case buffer.LModeHTML:
		return append(append([]string{}, syntax.HTMLKeywords...), syntax.HTMLAttrs...)
	case buffer.LModeCSS:
		return append(append([]string{}, syntax.CSSKeywords...), syntax.CSSTypes...)
	case buffer.LModeR:
		return append(append([]string{}, syntax.RKeywords...), syntax.RTypes...)
	default:
		return syntax.CommonKeywords
	}
}

func scanLineWords(lp *buffer.Line, add func(string)) {
	if lp == nil {
		return
	}
	i := 0
	n := len(lp.Data)
	for i < n {
		if !isIdentByte(lp.Data[i]) {
			i++
			continue
		}
		start := i
		for i < n && isIdentByte(lp.Data[i]) {
			i++
		}
		word := string(lp.Data[start:i])
		if len(word) > 0 && (unicode.IsLetter(rune(word[0])) || word[0] == '_') {
			add(word)
		}
	}
}

func goIdents(bp *buffer.Buffer) []string {
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

// CollectCandidates returns identifier completions for prefix in bp.
func CollectCandidates(bp *buffer.Buffer, prefix string) []string {
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
	for _, word := range keywordsForLang(bp.LangMode) {
		add(word)
	}
	for lineNum := uint(1); lineNum <= bp.LineCount; lineNum++ {
		scanLineWords(bp.Line(lineNum), add)
	}
	if bp.LangMode == buffer.LModeGo {
		for _, word := range goIdents(bp) {
			add(word)
		}
	}
	sort.Strings(out)
	return out
}

func setPending(fullWord, prefix string) {
	suffix := strings.TrimPrefix(fullWord, prefix)
	if len(suffix) > resultMax {
		suffix = suffix[:resultMax]
	}
	pending = suffix
}

func stringListProvider(ctx any, idx uint) []byte {
	names, ok := ctx.([]string)
	if !ok || int(idx) >= len(names) {
		return nil
	}
	return []byte(names[idx])
}

func pickMatch(candidates []string, prefix string) (string, bool) {
	if len(candidates) == 0 {
		return "", false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	label, pr := ui.MBReadFuzzyListString("Complete: ", stringListProvider, candidates, uint(len(candidates)))
	if pr != app.PromptResultYes {
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

// CmdComplete finds identifier completions at point (Shift-Tab).
func CmdComplete(f bool, n int) bool {
	_ = f
	_ = n
	pending = ""

	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
	if wp == nil || bp == nil {
		ui.MBWrite("[no buffer]")
		return false
	}
	if bp.IsReadonly {
		ui.MBWrite("[read-only buffer]")
		return false
	}

	prefix := PrefixAtPoint(wp)
	if prefix == "" {
		ui.MBWrite("[completion: no prefix at point]")
		return false
	}

	candidates := CollectCandidates(bp, prefix)
	match, ok := pickMatch(candidates, prefix)
	if !ok {
		if len(candidates) == 0 {
			ui.MBWrite("[completion: no matches]")
		}
		return false
	}

	setPending(match, prefix)
	if strings.Contains(pending, "\n") {
		first := strings.SplitN(pending, "\n", 2)[0]
		ui.MBWrite("[completion] %s...  (Shift+Ret to accept)", first)
	} else {
		ui.MBWrite("[completion] %s  (Shift+Ret to accept)", pending)
	}
	return true
}

// CmdAccept inserts the pending completion (Shift-Enter).
func CmdAccept(f bool, n int) bool {
	_ = f
	_ = n
	if pending == "" {
		ui.MBWrite("[completion: no pending suggestion]")
		return false
	}
	wp := app.State.CurrentWindow
	if wp == nil || wp.Buffer == nil || wp.Buffer.IsReadonly {
		return false
	}

	text := pending
	pending = ""
	for len(text) > 0 {
		nl := strings.IndexByte(text, '\n')
		seg := text
		if nl >= 0 {
			seg = text[:nl]
		}
		if len(seg) > 0 {
			if !edit.InsertText(wp, []byte(seg)) {
				return false
			}
		}
		if nl < 0 {
			break
		}
		if !edit.InsertNewline(wp) {
			return false
		}
		text = text[nl+1:]
	}
	ui.MBClear()
	return true
}

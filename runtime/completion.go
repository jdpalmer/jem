package runtime

import (
	"bytes"
	"github.com/jdpalmer/jem/minibuffer"
	"github.com/jdpalmer/jem/window"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"unicode"

	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/display"
	"github.com/jdpalmer/jem/syntax"
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
func PrefixAtPoint(win *window.Window) string {
	if win == nil || win.Buffer == nil {
		return ""
	}
	line := win.Buffer.Line(win.Cursor.Line)
	if line == nil || win.Cursor.Offset == 0 {
		return ""
	}
	start := win.Cursor.Offset
	for start > 0 && isIdentByte(line.Data[start-1]) {
		start--
	}
	if start == win.Cursor.Offset {
		return ""
	}
	return string(line.Data[start:win.Cursor.Offset])
}

func bufferTextBytes(buf *buffer.Buffer) []byte {
	if buf == nil {
		return nil
	}
	var out bytes.Buffer
	for lineNum := uint(1); lineNum <= buf.LineCount; lineNum++ {
		if lineNum > 1 {
			out.WriteByte('\n')
		}
		line := buf.Line(lineNum)
		if line != nil && line.Len() > 0 {
			out.Write(line.Data)
		}
	}
	return out.Bytes()
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

func scanLineWords(line *buffer.Line, add func(string)) {
	if line == nil {
		return
	}
	i := 0
	n := len(line.Data)
	for i < n {
		if !isIdentByte(line.Data[i]) {
			i++
			continue
		}
		start := i
		for i < n && isIdentByte(line.Data[i]) {
			i++
		}
		word := string(line.Data[start:i])
		if len(word) > 0 && (unicode.IsLetter(rune(word[0])) || word[0] == '_') {
			add(word)
		}
	}
}

func goIdents(buf *buffer.Buffer) []string {
	src := bufferTextBytes(buf)
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

// CollectCandidates returns identifier completions for prefix in buf.
func CollectCandidates(buf *buffer.Buffer, prefix string) []string {
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
	for _, word := range keywordsForLang(buf.LangMode) {
		add(word)
	}
	for lineNum := uint(1); lineNum <= buf.LineCount; lineNum++ {
		scanLineWords(buf.Line(lineNum), add)
	}
	if buf.LangMode == buffer.LModeGo {
		for _, word := range goIdents(buf) {
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

func pickMatch(candidates []string, prefix string, onDone func(match string, ok bool)) {
	if len(candidates) == 0 {
		onDone("", false)
		return
	}
	if len(candidates) == 1 {
		onDone(candidates[0], true)
		return
	}
	AskFuzzy("Complete: ", stringListProvider, candidates, uint(len(candidates)), func(label string, pr minibuffer.PromptResult) {
		if pr != minibuffer.PromptResultYes || label == "" {
			onDone("", false)
			return
		}
		if strings.HasPrefix(label, prefix) {
			onDone(label, true)
			return
		}
		onDone("", false)
	})
}

// CmdComplete finds identifier completions at point (Shift-Tab).
func CmdComplete(f bool, n int) bool {
	_ = f
	_ = n
	pending = ""

	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil {
		display.MBWrite("[no buffer]")
		return false
	}
	if buf.IsReadonly {
		display.MBWrite("[read-only buffer]")
		return false
	}

	prefix := PrefixAtPoint(win)
	if prefix == "" {
		display.MBWrite("[completion: no prefix at point]")
		return false
	}

	candidates := CollectCandidates(buf, prefix)
	if len(candidates) == 0 {
		display.MBWrite("[completion: no matches]")
		return false
	}
	pickMatch(candidates, prefix, func(match string, ok bool) {
		if !ok {
			return
		}
		setPending(match, prefix)
		if strings.Contains(pending, "\n") {
			first := strings.SplitN(pending, "\n", 2)[0]
			display.MBWrite("[completion] %s...  (Shift+Ret to accept)", first)
		} else {
			display.MBWrite("[completion] %s  (Shift+Ret to accept)", pending)
		}
	})
	return true
}

// CmdAccept inserts the pending completion (Shift-Enter).
func CmdAccept(f bool, n int) bool {
	_ = f
	_ = n
	if pending == "" {
		display.MBWrite("[completion: no pending suggestion]")
		return false
	}
	win := window.Active.CurrentWindow
	if win == nil || win.Buffer == nil || win.Buffer.IsReadonly {
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
			if err := window.InsertText(win, []byte(seg)); err != nil {
				return false
			}
		}
		if nl < 0 {
			break
		}
		if err := window.InsertNewline(win); err != nil {
			return false
		}
		text = text[nl+1:]
	}
	display.MBClear()
	return true
}

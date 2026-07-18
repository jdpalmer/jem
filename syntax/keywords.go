package syntax

import "github.com/jdpalmer/jem/buffer"

// Keyword tables for syntax highlighting.

var (
	keywordStyle = buffer.MakeTextStyle(buffer.TermColorBlue, buffer.TermColorDefault, buffer.TextStyleBold)
	typeStyle    = buffer.MakeTextStyle(buffer.TermColorMagenta, buffer.TermColorDefault, 0)
)

type identWordMaps struct {
	keywords map[string]bool
	types    map[string]bool
}

var identWordsByLang map[buffer.LangMode]identWordMaps

func mkMap(sl []string) map[string]bool {
	m := make(map[string]bool, len(sl))
	for _, s := range sl {
		m[s] = true
	}
	return m
}

func ident_color_for_lang(lang buffer.LangMode, ident string) buffer.TextStyle {
	tables, ok := identWordsByLang[lang]
	if !ok {
		return buffer.TextStyleDefault
	}
	if tables.keywords[ident] {
		return keywordStyle
	}
	if tables.types != nil && tables.types[ident] {
		return typeStyle
	}
	return buffer.TextStyleDefault
}

func initIdentWordsByLang() {
	identWordsByLang = map[buffer.LangMode]identWordMaps{
		buffer.LModeC:            {cKeywordsMap, cTypesMap},
		buffer.LModeJava:         {javaKeywordsMap, javaTypesMap},
		buffer.LModeCSharp:       {csKeywordsMap, csTypesMap},
		buffer.LModeGo:           {goKeywordsMap, goTypesMap},
		buffer.LModeJavaScript:   {jsKeywordsMap, jsTypesMap},
		buffer.LModeTypeScript:   {tsKeywordsMap, tsTypesMap},
		buffer.LModeDart:         {dartKeywordsMap, dartTypesMap},
		buffer.LModePython:       {pyKeywordsMap, pyTypesMap},
		buffer.LModeLua:          {luaKeywordsMap, nil},
		buffer.LModeLisp:         {lispKeywordsMap, nil},
		buffer.LModePascal:       {pasKeywordsMap, pasTypesMap},
		buffer.LModeVerilog:      {vlgKeywordsMap, vlgTypesMap},
		buffer.LModeSwift:        {swiftKeywordsMap, swiftTypesMap},
		buffer.LModeActionScript: {asKeywordsMap, asTypesMap},
		buffer.LModeRust:         {rustKeywordsMap, rustTypesMap},
		buffer.LModeR:            {rKeywordsMap, rTypesMap},
		buffer.LModeKotlin:       {ktKeywordsMap, ktTypesMap},
		buffer.LModeHTML:         {htmlKeywordsMap, htmlAttrsMap},
		buffer.LModeCSS:          {cssKeywordsMap, cssTypesMap},
	}
}

var commonKeywords = []string{"if", "for", "while", "return", "func", "package", "import", "class", "def", "else", "switch"}

var cKeywords = []string{"break", "case", "catch", "continue", "default", "delete", "do", "else", "for", "goto", "if", "new", "return", "sizeof", "switch", "throw", "try", "while"}
var cTypes = []string{"auto", "bool", "char", "class", "const", "constexpr", "double", "enum", "explicit", "extern", "false", "float", "friend", "inline", "int", "long", "mutable", "namespace", "nullptr", "operator", "private", "protected", "public", "register", "short", "signed", "static", "struct", "template", "this", "true", "typedef", "typename", "union", "unsigned", "using", "virtual", "void", "volatile"}

var javaKeywords = []string{"assert", "break", "case", "catch", "continue", "default", "do", "else", "finally", "for", "goto", "if", "instanceof", "new", "return", "switch", "throw", "try", "while"}
var javaTypes = []string{"abstract", "boolean", "byte", "char", "class", "const", "double", "enum", "extends", "false", "final", "float", "implements", "import", "int", "interface", "long", "native", "null", "package", "private", "protected", "public", "short", "static", "strictfp", "super", "synchronized", "this", "throws", "transient", "true", "void", "volatile"}

var swiftKeywords = []string{"associatedtype", "break", "case", "catch", "continue", "default", "defer", "do", "else", "fallthrough", "for", "guard", "if", "in", "repeat", "return", "switch", "throw", "throws", "try", "where", "while"}
var swiftTypes = []string{"Any", "AnyObject", "Bool", "Character", "Double", "Error", "Float", "Int", "Never", "Self", "String", "UInt", "actor", "as", "async", "await", "class", "enum", "extension", "false", "func", "import", "init", "inout", "internal", "let", "nil", "operator", "private", "protocol", "public", "self", "some", "static", "struct", "subscript", "super", "true", "typealias", "var"}

var jsKeywords = []string{"await", "break", "case", "catch", "class", "const", "continue", "debugger", "default", "delete", "do", "else", "export", "extends", "finally", "for", "function", "if", "import", "in", "instanceof", "new", "return", "switch", "throw", "try", "typeof", "var", "void", "while", "with", "yield"}
var jsTypes = []string{"Array", "Boolean", "Date", "Error", "Function", "Math", "Number", "Object", "Promise", "RegExp", "String", "console", "false", "null", "true", "undefined", "window"}

var asKeywords = []string{"break", "case", "catch", "class", "const", "continue", "default", "delete", "do", "dynamic", "else", "extends", "final", "finally", "for", "function", "get", "if", "implements", "import", "in", "instanceof", "interface", "internal", "new", "override", "package", "private", "protected", "public", "return", "set", "static", "super", "switch", "throw", "try", "use", "var", "while", "with"}
var asTypes = []string{"Array", "Boolean", "Class", "Date", "Error", "Function", "Infinity", "NaN", "Number", "Object", "String", "false", "null", "this", "true", "undefined", "void"}

var tsKeywords = []string{"abstract", "as", "asserts", "async", "await", "break", "case", "catch", "class", "const", "continue", "debugger", "declare", "default", "delete", "do", "else", "enum", "export", "extends", "finally", "for", "from", "function", "get", "if", "implements", "import", "in", "infer", "instanceof", "interface", "is", "keyof", "module", "namespace", "new", "readonly", "return", "satisfies", "set", "static", "super", "switch", "throw", "try", "type", "typeof", "var", "void", "while", "with", "yield"}
var tsTypes = []string{"any", "bigint", "boolean", "false", "never", "null", "number", "object", "string", "symbol", "true", "undefined", "unknown"}

var dartKeywords = []string{"abstract", "as", "assert", "async", "await", "break", "case", "catch", "class", "const", "continue", "default", "deferred", "do", "else", "enum", "export", "extends", "extension", "external", "factory", "false", "final", "finally", "for", "function", "get", "hide", "if", "implements", "import", "in", "interface", "is", "late", "library", "mixin", "new", "null", "on", "operator", "part", "required", "rethrow", "return", "set", "show", "static", "super", "switch", "sync", "this", "throw", "true", "try", "typedef", "var", "void", "while", "with", "yield"}
var dartTypes = []string{"Future", "List", "Map", "Never", "Object", "Set", "Stream", "String", "bool", "double", "dynamic", "int", "num"}

var goKeywords = []string{"break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select", "struct", "switch", "type", "var"}
var goTypes = []string{"any", "bool", "byte", "complex128", "complex64", "error", "false", "float32", "float64", "int", "int16", "int32", "int64", "int8", "nil", "rune", "string", "true", "uint", "uint16", "uint32", "uint64", "uint8", "uintptr"}

var csKeywords = []string{"abstract", "as", "async", "await", "base", "break", "case", "catch", "checked", "class", "const", "continue", "default", "delegate", "do", "else", "enum", "event", "explicit", "extern", "finally", "fixed", "for", "foreach", "goto", "if", "implicit", "in", "interface", "internal", "is", "lock", "namespace", "new", "operator", "out", "override", "params", "private", "protected", "public", "readonly", "ref", "return", "sealed", "sizeof", "stackalloc", "static", "struct", "switch", "throw", "try", "typeof", "unchecked", "unsafe", "using", "virtual", "void", "volatile", "while"}
var csTypes = []string{"bool", "byte", "char", "decimal", "double", "dynamic", "false", "float", "int", "long", "null", "object", "sbyte", "short", "string", "this", "true", "uint", "ulong", "ushort", "var"}

var rustKeywords = []string{"as", "async", "await", "break", "const", "continue", "crate", "dyn", "else", "enum", "extern", "false", "fn", "for", "if", "impl", "in", "let", "loop", "match", "mod", "move", "mut", "pub", "ref", "return", "self", "static", "struct", "super", "trait", "true", "type", "unsafe", "use", "where", "while"}
var rustTypes = []string{"Option", "Result", "Self", "String", "Vec", "bool", "char", "f32", "f64", "i128", "i16", "i32", "i64", "i8", "isize", "str", "u128", "u16", "u32", "u64", "u8", "usize"}

var rKeywords = []string{"break", "else", "for", "function", "if", "in", "next", "repeat", "return", "while"}
var rTypes = []string{"FALSE", "Inf", "NA", "NULL", "NaN", "TRUE", "library", "require", "source"}

var ktKeywords = []string{"abstract", "actual", "annotation", "as", "break", "by", "catch", "class", "companion", "const", "constructor", "continue", "crossinline", "data", "do", "else", "enum", "expect", "external", "false", "final", "finally", "for", "fun", "if", "import", "in", "infix", "init", "inline", "inner", "interface", "internal", "is", "lateinit", "noinline", "null", "object", "open", "operator", "out", "override", "package", "private", "protected", "public", "reified", "return", "sealed", "suspend", "super", "this", "throw", "true", "try", "typealias", "val", "var", "when", "while"}
var ktTypes = []string{"Any", "Boolean", "Byte", "Char", "Double", "Float", "Int", "Long", "Nothing", "Short", "String", "UInt", "ULong", "UShort", "Unit"}

var htmlKeywords = []string{"a", "article", "aside", "body", "button", "div", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "head", "header", "html", "img", "input", "label", "li", "link", "main", "meta", "nav", "ol", "option", "p", "path", "script", "section", "select", "span", "style", "svg", "table", "tbody", "td", "textarea", "th", "thead", "title", "tr", "ul", "xml"}
var htmlAttrs = []string{"alt", "charset", "class", "content", "data", "disabled", "fill", "height", "href", "id", "lang", "name", "placeholder", "rel", "role", "src", "style", "type", "value", "viewBox", "width", "xmlns"}

var cssKeywords = []string{"font-face", "import", "keyframes", "media", "namespace", "page", "supports"}
var cssTypes = []string{"align-items", "background", "background-color", "border", "border-radius", "color", "display", "flex", "flex-direction", "font-family", "font-size", "font-weight", "gap", "grid", "height", "justify-content", "line-height", "margin", "margin-bottom", "margin-left", "margin-right", "margin-top", "max-width", "min-height", "min-width", "padding", "padding-bottom", "padding-left", "padding-right", "padding-top", "position", "text-align", "transform", "transition", "width", "z-index"}

var pyKeywords = []string{"and", "as", "assert", "async", "await", "break", "class", "continue", "def", "del", "elif", "else", "except", "finally", "for", "from", "global", "if", "import", "in", "is", "lambda", "nonlocal", "not", "or", "pass", "raise", "return", "try", "while", "with", "yield"}
var pyTypes = []string{"False", "None", "True"}

var luaKeywords = []string{"and", "break", "do", "else", "elseif", "end", "false", "for", "function", "goto", "if", "in", "local", "nil", "not", "or", "repeat", "return", "then", "true", "until", "while"}
var lispKeywords = []string{"and", "apply", "begin", "call/cc", "car", "case", "cdr", "cond", "cons", "define", "define-syntax", "do", "else", "eq?", "equal?", "if", "lambda", "let", "let*", "letrec", "list", "map", "not", "null?", "or", "pair?", "quasiquote", "cmd_quote", "set!", "string?", "symbol?", "unless", "unquote", "values", "when", "while"}

var pasKeywords = []string{"and", "array", "begin", "case", "const", "div", "do", "downto", "else", "end", "file", "for", "forward", "function", "goto", "if", "implementation", "in", "interface", "label", "mod", "nil", "not", "object", "of", "or", "packed", "procedure", "program", "record", "repeat", "set", "string", "then", "to", "type", "unit", "until", "uses", "var", "while", "with"}
var pasTypes = []string{"boolean", "byte", "char", "extended", "integer", "longint", "real", "shortint", "single", "smallint", "word"}

var vlgKeywords = []string{"always", "assign", "automatic", "begin", "case", "casex", "casez", "default", "defparam", "disable", "else", "end", "endcase", "endfunction", "endmodule", "endspecify", "endtask", "event", "for", "force", "forever", "fork", "function", "if", "ifnone", "initial", "join", "localparam", "module", "negedge", "output", "parameter", "posedge", "primitive", "release", "repeat", "specify", "task", "while"}
var vlgTypes = []string{"inout", "input", "integer", "real", "realtime", "reg", "signed", "time", "tri", "tri0", "tri1", "triand", "trior", "trireg", "unsigned", "wand", "wire", "wor"}

// Map-backed lookups for fast ident classification
var cKeywordsMap = mkMap(cKeywords)
var cTypesMap = mkMap(cTypes)
var javaKeywordsMap = mkMap(javaKeywords)
var javaTypesMap = mkMap(javaTypes)
var swiftKeywordsMap = mkMap(swiftKeywords)
var swiftTypesMap = mkMap(swiftTypes)
var jsKeywordsMap = mkMap(jsKeywords)
var jsTypesMap = mkMap(jsTypes)
var asKeywordsMap = mkMap(asKeywords)
var asTypesMap = mkMap(asTypes)
var tsKeywordsMap = mkMap(tsKeywords)
var tsTypesMap = mkMap(tsTypes)
var dartKeywordsMap = mkMap(dartKeywords)
var dartTypesMap = mkMap(dartTypes)
var goKeywordsMap = mkMap(goKeywords)
var goTypesMap = mkMap(goTypes)
var csKeywordsMap = mkMap(csKeywords)
var csTypesMap = mkMap(csTypes)
var rustKeywordsMap = mkMap(rustKeywords)
var rustTypesMap = mkMap(rustTypes)
var rKeywordsMap = mkMap(rKeywords)
var rTypesMap = mkMap(rTypes)
var ktKeywordsMap = mkMap(ktKeywords)
var ktTypesMap = mkMap(ktTypes)
var htmlKeywordsMap = mkMap(htmlKeywords)
var htmlAttrsMap = mkMap(htmlAttrs)
var cssKeywordsMap = mkMap(cssKeywords)
var cssTypesMap = mkMap(cssTypes)
var pyKeywordsMap = mkMap(pyKeywords)
var pyTypesMap = mkMap(pyTypes)
var luaKeywordsMap = mkMap(luaKeywords)
var lispKeywordsMap = mkMap(lispKeywords)
var pasKeywordsMap = mkMap(pasKeywords)
var pasTypesMap = mkMap(pasTypes)
var vlgKeywordsMap = mkMap(vlgKeywords)
var vlgTypesMap = mkMap(vlgTypes)

func init() {
	initIdentWordsByLang()
	initOperatorsByLang()
}

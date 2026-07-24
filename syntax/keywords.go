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

func identColorForLang(lang buffer.LangMode, ident string) buffer.TextStyle {
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

var CommonKeywords = []string{"if", "for", "while", "return", "func", "package", "import", "class", "def", "else", "switch"}

var CKeywords = []string{"break", "case", "catch", "continue", "default", "delete", "do", "else", "for", "goto", "if", "new", "return", "sizeof", "switch", "throw", "try", "while"}
var CTypes = []string{"auto", "bool", "char", "class", "const", "constexpr", "double", "enum", "explicit", "extern", "false", "float", "friend", "inline", "int", "long", "mutable", "namespace", "nullptr", "operator", "private", "protected", "public", "register", "short", "signed", "static", "struct", "template", "this", "true", "typedef", "typename", "union", "unsigned", "using", "virtual", "void", "volatile"}

var JavaKeywords = []string{"assert", "break", "case", "catch", "continue", "default", "do", "else", "finally", "for", "goto", "if", "instanceof", "new", "return", "switch", "throw", "try", "while"}
var JavaTypes = []string{"abstract", "boolean", "byte", "char", "class", "const", "double", "enum", "extends", "false", "final", "float", "implements", "import", "int", "interface", "long", "native", "null", "package", "private", "protected", "public", "short", "static", "strictfp", "super", "synchronized", "this", "throws", "transient", "true", "void", "volatile"}

var SwiftKeywords = []string{"associatedtype", "break", "case", "catch", "continue", "default", "defer", "do", "else", "fallthrough", "for", "guard", "if", "in", "repeat", "return", "switch", "throw", "throws", "try", "where", "while"}
var SwiftTypes = []string{"Any", "AnyObject", "Bool", "Character", "Double", "Error", "Float", "Int", "Never", "Self", "String", "UInt", "actor", "as", "async", "await", "class", "enum", "extension", "false", "func", "import", "init", "inout", "internal", "let", "nil", "operator", "private", "protocol", "public", "self", "some", "static", "struct", "subscript", "super", "true", "typealias", "var"}

var JSKeywords = []string{"await", "break", "case", "catch", "class", "const", "continue", "debugger", "default", "delete", "do", "else", "export", "extends", "finally", "for", "function", "if", "import", "in", "instanceof", "new", "return", "switch", "throw", "try", "typeof", "var", "void", "while", "with", "yield"}
var JSTypes = []string{"Array", "Boolean", "Date", "Error", "Function", "Math", "Number", "Object", "Promise", "RegExp", "String", "console", "false", "null", "true", "undefined", "window"}

var ASKeywords = []string{"break", "case", "catch", "class", "const", "continue", "default", "delete", "do", "dynamic", "else", "extends", "final", "finally", "for", "function", "get", "if", "implements", "import", "in", "instanceof", "interface", "internal", "new", "override", "package", "private", "protected", "public", "return", "set", "static", "super", "switch", "throw", "try", "use", "var", "while", "with"}
var ASTypes = []string{"Array", "Boolean", "Class", "Date", "Error", "Function", "Infinity", "NaN", "Number", "Object", "String", "false", "null", "this", "true", "undefined", "void"}

var TSKeywords = []string{"abstract", "as", "asserts", "async", "await", "break", "case", "catch", "class", "const", "continue", "debugger", "declare", "default", "delete", "do", "else", "enum", "export", "extends", "finally", "for", "from", "function", "get", "if", "implements", "import", "in", "infer", "instanceof", "interface", "is", "keyof", "module", "namespace", "new", "readonly", "return", "satisfies", "set", "static", "super", "switch", "throw", "try", "type", "typeof", "var", "void", "while", "with", "yield"}
var TSTypes = []string{"any", "bigint", "boolean", "false", "never", "null", "number", "object", "string", "symbol", "true", "undefined", "unknown"}

var DartKeywords = []string{"abstract", "as", "assert", "async", "await", "break", "case", "catch", "class", "const", "continue", "default", "deferred", "do", "else", "enum", "export", "extends", "extension", "external", "factory", "false", "final", "finally", "for", "function", "get", "hide", "if", "implements", "import", "in", "interface", "is", "late", "library", "mixin", "new", "null", "on", "operator", "part", "required", "rethrow", "return", "set", "show", "static", "super", "switch", "sync", "this", "throw", "true", "try", "typedef", "var", "void", "while", "with", "yield"}
var DartTypes = []string{"Future", "List", "Map", "Never", "Object", "Set", "Stream", "String", "bool", "double", "dynamic", "int", "num"}

var GoKeywords = []string{"break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select", "struct", "switch", "type", "var"}
var GoTypes = []string{"any", "bool", "byte", "complex128", "complex64", "error", "false", "float32", "float64", "int", "int16", "int32", "int64", "int8", "nil", "rune", "string", "true", "uint", "uint16", "uint32", "uint64", "uint8", "uintptr"}

var CSKeywords = []string{"abstract", "as", "async", "await", "base", "break", "case", "catch", "checked", "class", "const", "continue", "default", "delegate", "do", "else", "enum", "event", "explicit", "extern", "finally", "fixed", "for", "foreach", "goto", "if", "implicit", "in", "interface", "internal", "is", "lock", "namespace", "new", "operator", "out", "override", "params", "private", "protected", "public", "readonly", "ref", "return", "sealed", "sizeof", "stackalloc", "static", "struct", "switch", "throw", "try", "typeof", "unchecked", "unsafe", "using", "virtual", "void", "volatile", "while"}
var CSTypes = []string{"bool", "byte", "char", "decimal", "double", "dynamic", "false", "float", "int", "long", "null", "object", "sbyte", "short", "string", "this", "true", "uint", "ulong", "ushort", "var"}

var RustKeywords = []string{"as", "async", "await", "break", "const", "continue", "crate", "dyn", "else", "enum", "extern", "false", "fn", "for", "if", "impl", "in", "let", "loop", "match", "mod", "move", "mut", "pub", "ref", "return", "self", "static", "struct", "super", "trait", "true", "type", "unsafe", "use", "where", "while"}
var RustTypes = []string{"Option", "Result", "Self", "String", "Vec", "bool", "char", "f32", "f64", "i128", "i16", "i32", "i64", "i8", "isize", "str", "u128", "u16", "u32", "u64", "u8", "usize"}

var RKeywords = []string{"break", "else", "for", "function", "if", "in", "next", "repeat", "return", "while"}
var RTypes = []string{"FALSE", "Inf", "NA", "NULL", "NaN", "TRUE", "library", "require", "source"}

var KTKeywords = []string{"abstract", "actual", "annotation", "as", "break", "by", "catch", "class", "companion", "const", "constructor", "continue", "crossinline", "data", "do", "else", "enum", "expect", "external", "false", "final", "finally", "for", "fun", "if", "import", "in", "infix", "init", "inline", "inner", "interface", "internal", "is", "lateinit", "noinline", "null", "object", "open", "operator", "out", "override", "package", "private", "protected", "public", "reified", "return", "sealed", "suspend", "super", "this", "throw", "true", "try", "typealias", "val", "var", "when", "while"}
var KTTypes = []string{"Any", "Boolean", "Byte", "Char", "Double", "Float", "Int", "Long", "Nothing", "Short", "String", "UInt", "ULong", "UShort", "Unit"}

var HTMLKeywords = []string{"a", "article", "aside", "body", "button", "div", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "head", "header", "html", "img", "input", "label", "li", "link", "main", "meta", "nav", "ol", "option", "p", "path", "script", "section", "select", "span", "style", "svg", "table", "tbody", "td", "textarea", "th", "thead", "title", "tr", "ul", "xml"}
var HTMLAttrs = []string{"alt", "charset", "class", "content", "data", "disabled", "fill", "height", "href", "id", "lang", "name", "placeholder", "rel", "role", "src", "style", "type", "value", "viewBox", "width", "xmlns"}

var CSSKeywords = []string{"font-face", "import", "keyframes", "media", "namespace", "page", "supports"}
var CSSTypes = []string{"align-items", "background", "background-color", "border", "border-radius", "color", "display", "flex", "flex-direction", "font-family", "font-size", "font-weight", "gap", "grid", "height", "justify-content", "line-height", "margin", "margin-bottom", "margin-left", "margin-right", "margin-top", "max-width", "min-height", "min-width", "padding", "padding-bottom", "padding-left", "padding-right", "padding-top", "position", "text-align", "transform", "transition", "width", "z-index"}

var PyKeywords = []string{"and", "as", "assert", "async", "await", "break", "class", "continue", "def", "del", "elif", "else", "except", "finally", "for", "from", "global", "if", "import", "in", "is", "lambda", "nonlocal", "not", "or", "pass", "raise", "return", "try", "while", "with", "yield"}
var PyTypes = []string{"False", "None", "True"}

var LuaKeywords = []string{"and", "break", "do", "else", "elseif", "end", "false", "for", "function", "goto", "if", "in", "local", "nil", "not", "or", "repeat", "return", "then", "true", "until", "while"}
var LispKeywords = []string{"and", "apply", "begin", "call/cc", "car", "case", "cdr", "cond", "cons", "define", "define-syntax", "do", "else", "eq?", "equal?", "if", "lambda", "let", "let*", "letrec", "list", "map", "not", "null?", "or", "pair?", "quasiquote", "cmd_quote", "set!", "string?", "symbol?", "unless", "unquote", "values", "when", "while"}

var PasKeywords = []string{"and", "array", "begin", "case", "const", "div", "do", "downto", "else", "end", "file", "for", "forward", "function", "goto", "if", "implementation", "in", "interface", "label", "mod", "nil", "not", "object", "of", "or", "packed", "procedure", "program", "record", "repeat", "set", "string", "then", "to", "type", "unit", "until", "uses", "var", "while", "with"}
var PasTypes = []string{"boolean", "byte", "char", "extended", "integer", "longint", "real", "shortint", "single", "smallint", "word"}

var VlgKeywords = []string{"always", "assign", "automatic", "begin", "case", "casex", "casez", "default", "defparam", "disable", "else", "end", "endcase", "endfunction", "endmodule", "endspecify", "endtask", "event", "for", "force", "forever", "fork", "function", "if", "ifnone", "initial", "join", "localparam", "module", "negedge", "output", "parameter", "posedge", "primitive", "release", "repeat", "specify", "task", "while"}
var VlgTypes = []string{"inout", "input", "integer", "real", "realtime", "reg", "signed", "time", "tri", "tri0", "tri1", "triand", "trior", "trireg", "unsigned", "wand", "wire", "wor"}

// Map-backed lookups for fast ident classification
var cKeywordsMap = mkMap(CKeywords)
var cTypesMap = mkMap(CTypes)
var javaKeywordsMap = mkMap(JavaKeywords)
var javaTypesMap = mkMap(JavaTypes)
var swiftKeywordsMap = mkMap(SwiftKeywords)
var swiftTypesMap = mkMap(SwiftTypes)
var jsKeywordsMap = mkMap(JSKeywords)
var jsTypesMap = mkMap(JSTypes)
var asKeywordsMap = mkMap(ASKeywords)
var asTypesMap = mkMap(ASTypes)
var tsKeywordsMap = mkMap(TSKeywords)
var tsTypesMap = mkMap(TSTypes)
var dartKeywordsMap = mkMap(DartKeywords)
var dartTypesMap = mkMap(DartTypes)
var goKeywordsMap = mkMap(GoKeywords)
var goTypesMap = mkMap(GoTypes)
var csKeywordsMap = mkMap(CSKeywords)
var csTypesMap = mkMap(CSTypes)
var rustKeywordsMap = mkMap(RustKeywords)
var rustTypesMap = mkMap(RustTypes)
var rKeywordsMap = mkMap(RKeywords)
var rTypesMap = mkMap(RTypes)
var ktKeywordsMap = mkMap(KTKeywords)
var ktTypesMap = mkMap(KTTypes)
var htmlKeywordsMap = mkMap(HTMLKeywords)
var htmlAttrsMap = mkMap(HTMLAttrs)
var cssKeywordsMap = mkMap(CSSKeywords)
var cssTypesMap = mkMap(CSSTypes)
var pyKeywordsMap = mkMap(PyKeywords)
var pyTypesMap = mkMap(PyTypes)
var luaKeywordsMap = mkMap(LuaKeywords)
var lispKeywordsMap = mkMap(LispKeywords)
var pasKeywordsMap = mkMap(PasKeywords)
var pasTypesMap = mkMap(PasTypes)
var vlgKeywordsMap = mkMap(VlgKeywords)
var vlgTypesMap = mkMap(VlgTypes)

func init() {
	initIdentWordsByLang()
	initOperatorsByLang()
}

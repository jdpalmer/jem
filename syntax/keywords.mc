// Keyword / type word tables for C, Go, and Python (linear lookup; no maps).
#include <stddef.h>
#include <stdint.h>
#include <string.h>
#include <stdbool.h>

static const char *c_keywords[] = {
	"break", "case", "catch", "continue", "default", "delete", "do", "else",
	"for", "goto", "if", "new", "return", "sizeof", "switch", "throw", "try",
	"while",
	NULL,
};

static const char *c_types[] = {
	"auto", "bool", "char", "class", "const", "constexpr", "double", "enum",
	"explicit", "extern", "false", "float", "friend", "inline", "int", "long",
	"mutable", "namespace", "nullptr", "operator", "private", "protected",
	"public", "register", "short", "signed", "static", "struct", "template",
	"this", "true", "typedef", "typename", "union", "unsigned", "using",
	"virtual", "void", "volatile",
	NULL,
};

static const char *go_keywords[] = {
	"break", "case", "chan", "const", "continue", "default", "defer", "else",
	"fallthrough", "for", "func", "go", "goto", "if", "import", "interface",
	"map", "package", "range", "return", "select", "struct", "switch", "type",
	"var",
	NULL,
};

static const char *go_types[] = {
	"any", "bool", "byte", "complex128", "complex64", "error", "false",
	"float32", "float64", "int", "int16", "int32", "int64", "int8", "nil",
	"rune", "string", "true", "uint", "uint16", "uint32", "uint64", "uint8",
	"uintptr",
	NULL,
};

static const char *py_keywords[] = {
	"and", "as", "assert", "async", "await", "break", "class", "continue",
	"def", "del", "elif", "else", "except", "finally", "for", "from", "global",
	"if", "import", "in", "is", "lambda", "nonlocal", "not", "or", "pass",
	"raise", "return", "try", "while", "with", "yield",
	NULL,
};

static const char *py_types[] = {
	"False", "None", "True",
	NULL,
};

static bool word_in_list(const char[..] word, const char **list) {
	size_t i = 0;
	size_t n = len(word);
	if (list == NULL || n == 0) {
		return false;
	}
	for (i = 0; list[i] != NULL; i++) {
		const char *z = list[i];
		size_t zn = strlen(z);
		if (zn == n && memcmp(ptr(word), z, n) == 0) {
			return true;
		}
	}
	return false;
}

// ident_style_for returns keyword/type style, or TextStyleDefault if plain.
TextStyle ident_style_for(LangMode lang, const char[..] ident) {
	const char **keys = NULL;
	const char **types = NULL;
	switch (lang) {
	case LModeC:
	case LModeModC:
		keys = c_keywords;
		types = c_types;
		break;
	case LModeGo:
		keys = go_keywords;
		types = go_types;
		break;
	case LModePython:
		keys = py_keywords;
		types = py_types;
		break;
	default:
		return TextStyleDefault;
	}
	if (word_in_list(ident, keys)) {
		return text_style_make(TermColorBlue, TermColorDefault, (uint16_t)TextStyleBold);
	}
	if (word_in_list(ident, types)) {
		return text_style_make(TermColorMagenta, TermColorDefault, 0);
	}
	return TextStyleDefault;
}

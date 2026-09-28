// mode — language detection, indent defaults, and mode-aware indent commands.
import "syntax";
import "buffer";
import "window";

#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

static char ascii_lower(char c) {
	if (c >= 'A' && c <= 'Z') {
		return (char)(c + ('a' - 'A'));
	}
	return c;
}

static bool path_ends_with_ci(const char[..] path, const char[..] ext) {
	size_t plen = len(path);
	size_t elen = len(ext);
	size_t i = 0;
	size_t off = 0;
	if (plen < elen) {
		return false;
	}
	off = plen - elen;
	{
		const char *pp = ptr(path);
		const char *ep = ptr(ext);
		for (i = 0; i < elen; i++) {
			if (ascii_lower(pp[off + i]) != ep[i]) {
				return false;
			}
		}
	}
	return true;
}

// mode_detect infers LangMode from a filename or path (extension only).
LangMode mode_detect(const char[..] path) {
	if (path_ends_with_ci(path, ".go")) {
		return LModeGo;
	}
	if (path_ends_with_ci(path, ".py")) {
		return LModePython;
	}
	if (path_ends_with_ci(path, ".mc")) {
		return LModeModC;
	}
	if (path_ends_with_ci(path, ".c") || path_ends_with_ci(path, ".h")) {
		return LModeC;
	}
	if (path_ends_with_ci(path, ".md") || path_ends_with_ci(path, ".markdown")) {
		return LModeMarkdown;
	}
	return LModeNone;
}

// mode_indent_default returns width (and related) defaults for lang.
IndentConfig mode_indent_default(LangMode lang) {
	IndentConfig cfg = {0};
	switch (lang) {
	case LModeGo:
	case LModeModC:
		/* One indent step = one tab (8 display columns), matching gofmt / %C. */
		cfg.width = 8;
		break;
	case LModePython:
		cfg.width = 4;
		cfg.continued = 4;
		break;
	default:
		cfg.width = 2;
		break;
	}
	return cfg;
}

const char *mode_display_name(LangMode lang) {
	switch (lang) {
	case LModeC:
		return "C";
	case LModeModC:
		return "%C";
	case LModeJava:
		return "Java";
	case LModePython:
		return "Python";
	case LModeLua:
		return "Lua";
	case LModeLisp:
		return "Lisp";
	case LModeMarkdown:
		return "Markdown";
	case LModePascal:
		return "Pascal";
	case LModeVerilog:
		return "Verilog";
	case LModeMake:
		return "Make";
	case LModeSwift:
		return "Swift";
	case LModeJavaScript:
		return "JavaScript";
	case LModeActionScript:
		return "ActionScript";
	case LModeTypeScript:
		return "TypeScript";
	case LModeDart:
		return "Dart";
	case LModeGo:
		return "Go";
	case LModeCSharp:
		return "C#";
	case LModeRust:
		return "Rust";
	case LModeR:
		return "R";
	case LModeKotlin:
		return "Kotlin";
	case LModeHTML:
		return "HTML/XML";
	case LModeCSS:
		return "CSS";
	default:
		return "Text";
	}
}

void mode_apply_from_path(Buffer *buf, const char *path) {
	char[..] v = {0};
	size_t plen = 0;
	if (buf == NULL) {
		return;
	}
	plen = path != NULL ? strlen(path) : 0;
	v = ranged(path, plen);
	buf.lang_mode = mode_detect(v);
	buf.indent = mode_indent_default(buf.lang_mode);
}

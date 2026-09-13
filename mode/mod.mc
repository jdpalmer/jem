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

static bool path_ends_with_ci(char[..] path, char[..] ext) {
	size_t plen = len(path);
	size_t elen = len(ext);
	size_t i = 0;
	size_t off = 0;
	if (plen < elen) {
		return false;
	}
	off = plen - elen;
	{
		char *pp = ptr(path);
		char *ep = ptr(ext);
		for (i = 0; i < elen; i++) {
			if (ascii_lower(pp[off + i]) != ep[i]) {
				return false;
			}
		}
	}
	return true;
}

// mode_detect infers LangMode from a filename or path (extension only).
LangMode mode_detect(char[..] path) {
	if (path_ends_with_ci(path, ".go")) {
		return LModeGo;
	}
	if (path_ends_with_ci(path, ".py")) {
		return LModePython;
	}
	if (path_ends_with_ci(path, ".c") || path_ends_with_ci(path, ".h") ||
		path_ends_with_ci(path, ".mc")) {
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

char *mode_display_name(LangMode lang) {
	switch (lang) {
	case LModeC:
		return (char *)(void *)"C";
	case LModeJava:
		return (char *)(void *)"Java";
	case LModePython:
		return (char *)(void *)"Python";
	case LModeLua:
		return (char *)(void *)"Lua";
	case LModeLisp:
		return (char *)(void *)"Lisp";
	case LModeMarkdown:
		return (char *)(void *)"Markdown";
	case LModePascal:
		return (char *)(void *)"Pascal";
	case LModeVerilog:
		return (char *)(void *)"Verilog";
	case LModeMake:
		return (char *)(void *)"Make";
	case LModeSwift:
		return (char *)(void *)"Swift";
	case LModeJavaScript:
		return (char *)(void *)"JavaScript";
	case LModeActionScript:
		return (char *)(void *)"ActionScript";
	case LModeTypeScript:
		return (char *)(void *)"TypeScript";
	case LModeDart:
		return (char *)(void *)"Dart";
	case LModeGo:
		return (char *)(void *)"Go";
	case LModeCSharp:
		return (char *)(void *)"C#";
	case LModeRust:
		return (char *)(void *)"Rust";
	case LModeR:
		return (char *)(void *)"R";
	case LModeKotlin:
		return (char *)(void *)"Kotlin";
	case LModeHTML:
		return (char *)(void *)"HTML/XML";
	case LModeCSS:
		return (char *)(void *)"CSS";
	default:
		return (char *)(void *)"Text";
	}
}

void mode_apply_from_path(Buffer *buf, char *path) {
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

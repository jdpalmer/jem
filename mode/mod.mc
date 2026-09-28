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
	if (path_ends_with_ci(path, ".c") || path_ends_with_ci(path, ".h") ||
		path_ends_with_ci(path, ".cc") || path_ends_with_ci(path, ".cpp") ||
		path_ends_with_ci(path, ".cxx") || path_ends_with_ci(path, ".c++") ||
		path_ends_with_ci(path, ".hh") || path_ends_with_ci(path, ".hpp") ||
		path_ends_with_ci(path, ".hxx") || path_ends_with_ci(path, ".h++") ||
		path_ends_with_ci(path, ".ipp") || path_ends_with_ci(path, ".tcc")) {
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
		cfg.use_tabs = 1;
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

/* Count leading ASCII spaces; -1 if the line mixes tabs into the indent. */
static int leading_spaces_only(Line *line) {
	char[..] v = {0};
	size_t i = 0;
	const char *p = NULL;
	if (line == NULL) {
		return 0;
	}
	v = line.view();
	p = ptr(v);
	while (i < len(v) && p[i] == ' ') {
		i++;
	}
	if (i < len(v) && p[i] == '\t') {
		return -1;
	}
	return (int)i;
}

static bool line_starts_with_tab(Line *line) {
	char[..] v = {0};
	if (line == NULL || line.byte_len() == 0) {
		return false;
	}
	v = line.view();
	return len(v) > 0 && ptr(v)[0] == '\t';
}

static int indent_gcd(int a, int b) {
	if (a < 0) {
		a = -a;
	}
	if (b < 0) {
		b = -b;
	}
	while (b != 0) {
		int t = a % b;
		a = b;
		b = t;
	}
	return a;
}

// mode_guess_indent inspects existing lines and updates buf.indent for C-family
// buffers: tabs vs spaces, and space width (2/4/8). Leaves defaults when the
// sample is empty or inconclusive.
void mode_guess_indent(Buffer *buf) {
	int tab_lines = 0;
	int space_lines = 0;
	int mixed_lines = 0;
	int space_gcd = 0;
	int i = 0;
	int nlines = 0;
	if (buf == NULL) {
		return;
	}
	nlines = buf.line_count_i();
	for (i = 1; i <= nlines; i++) {
		Line *line = buf.line(i);
		int spaces = 0;
		if (line == NULL || line.is_blank()) {
			continue;
		}
		if (line_starts_with_tab(line)) {
			tab_lines++;
			continue;
		}
		spaces = leading_spaces_only(line);
		if (spaces < 0) {
			mixed_lines++;
			continue;
		}
		if (spaces == 0) {
			continue;
		}
		space_lines++;
		space_gcd = space_gcd == 0 ? spaces : indent_gcd(space_gcd, spaces);
	}
	if (tab_lines == 0 && space_lines == 0) {
		return;
	}
	/* Prefer tabs when they clearly dominate (ignore sparse mixed noise). */
	if (tab_lines > space_lines && tab_lines >= space_lines + mixed_lines) {
		buf.indent.use_tabs = 1;
		buf.indent.width = 8;
		return;
	}
	if (space_lines == 0 || space_gcd <= 0) {
		return;
	}
	buf.indent.use_tabs = 0;
	/* Map GCD onto a practical step; huge GCDs (e.g. 16) collapse toward 8/4/2. */
	if (space_gcd > 8) {
		if (space_gcd % 8 == 0) {
			space_gcd = 8;
		} else if (space_gcd % 4 == 0) {
			space_gcd = 4;
		} else if (space_gcd % 2 == 0) {
			space_gcd = 2;
		} else {
			return;
		}
	}
	buf.indent.width = space_gcd;
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
	if (buf.lang_mode == LModeC) {
		mode_guess_indent(buf);
	}
}

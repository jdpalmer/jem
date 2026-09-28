// mode/func.mc — top/end/mark function for C-like and Python (jem.go mode_c / mode_py).
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <string.h>

/* mode_msg / mode_consume_echo live in match.mc */

static bool lang_is_brace_function(LangMode lang) {
	switch (lang) {
	case LModeC:
	case LModeModC:
	case LModeGo:
	case LModeJava:
	case LModeCSharp:
	case LModeKotlin:
	case LModeSwift:
	case LModeJavaScript:
	case LModeTypeScript:
	case LModeActionScript:
	case LModeDart:
	case LModeRust:
		return true;
	default:
		return false;
	}
}

static bool mode_c_top_of_function(Window *win, Buffer *buf) {
	int line_number = 0;
	int depth = 0;
	int ln = 0;
	if (win == NULL || buf == NULL || buf.line_count_i() == 0) {
		mode_msg("[Not in a function]");
		return false;
	}
	line_number = win.cursor.line;
	if (line_number == buf.eof_line()) {
		line_number = buf.line_count_i();
	}
	for (ln = line_number; ln >= 1; ln--) {
		Line *line = buf.line(ln);
		char[..] v = {0};
		const char *p = NULL;
		size_t n = 0;
		ptrdiff_t i = 0;
		bool found = false;
		if (line == NULL) {
			continue;
		}
		v = line.view();
		p = ptr(v);
		n = len(v);
		for (i = (ptrdiff_t)n - 1; i >= 0; i--) {
			char c = p[i];
			if (c == '}') {
				depth++;
				continue;
			}
			if (c == '{') {
				if (depth > 0) {
					depth--;
					continue;
				}
				if (line.indent_column() == 0) {
					found = true;
					break;
				}
			}
		}
		if (found) {
			int sig_line = ln;
			while (sig_line > 1) {
				Line *prev = buf.line(sig_line - 1);
				if (prev == NULL || !prev.is_blank()) {
					break;
				}
				sig_line--;
			}
			if (sig_line > 1) {
				sig_line--;
			}
			{
				Location loc = {0};
				loc.line = sig_line;
				loc.offset = 0;
				win.set_cursor(loc);
				win.did_move = true;
			}
			return true;
		}
	}
	mode_msg("[Not in a function]");
	return false;
}

static bool mode_c_end_of_function(Window *win, Buffer *buf) {
	int line_number = 0;
	int depth = 0;
	int ln = 0;
	int nlines = 0;
	if (win == NULL || buf == NULL || buf.line_count_i() == 0) {
		mode_msg("[Not in a function]");
		return false;
	}
	line_number = win.cursor.line;
	nlines = buf.line_count_i();
	if (line_number == buf.eof_line()) {
		line_number = nlines;
	}
	for (ln = line_number; ln <= nlines; ln++) {
		Line *line = buf.line(ln);
		char[..] v = {0};
		const char *p = NULL;
		size_t n = 0;
		size_t i = 0;
		if (line == NULL) {
			continue;
		}
		v = line.view();
		p = ptr(v);
		n = len(v);
		for (i = 0; i < n; i++) {
			char c = p[i];
			if (c == '{') {
				depth++;
			} else if (c == '}') {
				if (depth > 0) {
					depth--;
					continue;
				}
				{
					Location loc = {0};
					loc.line = ln;
					loc.offset = i;
					win.set_cursor(loc);
					win.did_move = true;
				}
				return true;
			}
		}
	}
	mode_msg("[Not in a function]");
	return false;
}

static bool mode_c_mark_function(Window *win, Buffer *buf) {
	int orig_line = 0;
	size_t orig_off = 0;
	Location saved = {0};
	if (win == NULL || buf == NULL) {
		return false;
	}
	orig_line = win.cursor.line;
	orig_off = win.cursor.offset;
	if (!mode_c_end_of_function(win, buf)) {
		return false;
	}
	win.mark.line = win.cursor.line;
	win.mark.offset = win.cursor.offset;
	saved.line = orig_line;
	saved.offset = orig_off;
	win.set_cursor(saved);
	if (!mode_c_top_of_function(win, buf)) {
		win.mark.line = 0;
		win.mark.offset = 0;
		return false;
	}
	win.did_move = true;
	mode_msg("[Function marked]");
	return true;
}

static bool py_keyword_at(Line *line, size_t i, const char *kw) {
	char[..] v = {0};
	size_t klen = 0;
	size_t n = 0;
	const char *p = NULL;
	char nc = 0;
	if (line == NULL || kw == NULL) {
		return false;
	}
	klen = strlen(kw);
	v = line.view();
	p = ptr(v);
	n = len(v);
	if (i + klen > n) {
		return false;
	}
	if (memcmp(p + i, kw, klen) != 0) {
		return false;
	}
	if (i + klen >= n) {
		return true;
	}
	nc = p[i + klen];
	return nc == ' ' || nc == '\t' || nc == ':' || nc == '(' || nc == '\r';
}

static int find_def_line_number(Buffer *buf, int line_number) {
	Line *line = NULL;
	size_t i = 0;
	size_t cur_ind = 0;
	if (buf == NULL || buf.line_count_i() == 0) {
		return 0;
	}
	if (line_number == buf.eof_line()) {
		line_number = buf.line_count_i();
	}
	line = buf.line(line_number);
	if (line == NULL) {
		return 0;
	}
	i = line.first_nonblank();
	if (py_keyword_at(line, i, "def") || py_keyword_at(line, i, "class")) {
		return line_number;
	}
	cur_ind = line.indent_column();
	while (line_number > 1) {
		size_t ind = 0;
		size_t j = 0;
		line_number--;
		line = buf.line(line_number);
		if (line == NULL) {
			continue;
		}
		if (!line.is_blank()) {
			ind = line.indent_column();
			if (ind < cur_ind) {
				j = line.first_nonblank();
				if (py_keyword_at(line, j, "def") || py_keyword_at(line, j, "class")) {
					return line_number;
				}
				cur_ind = ind;
			}
		}
	}
	return 0;
}

static bool mode_py_top_of_function(Window *win, Buffer *buf) {
	int def_line = 0;
	Location loc = {0};
	if (win == NULL || buf == NULL) {
		return false;
	}
	def_line = find_def_line_number(buf, win.cursor.line);
	if (def_line == 0) {
		mode_msg("[Not in a function]");
		return false;
	}
	loc.line = def_line;
	loc.offset = 0;
	win.set_cursor(loc);
	win.did_move = true;
	return true;
}

static bool mode_py_end_of_function(Window *win, Buffer *buf) {
	int def_line = 0;
	Line *def = NULL;
	size_t def_ind = 0;
	int line_number = 0;
	int last_line = 0;
	int nlines = 0;
	Location loc = {0};
	Line *line = NULL;
	if (win == NULL || buf == NULL) {
		return false;
	}
	def_line = find_def_line_number(buf, win.cursor.line);
	if (def_line == 0) {
		mode_msg("[Not in a function]");
		return false;
	}
	def = buf.line(def_line);
	if (def == NULL) {
		return false;
	}
	def_ind = def.indent_column();
	line_number = def_line + 1;
	last_line = def_line;
	nlines = buf.line_count_i();
	while (line_number <= nlines) {
		line = buf.line(line_number);
		if (line != NULL && !line.is_blank()) {
			if (line.indent_column() <= def_ind) {
				break;
			}
			last_line = line_number;
		}
		line_number++;
	}
	line = buf.line(last_line);
	loc.line = last_line;
	loc.offset = line != NULL ? line.byte_len() : 0;
	win.set_cursor(loc);
	win.did_move = true;
	return true;
}

static bool mode_py_mark_function(Window *win, Buffer *buf) {
	int orig_line = 0;
	size_t orig_off = 0;
	Location saved = {0};
	if (win == NULL || buf == NULL) {
		return false;
	}
	orig_line = win.cursor.line;
	orig_off = win.cursor.offset;
	if (!mode_py_end_of_function(win, buf)) {
		return false;
	}
	win.mark.line = win.cursor.line;
	win.mark.offset = win.cursor.offset;
	saved.line = orig_line;
	saved.offset = orig_off;
	win.set_cursor(saved);
	if (!mode_py_top_of_function(win, buf)) {
		win.mark.line = 0;
		win.mark.offset = 0;
		return false;
	}
	win.did_move = true;
	mode_msg("[function marked]");
	return true;
}

// mode_top_of_function jumps to the start of the enclosing function.
bool mode_top_of_function(bool f, int n) {
	Window *win = window_current();
	Buffer *buf = buffer_current();
	(void)f;
	(void)n;
	if (win == NULL || buf == NULL) {
		return false;
	}
	if (lang_is_brace_function(buf.lang_mode)) {
		return mode_c_top_of_function(win, buf);
	}
	if (buf.lang_mode == LModePython) {
		return mode_py_top_of_function(win, buf);
	}
	return false;
}

// mode_end_of_function jumps to the end of the enclosing function.
bool mode_end_of_function(bool f, int n) {
	Window *win = window_current();
	Buffer *buf = buffer_current();
	(void)f;
	(void)n;
	if (win == NULL || buf == NULL) {
		return false;
	}
	if (lang_is_brace_function(buf.lang_mode)) {
		return mode_c_end_of_function(win, buf);
	}
	if (buf.lang_mode == LModePython) {
		return mode_py_end_of_function(win, buf);
	}
	return false;
}

// mode_mark_function marks the enclosing function as the region.
bool mode_mark_function(bool f, int n) {
	Window *win = window_current();
	Buffer *buf = buffer_current();
	(void)f;
	(void)n;
	if (win == NULL || buf == NULL) {
		return false;
	}
	if (lang_is_brace_function(buf.lang_mode)) {
		return mode_c_mark_function(win, buf);
	}
	if (buf.lang_mode == LModePython) {
		return mode_py_mark_function(win, buf);
	}
	return false;
}

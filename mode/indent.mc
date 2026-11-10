// mode/indent.mc — C/Go/Python/Markdown calcIndent + Enter/Tab/} commands (jem.go/mode).
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

enum {
	GoIndentCols = 8,
	IndentPrefixCap = 512
};

static int line_col_of_offset(Line *line, size_t offset) {
	if (line == NULL) {
		return 0;
	}
	if (offset > line.byte_len()) {
		offset = line.byte_len();
	}
	return (int)line.display_col_at_offset(offset);
}

static size_t line_first_nonblank_ch(Line *line, char *fc) {
	if (fc != NULL) {
		*fc = 0;
	}
	if (line == NULL) {
		return 0;
	}
	size_t i = line.first_nonblank();
	if (fc != NULL && i < line.byte_len()) {
		*fc = line.data[i];
	}
	return i;
}

static bool line_starts_with(Line *line, const char *text) {
	if (line == NULL || text == NULL) {
		return false;
	}
	size_t off = line.first_nonblank();
	size_t tlen = strlen(text);
	if (off + tlen > line.byte_len()) {
		return false;
	}
	return memcmp(ptr(line.data) + off, text, tlen) == 0;
}

static bool line_is_comment_or_preproc(Buffer *buf, int line_number) {
	char ch = 0;
	(void)buf;
	Line *line = buf.line(line_number);
	if (line == NULL || line.is_blank()) {
		return false;
	}
	size_t off = line_first_nonblank_ch(line, &ch);
	if (off >= line.byte_len()) {
		return false;
	}
	if (ch == '*' || ch == '#') {
		return true;
	}
	if (ch == '/' && off + 1 < line.byte_len()) {
		char next = line.data[off + 1];
		if (next == '/' || next == '*') {
			return true;
		}
	}
	return false;
}

static bool line_is_preproc(Buffer *buf, int line_number) {
	Line *line = buf.line(line_number);
	char ch = 0;
	if (line == NULL || line.is_blank()) {
		return false;
	}
	line_first_nonblank_ch(line, &ch);
	return ch == '#';
}

static int prev_code_line_number(Buffer *buf, int line_number) {
	int ln = line_number;
	while (ln > 1) {
		Line *p = {0};
		ln--;
		p = buf.line(ln);
		if (p == NULL) {
			continue;
		}
		if (!p.is_blank() && !line_is_comment_or_preproc(buf, ln)) {
			return ln;
		}
	}
	return 0;
}

static bool line_is_case_label(Line *line) {
	if (line == NULL) {
		return false;
	}
	size_t off = line.first_nonblank();
	if (off >= line.byte_len()) {
		return false;
	}
	if (off + 4 <= line.byte_len() && memcmp(ptr(line.data) + off, "case", 4) == 0) {
		if (off + 4 == line.byte_len()) {
			return true;
		}
		{
			char nc = line.data[off + 4];
			return nc == ' ' || nc == '\t' || nc == '(';
		}
	}
	if (off + 7 <= line.byte_len() && memcmp(ptr(line.data) + off, "default", 7) == 0) {
		return true;
	}
	return false;
}

static bool line_ends_with_continuation(Line *line) {
	if (line == NULL || line.byte_len() == 0) {
		return false;
	}
	char last = line.last_byte();
	switch (last) {
	case ',':
	case '\\':
	case '?':
	case ':':
	case '+':
	case '-':
	case '*':
	case '/':
	case '%':
	case '&':
	case '|':
	case '^':
	case '=':
	case '<':
	case '>':
	case '!':
		return true;
	default:
		return false;
	}
}

static int calc_comment_indent(Buffer *buf, int line_number) {
	int prev_line = line_number;
	while (prev_line > 1) {
		Line *prev = {0};
		prev_line--;
		prev = buf.line(prev_line);
		if (prev == NULL || prev.is_blank()) {
			continue;
		}
		if (line_starts_with(prev, "/*")) {
			return (int)prev.indent_column() + 1;
		}
		if (prev.first_nonblank() < prev.byte_len()) {
			char ch = prev.data[prev.first_nonblank()];
			if (ch == '*' || line_starts_with(prev, "*/")) {
				return (int)prev.indent_column();
			}
		}
		break;
	}
	return 0;
}

static int find_case_indent(Buffer *buf, int line_number) {
	int c_indent = buf.indent.width;
	int c_colon = buf.indent.label;
	int ln = line_number;
	for (; ln >= 1; ln--) {
		Line *line = buf.line(ln);
		size_t i = 0;
		if (line == NULL) {
			continue;
		}
		for (i = 0; i < line.byte_len(); i++) {
			if (line.data[i] == '{') {
				int base = (int)line.indent_column() + c_indent + c_colon;
				if (base < 0) {
					return 0;
				}
				return base;
			}
		}
	}
	return 0;
}

/* Line that owns a block-open `{` when the brace hangs on a continued
 * declarator / condition (e.g. multi-line function signature). */
static int block_anchor_line(Buffer *buf, int open_line) {
	int ln = open_line;
	if (buf == NULL || ln < 1) {
		return open_line;
	}
	for (;;) {
		int prev = prev_code_line_number(buf, ln);
		Line *pl = NULL;
		char last = 0;
		if (prev == 0) {
			return ln;
		}
		pl = buf.line(prev);
		if (pl == NULL || pl.byte_len() == 0) {
			return ln;
		}
		last = pl.last_byte();
		if (last == ';' || last == '{' || last == '}') {
			return ln;
		}
		ln = prev;
	}
}

static int find_closing_delimiter_indent(Buffer *buf, int line_number, size_t offset) {
	Line *line = buf.line(line_number);
	Location open = {0};
	if (line == NULL || offset >= line.byte_len()) {
		return 0;
	}
	char ch = line.data[offset];
	if (ch != '}' && ch != ')' && ch != ']') {
		return 0;
	}
	Location here = { .line = line_number, .offset = offset };
	if (!mode_find_matching_delimiter(buf, here, &open)) {
		return 0;
	}
	{
		Line *ol = buf.line(open.line);
		if (ol == NULL) {
			return 0;
		}
		if (ch == '}') {
			int anchor = block_anchor_line(buf, open.line);
			Line *al = buf.line(anchor);
			if (al == NULL) {
				return (int)ol.indent_column();
			}
			return (int)al.indent_column();
		}
		return line_col_of_offset(ol, open.offset);
	}
}

static bool find_unmatched_open_delim(Buffer *buf, int line_number, size_t offset, Location *best, char *kind) {
	Location paren = {0};
	Location bracket = {0};
	bool found = false;
	Location at = { .line = line_number, .offset = offset };
	if (mode_find_enclosing_open(buf, at, '(', ')', &paren)) {
		*best = paren;
		*kind = '(';
		found = true;
	}
	if (mode_find_enclosing_open(buf, at, '[', ']', &bracket)) {
		if (!found || bracket.line > best.line ||
			(bracket.line == best.line && bracket.offset > best.offset)) {
			*best = bracket;
			*kind = '[';
			found = true;
		}
	}
	return found;
}

static int find_delimiter_continuation_indent(Buffer *buf, int line_number, size_t offset) {
	Location open = {0};
	char kind = 0;
	if (!find_unmatched_open_delim(buf, line_number, offset, &open, &kind)) {
		return -1;
	}
	(void)kind;
	Line *line = buf.line(open.line);
	if (line == NULL) {
		return -1;
	}
	size_t tail = open.offset + 1;
	while (tail < line.byte_len()) {
		char c = line.data[tail];
		if (c != ' ' && c != '\t') {
			return line_col_of_offset(line, tail);
		}
		tail++;
	}
	return (int)line.indent_column() + buf.indent.width;
}

static int find_enclosing_block_indent(Buffer *buf, int line_number, size_t offset) {
	Location open = {0};
	Location at = { .line = line_number, .offset = offset };
	if (!mode_find_enclosing_open(buf, at, '{', '}', &open)) {
		return -1;
	}
	int anchor = block_anchor_line(buf, open.line);
	Line *line = buf.line(anchor);
	if (line == NULL) {
		return -1;
	}
	return (int)line.indent_column() + buf.indent.width;
}

static int calc_indent_c(Buffer *buf, int line_number) {
	Line *line = buf.line(line_number);
	int c_indent = buf.indent.width;
	int c_brace = buf.indent.brace;
	char fc = 0;
	if (line == NULL) {
		return 0;
	}
	size_t first = line_first_nonblank_ch(line, &fc);
	if (line_is_preproc(buf, line_number)) {
		return 0;
	}
	if (!line.is_blank()) {
		if (fc == '*' || fc == '/') {
			return calc_comment_indent(buf, line_number);
		}
	}
	if (fc == '}' || fc == ')' || fc == ']') {
		return find_closing_delimiter_indent(buf, line_number, first);
	}
	if (line_is_case_label(line)) {
		return find_case_indent(buf, line_number);
	}
	int indent = find_delimiter_continuation_indent(buf, line_number, first);
	if (indent >= 0) {
		return indent;
	}
	indent = find_enclosing_block_indent(buf, line_number, first);
	if (indent >= 0) {
		return indent;
	}
	int ref_line = prev_code_line_number(buf, line_number);
	if (ref_line == 0) {
		return 0;
	}
	Line *ref = buf.line(ref_line);
	if (ref == NULL) {
		return 0;
	}
	int ind = (int)ref.indent_column();
	if (ref.last_byte() == ':' && line_is_case_label(ref)) {
		ind += c_indent;
	} else if (line_ends_with_continuation(ref)) {
		ind += c_indent;
	}
	if (fc == '{') {
		ind += c_brace;
	}
	if (ind < 0) {
		return 0;
	}
	return ind;
}

static int calc_indent_go(Buffer *buf, int line_number) {
	int saved = buf.indent.width;
	buf.indent.width = GoIndentCols;
	int ind = calc_indent_c(buf, line_number);
	buf.indent.width = saved;
	return ind;
}

static bool lp_keyword(Line *line, size_t i, const char *kw) {
	if (line == NULL || kw == NULL) {
		return false;
	}
	size_t klen = strlen(kw);
	if (i + klen > line.byte_len()) {
		return false;
	}
	if (memcmp(ptr(line.data) + i, kw, klen) != 0) {
		return false;
	}
	size_t after = i + klen;
	if (after >= line.byte_len()) {
		return true;
	}
	char nc = line.data[after];
	return nc == ' ' || nc == '\t' || nc == ':' || nc == '(' || nc == '\r';
}

static int prev_code_line_number_py(Buffer *buf, int line_number) {
	int ln = line_number;
	while (ln > 1) {
		Line *p = {0};
		ln--;
		p = buf.line(ln);
		if (p == NULL) {
			continue;
		}
		if (!p.is_blank() && p.first_byte() != '#') {
			return ln;
		}
	}
	return 0;
}

static bool line_is_deindent_kw(Line *line) {
	size_t i = line.first_nonblank();
	return lp_keyword(line, i, "elif") || lp_keyword(line, i, "else") ||
		lp_keyword(line, i, "except") || lp_keyword(line, i, "finally");
}

static bool line_is_dedent_stmt(Line *line) {
	size_t i = line.first_nonblank();
	return lp_keyword(line, i, "return") || lp_keyword(line, i, "break") ||
		lp_keyword(line, i, "continue") || lp_keyword(line, i, "raise") ||
		lp_keyword(line, i, "pass");
}

static int calc_indent_py(Buffer *buf, int line_number) {
	Line *line = buf.line(line_number);
	int py_width = buf.indent.width;
	int py_cont = buf.indent.continued;
	if (line != NULL && line_is_deindent_kw(line)) {
		int ref_line = prev_code_line_number_py(buf, line_number);
		if (ref_line == 0) {
			return 0;
		}
		Line *ref = buf.line(ref_line);
		if (ref == NULL) {
			return 0;
		}
		{
			int ind = (int)ref.indent_column() - py_width;
			if (ind < 0) {
				return 0;
			}
			return ind;
		}
	}
	int ref_line = prev_code_line_number_py(buf, line_number);
	if (ref_line == 0) {
		return 0;
	}
	int base_line = ref_line;
	while (base_line > 1) {
		Line *l = buf.line(base_line);
		int next_ref = 0;
		if (l == NULL) {
			break;
		}
		if (l.last_byte() != '\\') {
			break;
		}
		next_ref = prev_code_line_number_py(buf, base_line);
		if (next_ref == 0) {
			break;
		}
		base_line = next_ref;
	}
	Line *ref = buf.line(base_line);
	if (ref == NULL) {
		return 0;
	}
	int ref_ind = (int)ref.indent_column();
	Line *prev = buf.line(ref_line);
	if (prev == NULL) {
		return ref_ind;
	}
	char last = prev.last_byte();
	if (last == '\\') {
		return ref_ind + py_cont;
	}
	if (last == ':') {
		return ref_ind + py_width;
	}
	if (last == '(' || last == '[' || last == '{') {
		return ref_ind + py_width;
	}
	if (line_is_dedent_stmt(prev)) {
		int cur_ind = (int)prev.indent_column();
		int ln = prev_code_line_number_py(buf, ref_line);
		while (ln != 0) {
			Line *p = buf.line(ln);
			int ind = 0;
			if (p == NULL) {
				ln = prev_code_line_number_py(buf, ln);
				continue;
			}
			ind = (int)p.indent_column();
			if (ind < cur_ind) {
				if (ind < 0) {
					return 0;
				}
				return ind;
			}
			ln = prev_code_line_number_py(buf, ln);
		}
		return 0;
	}
	return ref_ind;
}

static int mode_calc_indent(Buffer *buf, int line_number) {
	if (buf == NULL) {
		return 0;
	}
	switch (buf.lang_mode) {
	case LModeGo:
	case LModeModC:
		/* Always compute in 8-col tab steps so a stale indent-width of 2
		 * cannot turn "tab mode" into two spaces. */
		return calc_indent_go(buf, line_number);
	case LModePython:
		return calc_indent_py(buf, line_number);
	case LModeC:
		if (buf.indent.use_tabs != 0) {
			return calc_indent_go(buf, line_number);
		}
		return calc_indent_c(buf, line_number);
	default:
		return -1;
	}
}

static size_t build_space_indent(int col, char *out, size_t cap) {
	char[..] o = ranged(out, cap);
	int i = 0;
	if (col < 0) {
		col = 0;
	}
	if ((size_t)col > cap) {
		col = (int)cap;
	}
	for (; i < col; i++) {
		o[i] = ' ';
	}
	return col;
}

static size_t build_tab_indent(int col, char *out, size_t cap) {
	char[..] o = ranged(out, cap);
	int i = 0;
	size_t n = 0;
	if (col <= 0) {
		return 0;
	}
	int tabs = col / 8;
	int spaces = col % 8;
	for (; i < tabs && n < cap; i++) {
		o[n++] = '\t';
	}
	for (i = 0; i < spaces && n < cap; i++) {
		o[n++] = ' ';
	}
	return n;
}

/* Replace leading whitespace; records undo edits (caller owns begin/end_command). */
static bool replace_leading(Window *win, char *prefix, size_t plen) {
	if (win == NULL || win.buffer == NULL) {
		return false;
	}
	Line *line = win.buffer.line(win.cursor.line);
	if (line == NULL) {
		return false;
	}
	size_t first = line.first_nonblank();
	size_t old_off = win.cursor.offset;
	Location begin = { .line = win.cursor.line, .offset = 0 };
	Location end = { .line = win.cursor.line, .offset = first };
	pool p {
		char[..] old = {0};
		if (first > 0) {
			old = p.alloc(char, first);
			if (ptr(old) == NULL) {
				return false;
			}
			memcpy(ptr(old), ptr(line.data), first);
		}
		if (!win.set_text(begin, end, prefix, plen)) {
			return false;
		}
		if (first > 0) {
			buffer_undo.record_edit(UndoDelete, begin.line, begin.offset, ptr(old), first);
		}
		if (plen > 0) {
			buffer_undo.record_edit(UndoInsert, begin.line, begin.offset, prefix, plen);
		}
		if (old_off <= first) {
			win.cursor.offset = plen;
		} else {
			win.cursor.offset = old_off - first + plen;
		}
		line = win.buffer.line(win.cursor.line);
		if (line != NULL && win.cursor.offset > line.byte_len()) {
			win.cursor.offset = line.byte_len();
		}
		win.set_cursor(win.cursor);
		return true;
	}
}

static bool set_line_indent_spaces(Window *win, int col) {
	char prefix[IndentPrefixCap] = {0};
	if (col < 0) {
		return false;
	}
	size_t n = build_space_indent(col, prefix, sizeof(prefix));
	return replace_leading(win, prefix, n);
}

static bool set_line_indent_tabs(Window *win, int col) {
	char prefix[IndentPrefixCap] = {0};
	if (col < 0) {
		return false;
	}
	size_t n = build_tab_indent(col, prefix, sizeof(prefix));
	return replace_leading(win, prefix, n);
}

static bool mode_uses_tab_indent(Buffer *buf) {
	if (buf == NULL) {
		return false;
	}
	if (buf.indent.use_tabs != 0) {
		return true;
	}
	return buf.lang_mode == LModeGo || buf.lang_mode == LModeModC;
}

static bool set_line_indent_for_mode(Window *win, int col) {
	if (win == NULL || win.buffer == NULL) {
		return false;
	}
	if (mode_uses_tab_indent(win.buffer)) {
		return set_line_indent_tabs(win, col);
	}
	return set_line_indent_spaces(win, col);
}

static int prev_nonblank_line(Buffer *buf, int line_number) {
	int ln = line_number;
	while (ln > 1) {
		Line *p = {0};
		ln--;
		p = buf.line(ln);
		if (p != NULL && !p.is_blank()) {
			return ln;
		}
	}
	return 0;
}

/* Markdown list/blockquote prefix from a reference line (jem.go mdBuildPrefix). */
static size_t md_build_prefix(Line *line, char *out, size_t cap) {
	char[..] o = ranged(out, cap);
	size_t n = 0;
	size_t i = 0;
	if (line == NULL || cap == 0) {
		return 0;
	}
	char[..] p = line.view();
	{
		while (i < len(p) && (p[i] == ' ' || p[i] == '\t') && n < cap) {
			o[n++] = p[i++];
		}
		while (i < len(p) && p[i] == '>' && n + 2 <= cap) {
			o[n++] = '>';
			o[n++] = ' ';
			i++;
			if (i < len(p) && p[i] == ' ') {
				i++;
			}
			while (i < len(p) && p[i] == ' ') {
				i++;
			}
		}
		if (i < len(p) && (p[i] == '-' || p[i] == '*' || p[i] == '+')) {
			if (i + 1 < len(p) && p[i + 1] == ' ' && n + 2 <= cap) {
				o[n++] = p[i];
				o[n++] = ' ';
				return n;
			}
		}
		size_t save = i;
		int num = 0;
		while (i < len(p) && p[i] >= '0' && p[i] <= '9') {
			num = num * 10 + (p[i] - '0');
			i++;
		}
		if (i > save && i < len(p)) {
			char del = p[i];
			if ((del == '.' || del == ')') && i + 1 < len(p) && p[i + 1] == ' ') {
				int next = num + 1;
				char digits[16] = {0};
				int dlen = 0;
				int t = next;
				int j = 0;
				if (t == 0) {
					digits[dlen++] = '0';
				} else {
					while (t > 0 && dlen < 15) {
						digits[dlen++] = (char)('0' + (t % 10));
						t /= 10;
					}
					for (; j < dlen / 2; j++) {
						char tmp = digits[j];
						digits[j] = digits[dlen - 1 - j];
						digits[dlen - 1 - j] = tmp;
					}
				}
				if (n + (size_t)dlen + 2 <= cap) {
					memcpy(out + n, digits, dlen);
					n += dlen;
					o[n++] = del;
					o[n++] = ' ';
					return n;
				}
			}
		}
	}
	return n;
}

static bool insert_md_prefix_after_newline(Window *win) {
	char prefix[IndentPrefixCap] = {0};
	if (win == NULL || win.buffer == NULL || win.cursor.line <= 1) {
		return true;
	}
	Buffer *buf = win.buffer;
	Line *prev = buf.line(win.cursor.line - 1);
	if (prev == NULL) {
		return true;
	}
	size_t n = md_build_prefix(prev, prefix, sizeof(prefix));
	if (n == 0) {
		return true;
	}
	Location at = win.cursor;
	if (!win.set_text(win.cursor, win.cursor, prefix, n)) {
		return false;
	}
	buffer_undo.record_edit(UndoInsert, at.line, at.offset, prefix, n);
	return true;
}

bool mode_newline_and_indent(bool f, int n) {
	Window *win = window_current();
	char nl = '\n';
	(void)f;
	if (win == NULL || win.buffer == NULL) {
		return true;
	}
	if (n < 0) {
		return false;
	}
	if (n < 1) {
		n = 1;
	}
	buffer_undo.begin_command(win.buffer, win.cursor);
	while (n-- > 0) {
		Location at = win.cursor;
		if (!win.set_text(win.cursor, win.cursor, &nl, 1)) {
			break;
		}
		buffer_undo.record_edit(UndoInsert, at.line, at.offset, &nl, 1);
		if (win.buffer.lang_mode == LModeMarkdown) {
			if (!insert_md_prefix_after_newline(win)) {
				break;
			}
			continue;
		}
		if (win.buffer.lang_mode == LModeNone) {
			continue;
		}
		int col = mode_calc_indent(win.buffer, win.cursor.line);
		if (col < 0) {
			continue;
		}
		if (!set_line_indent_for_mode(win, col)) {
			break;
		}
	}
	buffer_undo.end_command();
	return true;
}

bool mode_indent_line(bool f, int n) {
	Window *win = window_current();
	(void)f;
	(void)n;
	if (win == NULL || win.buffer == NULL) {
		return true;
	}
	if (win.buffer.lang_mode == LModeNone) {
		return true;
	}
	if (win.buffer.lang_mode == LModeMarkdown) {
		int ref = prev_nonblank_line(win.buffer, win.cursor.line);
		char prefix[IndentPrefixCap] = {0};
		if (ref == 0) {
			return true;
		}
		size_t plen = md_build_prefix(win.buffer.line(ref), prefix, sizeof(prefix));
		buffer_undo.begin_command(win.buffer, win.cursor);
		replace_leading(win, prefix, plen);
		buffer_undo.end_command();
		return true;
	}
	int col = mode_calc_indent(win.buffer, win.cursor.line);
	if (col < 0) {
		return true;
	}
	buffer_undo.begin_command(win.buffer, win.cursor);
	set_line_indent_for_mode(win, col);
	buffer_undo.end_command();
	return true;
}

bool mode_close_brace(bool f, int n) {
	Window *win = window_current();
	char ch = '}';
	int i = 0;
	(void)f;
	if (win == NULL || win.buffer == NULL) {
		return true;
	}
	if (n <= 0) {
		n = 1;
	}
	if (win.buffer.lang_mode != LModeGo && win.buffer.lang_mode != LModeC &&
		win.buffer.lang_mode != LModeModC) {
		buffer_undo.begin_command(win.buffer, win.cursor);
		for (; i < n; i++) {
			Location at = win.cursor;
			if (!win.set_text(win.cursor, win.cursor, &ch, 1)) {
				break;
			}
			buffer_undo.record_edit(UndoInsert, at.line, at.offset, &ch, 1);
		}
		buffer_undo.end_command();
		return true;
	}
	buffer_undo.begin_command(win.buffer, win.cursor);
	if (mode_uses_tab_indent(win.buffer)) {
		if (!set_line_indent_tabs(win, 0)) {
			buffer_undo.end_command();
			return false;
		}
		win.cursor.offset = 0;
		win.set_cursor(win.cursor);
		for (i = 0; i < n; i++) {
			Location at = win.cursor;
			if (!win.set_text(win.cursor, win.cursor, &ch, 1)) {
				buffer_undo.end_command();
				return false;
			}
			buffer_undo.record_edit(UndoInsert, at.line, at.offset, &ch, 1);
		}
		int col = mode_calc_indent(win.buffer, win.cursor.line);
		if (!set_line_indent_tabs(win, col)) {
			buffer_undo.end_command();
			return false;
		}
		{
			Line *line = win.buffer.line(win.cursor.line);
			if (line != NULL) {
				win.cursor.offset = line.byte_len();
				win.set_cursor(win.cursor);
			}
		}
	} else {
		int col = calc_indent_c(win.buffer, win.cursor.line);
		if (!set_line_indent_spaces(win, col)) {
			buffer_undo.end_command();
			return false;
		}
		for (i = 0; i < n; i++) {
			Location at = win.cursor;
			if (!win.set_text(win.cursor, win.cursor, &ch, 1)) {
				buffer_undo.end_command();
				return false;
			}
			buffer_undo.record_edit(UndoInsert, at.line, at.offset, &ch, 1);
		}
		{
			Line *line = win.buffer.line(win.cursor.line);
			size_t off = (size_t)col + (size_t)n;
			if (line != NULL && off > line.byte_len()) {
				off = line.byte_len();
			}
			win.cursor.offset = off;
			win.set_cursor(win.cursor);
		}
	}
	buffer_undo.end_command();
	return true;
}

// mode/indent.mc — C/Go/Python/Markdown calcIndent + Enter/Tab/} commands (jem.go/mode).
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

enum {
	CodeScanCode = 0,
	CodeScanString = 1,
	CodeScanStringEsc = 2,
	CodeScanChar = 3,
	CodeScanCharEsc = 4,
	CodeScanLineCmt = 5,
	CodeScanBlockCmt = 6,
	CodeScanBlockCmtStar = 7,
	IndentStackMax = 512,
	GoIndentCols = 8,
	IndentPrefixCap = 512
};

struct DelimStacks {
	Location parens[IndentStackMax];
	Location brackets[IndentStackMax];
	Location braces[IndentStackMax];
	int nparens;
	int nbrackets;
	int nbraces;
};

static int code_scan_step(int state, char c, char next, bool *in_code) {
	*in_code = false;
	switch (state) {
	case CodeScanString:
		if (c == '\\') {
			return CodeScanStringEsc;
		}
		if (c == '"') {
			return CodeScanCode;
		}
		return CodeScanString;
	case CodeScanStringEsc:
		return CodeScanString;
	case CodeScanChar:
		if (c == '\\') {
			return CodeScanCharEsc;
		}
		if (c == '\'') {
			return CodeScanCode;
		}
		return CodeScanChar;
	case CodeScanCharEsc:
		return CodeScanChar;
	case CodeScanLineCmt:
		return CodeScanLineCmt;
	case CodeScanBlockCmt:
		if (c == '*') {
			return CodeScanBlockCmtStar;
		}
		return CodeScanBlockCmt;
	case CodeScanBlockCmtStar:
		if (c == '/') {
			return CodeScanCode;
		}
		if (c == '*') {
			return CodeScanBlockCmtStar;
		}
		return CodeScanBlockCmt;
	default:
		if (c == '"') {
			return CodeScanString;
		}
		if (c == '\'') {
			return CodeScanChar;
		}
		if (c == '/') {
			if (next == '/') {
				return CodeScanLineCmt;
			}
			if (next == '*') {
				return CodeScanBlockCmt;
			}
			*in_code = true;
			return CodeScanCode;
		}
		*in_code = true;
		return CodeScanCode;
	}
}

static void stack_push(Location *stack, int *n, Location loc) {
	if (*n < IndentStackMax) {
		stack[*n] = loc;
		(*n)++;
	}
}

static bool stack_pop(Location *stack, int *n, Location *out) {
	(void)stack;
	if (*n <= 0) {
		return false;
	}
	(*n)--;
	if (out != NULL) {
		*out = stack[*n];
	}
	return true;
}

static void scan_delim_stacks(Buffer *buf, int end_line, size_t end_offset, DelimStacks *out) {
	int state = CodeScanCode;
	int ln = 0;
	memset(out, 0, sizeof(*out));
	if (buf == NULL || end_line < 1) {
		return;
	}
	for (ln = 1; ln <= end_line; ln++) {
		Line *line = {0};
		size_t limit = 0;
		size_t i = 0;
		if (state == CodeScanLineCmt) {
			state = CodeScanCode;
		}
		line = buf.line(ln);
		if (line == NULL) {
			continue;
		}
		limit = line.byte_len();
		if (ln == end_line) {
			if (end_offset < limit) {
				limit = end_offset;
			}
		}
		for (i = 0; i < limit; i++) {
			char c = line.data[i];
			char next = 0;
			bool in_code = false;
			Location loc = {0};
			if (i + 1 < line.byte_len()) {
				next = line.data[i + 1];
			}
			state = code_scan_step(state, c, next, &in_code);
			if (!in_code) {
				if (c == '/' && (next == '/' || next == '*') &&
					(state == CodeScanLineCmt || state == CodeScanBlockCmt)) {
					i++;
				}
				continue;
			}
			loc.line = ln;
			loc.offset = i;
			switch (c) {
			case '(':
				stack_push(out.parens, &out.nparens, loc);
				break;
			case ')':
				stack_pop(out.parens, &out.nparens, NULL);
				break;
			case '[':
				stack_push(out.brackets, &out.nbrackets, loc);
				break;
			case ']':
				stack_pop(out.brackets, &out.nbrackets, NULL);
				break;
			case '{':
				stack_push(out.braces, &out.nbraces, loc);
				break;
			case '}':
				stack_pop(out.braces, &out.nbraces, NULL);
				break;
			}
		}
	}
}

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
	size_t i = 0;
	if (fc != NULL) {
		*fc = 0;
	}
	if (line == NULL) {
		return 0;
	}
	i = line.first_nonblank();
	if (fc != NULL && i < line.byte_len()) {
		*fc = line.data[i];
	}
	return i;
}

static bool line_starts_with(Line *line, const char *text) {
	size_t off = 0;
	size_t tlen = 0;
	if (line == NULL || text == NULL) {
		return false;
	}
	off = line.first_nonblank();
	tlen = strlen(text);
	if (off + tlen > line.byte_len()) {
		return false;
	}
	return memcmp(ptr(line.data) + off, text, tlen) == 0;
}

static bool line_is_comment_or_preproc(Buffer *buf, int line_number) {
	Line *line = {0};
	size_t off = 0;
	char ch = 0;
	(void)buf;
	line = buf.line(line_number);
	if (line == NULL || line.is_blank()) {
		return false;
	}
	off = line_first_nonblank_ch(line, &ch);
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
	size_t off = 0;
	if (line == NULL) {
		return false;
	}
	off = line.first_nonblank();
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
	char last = 0;
	if (line == NULL || line.byte_len() == 0) {
		return false;
	}
	last = line.last_byte();
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
	int ln = 0;
	for (ln = line_number; ln >= 1; ln--) {
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

static int find_closing_delimiter_indent(Buffer *buf, int line_number, size_t offset) {
	Line *line = buf.line(line_number);
	DelimStacks stacks = {0};
	char ch = 0;
	Location open = {0};
	bool ok = false;
	if (line == NULL || offset >= line.byte_len()) {
		return 0;
	}
	ch = line.data[offset];
	scan_delim_stacks(buf, line_number, offset, &stacks);
	switch (ch) {
	case '}':
		ok = stack_pop(stacks.braces, &stacks.nbraces, &open);
		if (!ok) {
			return 0;
		}
		{
			Line *ol = buf.line(open.line);
			if (ol != NULL) {
				return (int)ol.indent_column();
			}
		}
		return 0;
	case ')':
		ok = stack_pop(stacks.parens, &stacks.nparens, &open);
		break;
	case ']':
		ok = stack_pop(stacks.brackets, &stacks.nbrackets, &open);
		break;
	default:
		return 0;
	}
	if (!ok) {
		return 0;
	}
	{
		Line *ol = buf.line(open.line);
		if (ol != NULL) {
			return line_col_of_offset(ol, open.offset);
		}
	}
	return 0;
}

static bool find_unmatched_open_delim(Buffer *buf, int line_number, size_t offset, Location *best, char *kind) {
	DelimStacks stacks = {0};
	bool found = false;
	scan_delim_stacks(buf, line_number, offset, &stacks);
	if (stacks.nparens > 0) {
		*best = stacks.parens[stacks.nparens - 1];
		*kind = '(';
		found = true;
	}
	if (stacks.nbrackets > 0) {
		Location loc = stacks.brackets[stacks.nbrackets - 1];
		if (!found || loc.line > best.line || (loc.line == best.line && loc.offset > best.offset)) {
			*best = loc;
			*kind = '[';
			found = true;
		}
	}
	return found;
}

static int find_delimiter_continuation_indent(Buffer *buf, int line_number, size_t offset) {
	Location open = {0};
	char kind = 0;
	Line *line = {0};
	size_t tail = 0;
	if (!find_unmatched_open_delim(buf, line_number, offset, &open, &kind)) {
		return -1;
	}
	(void)kind;
	line = buf.line(open.line);
	if (line == NULL) {
		return -1;
	}
	tail = open.offset + 1;
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
	DelimStacks stacks = {0};
	Location open = {0};
	Line *line = {0};
	scan_delim_stacks(buf, line_number, offset, &stacks);
	if (stacks.nbraces <= 0) {
		return -1;
	}
	open = stacks.braces[stacks.nbraces - 1];
	line = buf.line(open.line);
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
	size_t first = 0;
	int indent = 0;
	int ref_line = 0;
	Line *ref = {0};
	int ind = 0;
	if (line == NULL) {
		return 0;
	}
	first = line_first_nonblank_ch(line, &fc);
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
	indent = find_delimiter_continuation_indent(buf, line_number, first);
	if (indent >= 0) {
		return indent;
	}
	indent = find_enclosing_block_indent(buf, line_number, first);
	if (indent >= 0) {
		return indent;
	}
	ref_line = prev_code_line_number(buf, line_number);
	if (ref_line == 0) {
		return 0;
	}
	ref = buf.line(ref_line);
	if (ref == NULL) {
		return 0;
	}
	ind = (int)ref.indent_column();
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
	int ind = 0;
	buf.indent.width = GoIndentCols;
	ind = calc_indent_c(buf, line_number);
	buf.indent.width = saved;
	return ind;
}

static bool lp_keyword(Line *line, size_t i, const char *kw) {
	size_t klen = 0;
	size_t after = 0;
	char nc = 0;
	if (line == NULL || kw == NULL) {
		return false;
	}
	klen = strlen(kw);
	if (i + klen > line.byte_len()) {
		return false;
	}
	if (memcmp(ptr(line.data) + i, kw, klen) != 0) {
		return false;
	}
	after = i + klen;
	if (after >= line.byte_len()) {
		return true;
	}
	nc = line.data[after];
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
	int ref_line = 0;
	int base_line = 0;
	Line *ref = {0};
	int ref_ind = 0;
	char last = 0;
	if (line != NULL && line_is_deindent_kw(line)) {
		ref_line = prev_code_line_number_py(buf, line_number);
		if (ref_line == 0) {
			return 0;
		}
		ref = buf.line(ref_line);
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
	ref_line = prev_code_line_number_py(buf, line_number);
	if (ref_line == 0) {
		return 0;
	}
	base_line = ref_line;
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
	ref = buf.line(base_line);
	if (ref == NULL) {
		return 0;
	}
	ref_ind = (int)ref.indent_column();
	{
		Line *prev = buf.line(ref_line);
		if (prev == NULL) {
			return ref_ind;
		}
		last = prev.last_byte();
	}
	if (last == '\\') {
		return ref_ind + py_cont;
	}
	if (last == ':') {
		return ref_ind + py_width;
	}
	if (last == '(' || last == '[' || last == '{') {
		return ref_ind + py_width;
	}
	{
		Line *prev = buf.line(ref_line);
		if (prev != NULL && line_is_dedent_stmt(prev)) {
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
	int i = 0;
	if (col < 0) {
		col = 0;
	}
	if ((size_t)col > cap) {
		col = (int)cap;
	}
	for (i = 0; i < col; i++) {
		out[i] = ' ';
	}
	return col;
}

static size_t build_tab_indent(int col, char *out, size_t cap) {
	int tabs = 0;
	int spaces = 0;
	int i = 0;
	size_t n = 0;
	if (col <= 0) {
		return 0;
	}
	tabs = col / 8;
	spaces = col % 8;
	for (i = 0; i < tabs && n < cap; i++) {
		out[n++] = '\t';
	}
	for (i = 0; i < spaces && n < cap; i++) {
		out[n++] = ' ';
	}
	return n;
}

/* Replace leading whitespace; records undo edits (caller owns begin/end_command). */
static bool replace_leading(Window *win, char *prefix, size_t plen) {
	Line *line = {0};
	size_t first = 0;
	size_t old_off = 0;
	Location begin = {0};
	Location end = {0};
	char *old = NULL;
	if (win == NULL || win.buffer == NULL) {
		return false;
	}
	line = win.buffer.line(win.cursor.line);
	if (line == NULL) {
		return false;
	}
	first = line.first_nonblank();
	old_off = win.cursor.offset;
	begin.line = win.cursor.line;
	begin.offset = 0;
	end.line = win.cursor.line;
	end.offset = first;
	if (first > 0) {
		old = malloc(first);
		if (old == NULL) {
			return false;
		}
		memcpy(old, ptr(line.data), first);
	}
	if (!win.set_text(begin, end, prefix, plen)) {
		free(old);
		return false;
	}
	if (first > 0) {
		buffer_undo.record_edit(UndoDelete, begin.line, begin.offset, old, first);
	}
	if (plen > 0) {
		buffer_undo.record_edit(UndoInsert, begin.line, begin.offset, prefix, plen);
	}
	free(old);
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

static bool set_line_indent_spaces(Window *win, int col) {
	char prefix[IndentPrefixCap] = {0};
	size_t n = 0;
	if (col < 0) {
		return false;
	}
	n = build_space_indent(col, prefix, sizeof(prefix));
	return replace_leading(win, prefix, n);
}

static bool set_line_indent_tabs(Window *win, int col) {
	char prefix[IndentPrefixCap] = {0};
	size_t n = 0;
	if (col < 0) {
		return false;
	}
	n = build_tab_indent(col, prefix, sizeof(prefix));
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
	char[..] p = {0};
	size_t n = 0;
	size_t i = 0;
	size_t save = 0;
	int num = 0;
	if (line == NULL || cap == 0) {
		return 0;
	}
	p = line.view();
	{
		char *pp = ptr(p);
		while (i < len(p) && (pp[i] == ' ' || pp[i] == '\t') && n < cap) {
			out[n++] = pp[i++];
		}
		while (i < len(p) && pp[i] == '>' && n + 2 <= cap) {
			out[n++] = '>';
			out[n++] = ' ';
			i++;
			if (i < len(p) && pp[i] == ' ') {
				i++;
			}
			while (i < len(p) && pp[i] == ' ') {
				i++;
			}
		}
		if (i < len(p) && (pp[i] == '-' || pp[i] == '*' || pp[i] == '+')) {
			if (i + 1 < len(p) && pp[i + 1] == ' ' && n + 2 <= cap) {
				out[n++] = pp[i];
				out[n++] = ' ';
				return n;
			}
		}
		save = i;
		num = 0;
		while (i < len(p) && pp[i] >= '0' && pp[i] <= '9') {
			num = num * 10 + (pp[i] - '0');
			i++;
		}
		if (i > save && i < len(p)) {
			char del = pp[i];
			if ((del == '.' || del == ')') && i + 1 < len(p) && pp[i + 1] == ' ') {
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
					for (j = 0; j < dlen / 2; j++) {
						char tmp = digits[j];
						digits[j] = digits[dlen - 1 - j];
						digits[dlen - 1 - j] = tmp;
					}
				}
				if (n + (size_t)dlen + 2 <= cap) {
					memcpy(out + n, digits, dlen);
					n += dlen;
					out[n++] = del;
					out[n++] = ' ';
					return n;
				}
			}
		}
	}
	return n;
}

static bool insert_md_prefix_after_newline(Window *win) {
	Buffer *buf = {0};
	Line *prev = {0};
	char prefix[IndentPrefixCap] = {0};
	size_t n = 0;
	Location at = {0};
	if (win == NULL || win.buffer == NULL || win.cursor.line <= 1) {
		return true;
	}
	buf = win.buffer;
	prev = buf.line(win.cursor.line - 1);
	if (prev == NULL) {
		return true;
	}
	n = md_build_prefix(prev, prefix, sizeof(prefix));
	if (n == 0) {
		return true;
	}
	at = win.cursor;
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
		int col = 0;
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
		col = mode_calc_indent(win.buffer, win.cursor.line);
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
	int col = 0;
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
		size_t plen = 0;
		if (ref == 0) {
			return true;
		}
		plen = md_build_prefix(win.buffer.line(ref), prefix, sizeof(prefix));
		buffer_undo.begin_command(win.buffer, win.cursor);
		replace_leading(win, prefix, plen);
		buffer_undo.end_command();
		return true;
	}
	col = mode_calc_indent(win.buffer, win.cursor.line);
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
	int col = 0;
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
		for (i = 0; i < n; i++) {
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
		col = mode_calc_indent(win.buffer, win.cursor.line);
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
		col = calc_indent_c(win.buffer, win.cursor.line);
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

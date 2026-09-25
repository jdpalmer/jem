// runtime/edit_extra.mc — quote, blank lines, transpose-lines, region case/sort, ident case.
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

static bool line_is_empty(Line *line) {
	return line == NULL || line.byte_len() == 0;
}

static char ident_lower(char c) {
	if (c >= 'A' && c <= 'Z') {
		return (char)(c + ('a' - 'A'));
	}
	return c;
}

static char ident_upper(char c) {
	if (c >= 'a' && c <= 'z') {
		return (char)(c - ('a' - 'A'));
	}
	return c;
}

static bool replace_range_undo(Window *w, Location begin, Location end, const char *text, size_t n) {
	char *old = NULL;
	size_t old_len = 0;
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	old = region_text(w, begin, end, &old_len);
	if (!w.set_text(begin, end, text != NULL ? text : "", n)) {
		free(old);
		return false;
	}
	if (old != NULL && old_len > 0) {
		buffer_undo.record_edit(UndoDelete, begin.line, begin.offset, old, old_len);
	}
	if (n > 0 && text != NULL) {
		buffer_undo.record_edit(UndoInsert, begin.line, begin.offset, text, n);
	}
	free(old);
	return true;
}

static bool replace_range_command(Window *w, Location begin, Location end, const char *text, size_t n) {
	bool ok = false;
	buffer_undo.begin_command(w.buffer, w.cursor);
	ok = replace_range_undo(w, begin, end, text, n);
	buffer_undo.end_command();
	return ok;
}

static bool quote_insert_key(uint32_t k, int n) {
	Window *w = window_current();
	char utf[8] = {0};
	int len = 0;
	Location at = {0};
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	if (n <= 0) {
		return true;
	}
	if (k == '\n' || k == '\r' || k == term_key_value(TermKeyEnter)) {
		char nl = '\n';
		at = w.cursor;
		buffer_undo.begin_command(w.buffer, at);
		while (n-- > 0) {
			Location cur = w.cursor;
			if (!w.set_text(w.cursor, w.cursor, &nl, 1)) {
				break;
			}
			buffer_undo.record_edit(UndoInsert, cur.line, cur.offset, &nl, 1);
		}
		buffer_undo.end_command();
		return true;
	}
	/* Strip modifier bits for literal insert of the base character. */
	{
		uint32_t base = k & ~term_key_value(TermKeyMask);
		if ((k & term_key_value(TermKeyMask)) != 0 && base < 128) {
			k = base;
		}
	}
	if (k < 128) {
		utf[0] = (char)k;
		len = 1;
	} else if (k < 0x800) {
		utf[0] = (char)(0xC0 | (k >> 6));
		utf[1] = (char)(0x80 | (k & 0x3F));
		len = 2;
	} else if (k < 0x10000) {
		utf[0] = (char)(0xE0 | (k >> 12));
		utf[1] = (char)(0x80 | ((k >> 6) & 0x3F));
		utf[2] = (char)(0x80 | (k & 0x3F));
		len = 3;
	} else if (k < 0x110000) {
		utf[0] = (char)(0xF0 | (k >> 18));
		utf[1] = (char)(0x80 | ((k >> 12) & 0x3F));
		utf[2] = (char)(0x80 | ((k >> 6) & 0x3F));
		utf[3] = (char)(0x80 | (k & 0x3F));
		len = 4;
	} else {
		return false;
	}
	at = w.cursor;
	buffer_undo.begin_command(w.buffer, at);
	while (n-- > 0) {
		Location cur = w.cursor;
		if (!w.set_text(w.cursor, w.cursor, utf, len)) {
			break;
		}
		buffer_undo.record_edit(UndoInsert, cur.line, cur.offset, utf, len);
	}
	buffer_undo.end_command();
	return true;
}

// cmd_quote reads the next key and inserts it literally n times (C-q).
bool cmd_quote(bool f, int n) {
	(void)f;
	if (n < 0) {
		return false;
	}
	push_listener(ListenerQuote);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.universal_n = n;
			L.universal_f = f;
		}
	}
	display_set_echo("C-q-");
	return true;
}

bool cmd_quote_handle_key(uint32_t key, int n) {
	display_set_echo("");
	return quote_insert_key(key, n);
}

// cmd_delete_blank_lines collapses blank-line runs around point (C-x C-o).
bool cmd_delete_blank_lines(bool f, int n) {
	Window *w = window_current();
	Buffer *buf = NULL;
	int cur = 0;
	Location begin = {0};
	Location end = {0};
	(void)f;
	(void)n;
	if (w == NULL || w.buffer == NULL) {
		return false;
	}
	buf = w.buffer;
	if (buf.is_readonly) {
		return false;
	}
	cur = w.cursor.line;
	if (cur > buf.line_count_i()) {
		return true;
	}
	if (line_is_empty(buf.line(cur))) {
		int start = cur;
		int end_line = cur;
		int run = 0;
		while (start > 1 && line_is_empty(buf.line(start - 1))) {
			start--;
		}
		while (end_line < buf.line_count_i() && line_is_empty(buf.line(end_line + 1))) {
			end_line++;
		}
		run = end_line - start + 1;
		if (run <= 1) {
			return true;
		}
		/* Keep `start`; delete lines start+1 .. end_line via [start+1,0) .. [end+1,0). */
		begin.line = start + 1;
		begin.offset = 0;
		end.line = end_line + 1;
		end.offset = 0;
		return replace_range_command(w, begin, end, "", 0);
	}
	{
		int line_num = cur;
		int nld = 0;
		while (line_num < buf.line_count_i() && line_is_empty(buf.line(line_num + 1))) {
			line_num++;
			nld++;
		}
		if (nld == 0) {
			return true;
		}
		begin.line = cur + 1;
		begin.offset = 0;
		end.line = cur + nld + 1;
		end.offset = 0;
		return replace_range_command(w, begin, end, "", 0);
	}
}

// cmd_transpose_lines swaps the current line with the one above (C-x C-t).
bool cmd_transpose_lines(bool f, int n) {
	Window *w = window_current();
	Buffer *buf = NULL;
	int curr = 0;
	Location p0 = {0};
	Location p2 = {0};
	Line *prev = NULL;
	char *original = NULL;
	size_t total = 0;
	size_t len1 = 0;
	char *swapped = NULL;
	Location saved = {0};
	(void)f;
	(void)n;
	if (w == NULL || w.buffer == NULL) {
		return false;
	}
	buf = w.buffer;
	if (buf.is_readonly) {
		return false;
	}
	curr = w.cursor.line;
	if (curr <= 1 || curr >= buf.eof_line()) {
		return false;
	}
	prev = buf.line(curr - 1);
	if (prev == NULL || buf.line(curr) == NULL) {
		return false;
	}
	p0.line = curr - 1;
	p0.offset = 0;
	p2.line = curr + 1;
	p2.offset = 0; /* may be EOF when curr is last line */
	original = region_text(w, p0, p2, &total);
	if (original == NULL) {
		return false;
	}
	len1 = prev.byte_len() + 1;
	if (total < len1) {
		free(original);
		return false;
	}
	swapped = malloc(total + 2);
	if (swapped == NULL) {
		free(original);
		return false;
	}
	if (curr < buf.line_count_i()) {
		/* original = prev\ncurr\n… */
		memcpy(swapped, original + len1, total - len1);
		memcpy(swapped + (total - len1), original, len1);
	} else {
		/* get_text to EOF is prev\ncurr (no nl after curr); insert separator. */
		size_t len2 = total - len1;
		memcpy(swapped, original + len1, len2);
		swapped[len2] = '\n';
		memcpy(swapped + len2 + 1, original, prev.byte_len());
		total = len2 + 1 + prev.byte_len();
	}
	saved = w.cursor;
	if (!replace_range_command(w, p0, p2, swapped, total)) {
		free(original);
		free(swapped);
		return false;
	}
	free(original);
	free(swapped);
	saved.line = curr - 1;
	saved.offset = 0;
	w.set_cursor(saved);
	w.did_move = true;
	return true;
}

static bool get_region(Window *w, Location *start_out, Location *end_out) {
	Location cursor = {0};
	Location mark = {0};
	if (w == NULL || w.mark.line == 0) {
		display_set_echo("No mark set");
		return false;
	}
	cursor = w.cursor;
	mark = w.mark;
	if (cursor.line < mark.line || (cursor.line == mark.line && cursor.offset <= mark.offset)) {
		*start_out = cursor;
		*end_out = mark;
	} else {
		*start_out = mark;
		*end_out = cursor;
	}
	return true;
}

static bool transform_region_case(bool upper) {
	Window *w = window_current();
	Location start = {0};
	Location end = {0};
	char *text = NULL;
	size_t tlen = 0;
	size_t i = 0;
	bool changed = false;
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	if (!get_region(w, &start, &end)) {
		return false;
	}
	text = region_text(w, start, end, &tlen);
	if (text == NULL) {
		return false;
	}
	for (i = 0; i < tlen; i++) {
		char t = 0;
		if (upper) {
			t = ident_upper(text[i]);
		} else {
			t = ident_lower(text[i]);
		}
		if (t != text[i]) {
			text[i] = t;
			changed = true;
		}
	}
	if (!changed) {
		free(text);
		return true;
	}
	if (!replace_range_command(w, start, end, text, tlen)) {
		free(text);
		return false;
	}
	free(text);
	return true;
}

bool cmd_lower_region(bool f, int n) {
	(void)f;
	(void)n;
	return transform_region_case(false);
}

bool cmd_upper_region(bool f, int n) {
	(void)f;
	(void)n;
	return transform_region_case(true);
}

int cmp_cstr_ptr(void *a, void *b) {
	char *sa = *(char **)a;
	char *sb = *(char **)b;
	return strcmp(sa, sb);
}

// cmd_sort_region sorts complete lines in the active region alphabetically.
bool cmd_sort_region(bool f, int n) {
	Window *w = window_current();
	Buffer *buf = NULL;
	Location rstart = {0};
	Location rend = {0};
	Location start = {0};
	Location end = {0};
	char *text = NULL;
	size_t tlen = 0;
	char **lines = NULL;
	int nlines = 0;
	int cap = 0;
	size_t i = 0;
	size_t line_start = 0;
	char *sorted = NULL;
	size_t sorted_len = 0;
	size_t sorted_cap = 0;
	int li = 0;
	Location saved = {0};
	(void)f;
	(void)n;
	if (w == NULL || w.buffer == NULL) {
		return false;
	}
	buf = w.buffer;
	if (buf.is_readonly) {
		return false;
	}
	if (!get_region(w, &rstart, &rend)) {
		return false;
	}
	if (rend.line - rstart.line + 1 < 2) {
		display_set_echo("Sort needs at least 2 lines");
		return false;
	}
	start.line = rstart.line;
	start.offset = 0;
	end.line = rend.line + 1;
	end.offset = 0; /* may be EOF */
	text = region_text(w, start, end, &tlen);
	if (text == NULL) {
		return false;
	}
	/* Split on newlines into lines[]. */
	line_start = 0;
	for (i = 0; i <= tlen; i++) {
		if (i == tlen || text[i] == '\n') {
			size_t llen = i - line_start;
			char *one = NULL;
			/* Drop trailing empty segment from final newline. */
			if (i == tlen && llen == 0 && nlines > 0) {
				break;
			}
			if (nlines >= cap) {
				int ncap = cap != 0 ? cap * 2 : 8;
				char **p = realloc(lines, (size_t)ncap * sizeof(char *));
				if (p == NULL) {
					free(text);
					for (li = 0; li < nlines; li++) {
						free(lines[li]);
					}
					free(lines);
					return false;
				}
				lines = p;
				cap = ncap;
			}
			one = malloc(llen + 1);
			if (one == NULL) {
				free(text);
				for (li = 0; li < nlines; li++) {
					free(lines[li]);
				}
				free(lines);
				return false;
			}
			if (llen > 0) {
				memcpy(one, text + line_start, llen);
			}
			one[llen] = 0;
			lines[nlines++] = one;
			line_start = i + 1;
		}
	}
	free(text);
	if (nlines < 2) {
		for (li = 0; li < nlines; li++) {
			free(lines[li]);
		}
		free(lines);
		display_set_echo("Sort needs at least 2 lines");
		return false;
	}
	qsort(lines, nlines, sizeof(char *), cmp_cstr_ptr);
	sorted_cap = tlen + (size_t)nlines + 8;
	sorted = malloc(sorted_cap);
	if (sorted == NULL) {
		for (li = 0; li < nlines; li++) {
			free(lines[li]);
		}
		free(lines);
		return false;
	}
	for (li = 0; li < nlines; li++) {
		size_t llen = strlen(lines[li]);
		if (sorted_len + llen + 2 > sorted_cap) {
			size_t ncap = sorted_cap * 2 + llen + 2;
			char *p = realloc(sorted, ncap);
			if (p == NULL) {
				free(sorted);
				for (li = 0; li < nlines; li++) {
					free(lines[li]);
				}
				free(lines);
				return false;
			}
			sorted = p;
			sorted_cap = ncap;
		}
		memcpy(sorted + sorted_len, lines[li], llen);
		sorted_len += llen;
		sorted[sorted_len++] = '\n';
	}
	for (li = 0; li < nlines; li++) {
		free(lines[li]);
	}
	free(lines);
	saved = w.cursor;
	if (!replace_range_command(w, start, end, sorted, sorted_len)) {
		free(sorted);
		return false;
	}
	free(sorted);
	w.set_cursor(saved);
	w.mark.line = 0;
	w.mark.offset = 0;
	display_set_echo("Region sorted");
	return true;
}

/* --- identifier case conversion --- */

enum IdentCaseStyle {
	IdentCamel = 0,
	IdentPascal = 1,
	IdentSnake = 2,
	IdentConstant = 3
};

static bool ident_word_byte(char b) {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_';
}

static bool ident_bounds(Buffer *buf, Location loc, Location *start_out, Location *end_out) {
	Line *line = NULL;
	char[..] v = {0};
	char *vp = NULL;
	size_t off = 0;
	size_t start_off = 0;
	size_t end_off = 0;
	line = buf.line(loc.line);
	if (line == NULL) {
		return false;
	}
	v = line.view();
	vp = ptr(v);
	off = loc.offset;
	if (off > len(v)) {
		off = len(v);
	}
	if (off > 0 && (off >= len(v) || !ident_word_byte(vp[off])) && ident_word_byte(vp[off - 1])) {
		off--;
	}
	if (off >= len(v) || !ident_word_byte(vp[off])) {
		return false;
	}
	start_off = off;
	while (start_off > 0 && ident_word_byte(vp[start_off - 1])) {
		start_off--;
	}
	end_off = off + 1;
	while (end_off < len(v) && ident_word_byte(vp[end_off])) {
		end_off++;
	}
	start_out.line = loc.line;
	start_out.offset = start_off;
	end_out.line = loc.line;
	end_out.offset = end_off;
	return true;
}

static bool is_upper_letter(char c) {
	return c >= 'A' && c <= 'Z';
}

static bool is_lower_letter(char c) {
	return c >= 'a' && c <= 'z';
}

static bool is_digit_c(char c) {
	return c >= '0' && c <= '9';
}

/* Split into lowercased parts; parts stored in blob, pointers in outs. */
static int split_ident_parts(const char *s, size_t n, char *blob, size_t blob_cap,
	char **outs, int max_outs, size_t *blob_used) {
	size_t i = 0;
	char cur[256] = {0};
	size_t cur_len = 0;
	int count = 0;
	size_t used = 0;
	*blob_used = 0;

	for (i = 0; i < n; i++) {
		char r = s[i];
		if (r == '_' || r == '-') {
			if (cur_len > 0) {
				size_t j = 0;
				if (count >= max_outs || used + cur_len + 1 > blob_cap) {
					return -1;
				}
				for (j = 0; j < cur_len; j++) {
					blob[used + j] = ident_lower(cur[j]);
				}
				blob[used + cur_len] = 0;
				outs[count++] = blob + used;
				used = used + cur_len + 1;
				cur_len = 0;
			}
			continue;
		}
		if (i > 0 && is_upper_letter(r)) {
			char prev = s[i - 1];
			bool next_lower = (i + 1 < n) && is_lower_letter(s[i + 1]);
			if (is_lower_letter(prev) || is_digit_c(prev) || (is_upper_letter(prev) && next_lower)) {
				if (cur_len > 0) {
					size_t j = 0;
					if (count >= max_outs || used + cur_len + 1 > blob_cap) {
						return -1;
					}
					for (j = 0; j < cur_len; j++) {
						blob[used + j] = ident_lower(cur[j]);
					}
					blob[used + cur_len] = 0;
					outs[count++] = blob + used;
					used = used + cur_len + 1;
					cur_len = 0;
				}
			}
		}
		if (cur_len + 1 >= sizeof(cur)) {
			return -1;
		}
		cur[cur_len++] = r;
	}
	if (cur_len > 0) {
		size_t j = 0;
		if (count >= max_outs || used + cur_len + 1 > blob_cap) {
			return -1;
		}
		for (j = 0; j < cur_len; j++) {
			blob[used + j] = ident_lower(cur[j]);
		}
		blob[used + cur_len] = 0;
		outs[count++] = blob + used;
		used = used + cur_len + 1;
	}
	*blob_used = used;
	return count;
}

static void title_part(char *dst, const char *src) {
	size_t i = 0;
	size_t n = strlen(src);
	if (n == 0) {
		dst[0] = 0;
		return;
	}
	dst[0] = ident_upper(src[0]);
	for (i = 1; i < n; i++) {
		dst[i] = ident_lower(src[i]);
	}
	dst[n] = 0;
}

static bool join_ident_parts(char **parts, int nparts, IdentCaseStyle style, char *out, size_t out_cap) {
	size_t len = 0;
	int i = 0;
	char titled[256] = {0};
	out[0] = 0;
	if (nparts == 0) {
		return true;
	}
	if (style == IdentCamel || style == IdentPascal) {
		for (i = 0; i < nparts; i++) {
			if (style == IdentCamel && i == 0) {
				size_t plen = strlen(parts[i]);
				size_t j = 0;
				if (len + plen + 1 > out_cap) {
					return false;
				}
				for (j = 0; j < plen; j++) {
					out[len + j] = ident_lower(parts[i][j]);
				}
				len += plen;
			} else {
				title_part(titled, parts[i]);
				{
					size_t plen = strlen(titled);
					if (len + plen + 1 > out_cap) {
						return false;
					}
					memcpy(out + len, titled, plen);
					len += plen;
				}
			}
		}
		out[len] = 0;
		return true;
	}
	/* snake / constant */
	for (i = 0; i < nparts; i++) {
		size_t plen = strlen(parts[i]);
		size_t j = 0;
		if (i > 0) {
			if (len + 2 > out_cap) {
				return false;
			}
			out[len++] = '_';
		}
		if (len + plen + 1 > out_cap) {
			return false;
		}
		for (j = 0; j < plen; j++) {
			char c = parts[i][j];
			if (style == IdentConstant) {
				out[len + j] = ident_upper(c);
			} else {
				out[len + j] = ident_lower(c);
			}
		}
		len += plen;
	}
	out[len] = 0;
	return true;
}

static bool convert_ident_at_point(IdentCaseStyle style) {
	Window *w = window_current();
	Buffer *buf = NULL;
	Location start = {0};
	Location end = {0};
	char *text = NULL;
	size_t tlen = 0;
	char blob[1024] = {0};
	char *parts[64] = {0};
	size_t blob_used = 0;
	int nparts = 0;
	char out[1024] = {0};
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	buf = w.buffer;
	if (!ident_bounds(buf, w.cursor, &start, &end)) {
		return false;
	}
	text = region_text(w, start, end, &tlen);
	if (text == NULL || tlen == 0) {
		free(text);
		return false;
	}
	nparts = split_ident_parts(text, tlen, blob, sizeof(blob), parts, 64, &blob_used);
	if (nparts < 0) {
		free(text);
		return false;
	}
	if (nparts == 0) {
		free(text);
		return true;
	}
	if (!join_ident_parts(parts, nparts, style, out, sizeof(out))) {
		free(text);
		return false;
	}
	if (strcmp(out, text) == 0) {
		free(text);
		return true;
	}
	if (!replace_range_command(w, start, end, out, strlen(out))) {
		free(text);
		return false;
	}
	free(text);
	return true;
}

bool cmd_camel_case(bool f, int n) {
	(void)f;
	(void)n;
	return convert_ident_at_point(IdentCamel);
}

bool cmd_pascal_case(bool f, int n) {
	(void)f;
	(void)n;
	return convert_ident_at_point(IdentPascal);
}

bool cmd_snake_case(bool f, int n) {
	(void)f;
	(void)n;
	return convert_ident_at_point(IdentSnake);
}

bool cmd_constant_case(bool f, int n) {
	(void)f;
	(void)n;
	return convert_ident_at_point(IdentConstant);
}

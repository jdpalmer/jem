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
	size_t old_len = 0;
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	char *old = region_text(w, begin, end, &old_len);
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
	buffer_undo.begin_command(w.buffer, w.cursor);
	bool ok = replace_range_undo(w, begin, end, text, n);
	buffer_undo.end_command();
	return ok;
}

static bool quote_insert_key(uint32_t k, int n) {
	Window *w = window_current();
	char utf[8] = {0};
	int len = 0;
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	if (n <= 0) {
		return true;
	}
	if (k == '\n' || k == '\r' || k == term_key_value(TermKeyEnter)) {
		char nl = '\n';
		Location at = w.cursor;
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
	Location at = w.cursor;
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
	(void)f;
	(void)n;
	if (w == NULL || w.buffer == NULL) {
		return false;
	}
	Buffer *buf = w.buffer;
	if (buf.is_readonly) {
		return false;
	}
	int cur = w.cursor.line;
	if (cur > buf.line_count_i()) {
		return true;
	}
	if (line_is_empty(buf.line(cur))) {
		int start = cur;
		int end_line = cur;
		while (start > 1 && line_is_empty(buf.line(start - 1))) {
			start--;
		}
		while (end_line < buf.line_count_i() && line_is_empty(buf.line(end_line + 1))) {
			end_line++;
		}
		int run = end_line - start + 1;
		if (run <= 1) {
			return true;
		}
		/* Keep `start`; delete lines start+1 .. end_line via [start+1,0) .. [end+1,0). */
		Location begin = { .line = start + 1, .offset = 0 };
		Location end = { .line = end_line + 1, .offset = 0 };
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
		Location begin = { .line = cur + 1, .offset = 0 };
		Location end = { .line = cur + nld + 1, .offset = 0 };
		return replace_range_command(w, begin, end, "", 0);
	}
}

// cmd_transpose_lines swaps the current line with the one above (C-x C-t).
bool cmd_transpose_lines(bool f, int n) {
	Window *w = window_current();
	(void)f;
	(void)n;
	if (w == NULL || w.buffer == NULL) {
		return false;
	}
	Buffer *buf = w.buffer;
	if (buf.is_readonly) {
		return false;
	}
	int curr = w.cursor.line;
	if (curr <= 1 || curr >= buf.eof_line()) {
		return false;
	}
	Line *prev = buf.line(curr - 1);
	if (prev == NULL || buf.line(curr) == NULL) {
		return false;
	}
	Location p0 = { .line = curr - 1, .offset = 0 };
	Location p2 = { .line = curr + 1, .offset = 0 }; /* offset 0 may be EOF */
	pool p {
		size_t total = 0;
		char *original = region_text(w, p0, p2, &total);
		if (original == NULL) {
			return false;
		}
		size_t len1 = prev.byte_len() + 1;
		if (total < len1) {
			free(original);
			return false;
		}
		char[..] orig = {0};
		if (!p.copy(&orig, ranged(original, total))) {
			free(original);
			return false;
		}
		free(original);

		char[..] swapped = {0};
		if (curr < buf.line_count_i()) {
			/* original = prev\ncurr\n… */
			if (!p.append(&swapped, orig[len1..])) {
				return false;
			}
			if (!p.append(&swapped, orig[..len1])) {
				return false;
			}
		} else {
			/* get_text to EOF is prev\ncurr (no nl after curr); insert separator. */
			if (!p.append(&swapped, orig[len1..])) {
				return false;
			}
			if (!p.push(&swapped, '\n')) {
				return false;
			}
			if (!p.append(&swapped, orig[..prev.byte_len()])) {
				return false;
			}
		}
		if (!replace_range_command(w, p0, p2, ptr(swapped), len(swapped))) {
			return false;
		}
		Location saved = { .line = curr - 1, .offset = 0 };
		w.set_cursor(saved);
		w.did_move = true;
		return true;
	}
}

static bool get_region(Window *w, Location *start_out, Location *end_out) {
	if (w == NULL || w.mark.line == 0) {
		display_set_echo("No mark set");
		return false;
	}
	Location cursor = w.cursor;
	Location mark = w.mark;
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
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	if (!get_region(w, &start, &end)) {
		return false;
	}
	pool p {
		size_t tlen = 0;
		size_t i = 0;
		bool changed = false;
		char *text = region_text(w, start, end, &tlen);
		if (text == NULL) {
			return false;
		}
		char[..] tv = {0};
		if (!p.copy(&tv, ranged(text, tlen))) {
			free(text);
			return false;
		}
		free(text);
		for (; i < len(tv); i++) {
			char t = 0;
			if (upper) {
				t = ident_upper(tv[i]);
			} else {
				t = ident_lower(tv[i]);
			}
			if (t != tv[i]) {
				tv[i] = t;
				changed = true;
			}
		}
		if (!changed) {
			return true;
		}
		if (!replace_range_command(w, start, end, ptr(tv), len(tv))) {
			return false;
		}
		return true;
	}
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

static int cmp_char_slice(void *a, void *b) {
	const char[..] *sa = (const char[..] *)a;
	const char[..] *sb = (const char[..] *)b;
	return str_cmp(*sa, *sb);
}

// cmd_sort_region sorts complete lines in the active region alphabetically.
bool cmd_sort_region(bool f, int n) {
	Window *w = window_current();
	Location rstart = {0};
	Location rend = {0};
	(void)f;
	(void)n;
	if (w == NULL || w.buffer == NULL) {
		return false;
	}
	Buffer *buf = w.buffer;
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
	Location start = { .line = rstart.line, .offset = 0 };
	Location end = { .line = rend.line + 1, .offset = 0 }; /* may be EOF */
	pool p {
		size_t tlen = 0;
		char *text = region_text(w, start, end, &tlen);
		if (text == NULL) {
			return false;
		}
		/* Copy region into the pool, then drop the heap buffer. */
		char[..] tv = {0};
		if (!p.copy(&tv, ranged(text, tlen))) {
			free(text);
			return false;
		}
		free(text);

		/* Line views into tv; headers + sorted bytes live in the pool. */
		char[..][..] lines = {0};
		size_t i = 0;
		size_t line_start = 0;
		for (; i <= tlen; i++) {
			if (i == tlen || tv[i] == '\n') {
				size_t llen = i - line_start;
				/* Drop trailing empty segment from final newline. */
				if (i == tlen && llen == 0 && len(lines) > 0) {
					break;
				}
				if (!p.push(&lines, tv[line_start..i])) {
					return false;
				}
				line_start = i + 1;
			}
		}
		if (len(lines) < 2) {
			display_set_echo("Sort needs at least 2 lines");
			return false;
		}
		qsort(ptr(lines), len(lines), sizeof(lines[0]), cmp_char_slice);

		char[..] sorted = {0};
		for (auto line : lines) {
			if (!p.append(&sorted, line)) {
				return false;
			}
			if (!p.push(&sorted, '\n')) {
				return false;
			}
		}

		Location saved = w.cursor;
		if (!replace_range_command(w, start, end, ptr(sorted), len(sorted))) {
			return false;
		}
		w.set_cursor(saved);
		w.mark.line = 0;
		w.mark.offset = 0;
		display_set_echo("Region sorted");
		return true;
	}
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
	Line *line = buf.line(loc.line);
	if (line == NULL) {
		return false;
	}
	char[..] v = line.view();
	const char[..] vp = v;
	size_t off = loc.offset;
	if (off > len(v)) {
		off = len(v);
	}
	if (off > 0 && (off >= len(v) || !ident_word_byte(vp[off])) && ident_word_byte(vp[off - 1])) {
		off--;
	}
	if (off >= len(v) || !ident_word_byte(vp[off])) {
		return false;
	}
	size_t start_off = off;
	while (start_off > 0 && ident_word_byte(vp[start_off - 1])) {
		start_off--;
	}
	size_t end_off = off + 1;
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
	const char[..] sv = ranged(s, n);
	char[..] bv = ranged(blob, blob_cap);
	char *[..] ov = ranged(outs, (size_t)max_outs);
	*blob_used = 0;

	for (; i < n; i++) {
		char r = sv[i];
		if (r == '_' || r == '-') {
			if (cur_len > 0) {
				size_t j = 0;
				if (count >= max_outs || used + cur_len + 1 > blob_cap) {
					return -1;
				}
				for (j = 0; j < cur_len; j++) {
					bv[used + j] = ident_lower(cur[j]);
				}
				bv[used + cur_len] = 0;
				ov[count++] = blob + used;
				used = used + cur_len + 1;
				cur_len = 0;
			}
			continue;
		}
		if (i > 0 && is_upper_letter(r)) {
			char prev = sv[i - 1];
			bool next_lower = (i + 1 < n) && is_lower_letter(sv[i + 1]);
			if (is_lower_letter(prev) || is_digit_c(prev) || (is_upper_letter(prev) && next_lower)) {
				if (cur_len > 0) {
					size_t j = 0;
					if (count >= max_outs || used + cur_len + 1 > blob_cap) {
						return -1;
					}
					for (j = 0; j < cur_len; j++) {
						bv[used + j] = ident_lower(cur[j]);
					}
					bv[used + cur_len] = 0;
					ov[count++] = blob + used;
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
		for (; j < cur_len; j++) {
			bv[used + j] = ident_lower(cur[j]);
		}
		bv[used + cur_len] = 0;
		ov[count++] = blob + used;
		used = used + cur_len + 1;
	}
	*blob_used = used;
	return count;
}

static void title_part(char *dst, const char *src) {
	size_t i = 1;
	size_t n = strlen(src);
	char[..] dv = ranged(dst, n + 1);
	const char[..] sv = ranged(src, n + 1);
	if (n == 0) {
		dv[0] = 0;
		return;
	}
	dv[0] = ident_upper(sv[0]);
	for (; i < n; i++) {
		dv[i] = ident_lower(sv[i]);
	}
	dv[n] = 0;
}

static bool join_ident_parts(char **parts, int nparts, IdentCaseStyle style, char *out, size_t out_cap) {
	size_t len = 0;
	int i = 0;
	char titled[256] = {0};
	char[..] ov = ranged(out, out_cap);
	char *[..] pv = ranged(parts, (size_t)nparts);
	ov[0] = 0;
	if (nparts == 0) {
		return true;
	}
	if (style == IdentCamel || style == IdentPascal) {
		for (; i < nparts; i++) {
			if (style == IdentCamel && i == 0) {
				size_t plen = strlen(pv[i]);
				const char[..] pi = ranged(pv[i], plen);
				size_t j = 0;
				if (len + plen + 1 > out_cap) {
					return false;
				}
				for (j = 0; j < plen; j++) {
					ov[len + j] = ident_lower(pi[j]);
				}
				len += plen;
			} else {
				title_part(titled, pv[i]);
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
		ov[len] = 0;
		return true;
	}
	/* snake / constant */
	for (i = 0; i < nparts; i++) {
		size_t plen = strlen(pv[i]);
		const char[..] pi = ranged(pv[i], plen);
		size_t j = 0;
		if (i > 0) {
			if (len + 2 > out_cap) {
				return false;
			}
			ov[len++] = '_';
		}
		if (len + plen + 1 > out_cap) {
			return false;
		}
		for (j = 0; j < plen; j++) {
			char c = pi[j];
			if (style == IdentConstant) {
				ov[len + j] = ident_upper(c);
			} else {
				ov[len + j] = ident_lower(c);
			}
		}
		len += plen;
	}
	ov[len] = 0;
	return true;
}

static bool convert_ident_at_point(IdentCaseStyle style) {
	Window *w = window_current();
	Location start = {0};
	Location end = {0};
	size_t tlen = 0;
	char blob[1024] = {0};
	char *parts[64] = {0};
	size_t blob_used = 0;
	char out[1024] = {0};
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	Buffer *buf = w.buffer;
	if (!ident_bounds(buf, w.cursor, &start, &end)) {
		return false;
	}
	char *text = region_text(w, start, end, &tlen);
	if (text == NULL || tlen == 0) {
		free(text);
		return false;
	}
	int nparts = split_ident_parts(text, tlen, blob, sizeof(blob), parts, 64, &blob_used);
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

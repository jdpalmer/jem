// search — incremental search and query-replace (literal + regex).
import "buffer";
import "window";
import "regex";

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>
#include <string.h>
#include <stdlib.h>
#include <stdio.h>
#include <ctype.h>
#include <sys/types.h>

struct SearchState {
	bool active;
	bool forward;
	bool regexp;
	bool replace_mode;
	char needle[256];
	size_t needle_len;
	char replacement[256];
	size_t replacement_len;
	Location start;
	Location match_begin;
	Location match_end;
	bool have_match;
	bool wrap;
	ReCapture captures[ReMaxCaptures];
	char *re_slice;
	Location re_slice_start;
	char last_error[80];
};

enum SearchScopeMode {
	SearchScopeBuffer = 0,
	SearchScopeAllBuffers = 1
};

static SearchState g_search;
static int g_search_scope = SearchScopeBuffer;
static char g_last_pattern[256];

static void search_clear_re_slice() {
	free(g_search.re_slice);
	g_search.re_slice = NULL;
	g_search.re_slice_start.line = 0;
	g_search.re_slice_start.offset = 0;
}

static void search_clear_error() {
	g_search.last_error[0] = 0;
}

static void search_set_error(char *msg) {
	if (msg == NULL) {
		search_clear_error();
		return;
	}
	snprintf(g_search.last_error, sizeof(g_search.last_error), "%s", msg);
}

void search_reset() {
	search_clear_re_slice();
	memset(&g_search, 0, sizeof(g_search));
	g_search.forward = true;
}

/* Sticky isearch origin remaps; current match / spans are ephemeral → clear. */
void search_after_buffer_edit(Buffer *buf, Location begin, Location end, Location new_end) {
	Window *win = window_current();
	if (!g_search.active || win == NULL || win.buffer != buf) {
		return;
	}
	location_remap_replace(&g_search.start, begin, end, new_end);
	buffer_clamp_location(buf, &g_search.start);
	g_search.have_match = false;
	g_search.match_begin.line = 0;
	g_search.match_end.line = 0;
	search_clear_re_slice();
	window_clear_search_spans(win);
}

SearchState *search_state() {
	return &g_search;
}

bool search_active() {
	return g_search.active;
}

bool search_regexp() {
	return g_search.regexp;
}

char *search_last_error() {
	return g_search.last_error;
}

static bool line_match_at(Line *line, size_t offset, char *needle, size_t nlen) {
	size_t i = 0;
	if (line == NULL || needle == NULL || nlen == 0) {
		return false;
	}
	char[..] v = line.view();
	if (offset + nlen > len(v)) {
		return false;
	}
	{
		const char[..] nv = ranged(needle, nlen);
		for (; i < nlen; i++) {
			if (v[offset + i] != nv[i]) {
				return false;
			}
		}
	}
	return true;
}

static bool search_from(Buffer *buf, Location from, bool forward, char *needle, size_t nlen, Location *begin, Location *end) {
	if (buf == NULL || nlen == 0) {
		return false;
	}
	int nlines = buf.line_count_i();
	if (forward) {
		int line_n = from.line;
		while (line_n <= nlines) {
			Line *line = buf.line(line_n);
			size_t off = 0;
			size_t lim = 0;
			if (line_n == from.line) {
				off = from.offset;
			}
			if (line != NULL) {
				lim = line.byte_len();
			}
			while (nlen <= lim && off <= lim - nlen) {
				if (line_match_at(line, off, needle, nlen)) {
					begin.line = line_n;
					begin.offset = off;
					end.line = line_n;
					end.offset = off + nlen;
					return true;
				}
				off++;
			}
			line_n++;
		}
	} else {
		int line_n = from.line;
		while (line_n >= 1) {
			Line *line = buf.line(line_n);
			size_t lim = 0;
			size_t off = 0;
			if (line != NULL) {
				lim = line.byte_len();
			}
			off = lim;
			if (line_n == from.line) {
				off = from.offset;
			}
			if (off > lim) {
				off = lim;
			}
			if (nlen <= off) {
				off = off - nlen;
				for (;;) {
					if (line_match_at(line, off, needle, nlen)) {
						begin.line = line_n;
						begin.offset = off;
						end.line = line_n;
						end.offset = off + nlen;
						return true;
					}
					if (off == 0) {
						break;
					}
					off--;
				}
			}
			line_n--;
		}
	}
	return false;
}

static int location_compare(Location a, Location b) {
	if (a.line == b.line) {
		if (a.offset < b.offset) {
			return -1;
		}
		if (a.offset > b.offset) {
			return 1;
		}
		return 0;
	}
	if (a.line < b.line) {
		return -1;
	}
	return 1;
}

static size_t bytes_to_buffer_end(Buffer *buf, Location start) {
	int line_number = start.line;
	size_t offset = start.offset;
	size_t size = 0;
	if (buf == NULL || line_number < 1) {
		return 0;
	}
	int nlines = buf.line_count_i();
	while (line_number <= nlines) {
		Line *lp = buf.line(line_number);
		size_t line_len = 0;
		if (lp != NULL) {
			line_len = lp.byte_len();
		}
		if (offset > line_len) {
			offset = line_len;
		}
		size = size + (line_len - offset) + 1;
		line_number++;
		offset = 0;
	}
	return size;
}

static char *buffer_slice_from(Buffer *buf, Location start, size_t *out_len) {
	size_t used = 0;
	if (buf == NULL || out_len == NULL) {
		return NULL;
	}
	size_t size = bytes_to_buffer_end(buf, start);
	char *text = malloc(size + 1);
	if (text == NULL) {
		return NULL;
	}
	int line_number = start.line;
	size_t offset = start.offset;
	int nlines = buf.line_count_i();
	while (line_number <= nlines) {
		Line *lp = buf.line(line_number);
		size_t line_len = 0;
		size_t chunk = 0;
		if (lp != NULL) {
			line_len = lp.byte_len();
		}
		if (offset > line_len) {
			offset = line_len;
		}
		chunk = line_len - offset;
		if (chunk > 0 && lp != NULL) {
			memcpy(text + used, ptr(lp.data) + offset, chunk);
			used = used + chunk;
		}
		{
			char[..] tv = ranged(text, used + 1, size + 1);
			tv[used] = '\n';
		}
		used++;
		line_number++;
		offset = 0;
	}
	{
		char[..] tv = ranged(text, used + 1, size + 1);
		tv[used] = 0;
	}
	*out_len = used;
	return text;
}

static Location location_advance_bytes(Buffer *buf, Location loc, size_t bytes) {
	if (buf == NULL) {
		return loc;
	}
	int nlines = buf.line_count_i();
	while (bytes > 0) {
		Line *lp = NULL;
		size_t line_len = 0;
		if (loc.line > nlines) {
			break;
		}
		lp = buf.line(loc.line);
		if (lp != NULL) {
			line_len = lp.byte_len();
		}
		if (loc.offset < line_len) {
			size_t step = bytes;
			size_t remain = line_len - loc.offset;
			if (step > remain) {
				step = remain;
			}
			loc.offset = loc.offset + step;
			bytes = bytes - step;
		} else {
			loc.line++;
			loc.offset = 0;
			bytes--;
		}
	}
	return loc;
}

static void search_store_captures(ReCapture[..] src) {
	size_t i = 0;
	size_t n = len(src);
	if (n > ReMaxCaptures) {
		n = ReMaxCaptures;
	}
	for (; i < n; i++) {
		g_search.captures[i] = src[i];
	}
}

/* 1 = found, 0 = none, -1 = error (invalid / zero-length). */
static int find_next_regex_match_from(Buffer *buf, Location search_start, char *pattern,
	Location *begin, Location *end) {
	size_t text_len = 0;
	ReCapture caps[ReMaxCaptures] = {0};
	size_t i = 0;

	search_clear_error();
	if (buf == NULL || pattern == NULL || begin == NULL || end == NULL) {
		return -1;
	}
	for (; i < ReMaxCaptures; i++) {
		caps[i].start = -1;
		caps[i].length = -1;
	}
	char *text = buffer_slice_from(buf, search_start, &text_len);
	if (text == NULL) {
		search_set_error((char *)(void *)"[out of memory]");
		return -1;
	}
	auto (st, match_offset, match_length) = re_find_groups(ranged(pattern, strlen(pattern)),
		ranged(text, text_len), ranged(caps, ReMaxCaptures));
	ReStatus status = st;
	if (status != ReFound) {
		free(text);
		if (status == ReInvalid) {
			search_set_error((char *)(void *)"[invalid regular expression]");
			return -1;
		}
		return 0;
	}
	if (match_length == 0) {
		free(text);
		search_set_error((char *)(void *)"[zero-length regex matches not supported]");
		return -1;
	}
	*begin = location_advance_bytes(buf, search_start, match_offset);
	*end = location_advance_bytes(buf, search_start, match_offset + match_length);
	search_clear_re_slice();
	g_search.re_slice = text;
	g_search.re_slice_start = search_start;
	search_store_captures(caps);
	(void)text_len;
	return 1;
}

static int find_prev_regex_match_from(Buffer *buf, Location limit, char *pattern,
	Location *begin, Location *end) {
	Location best_begin = {0};
	Location best_end = {0};
	ReCapture best_caps[ReMaxCaptures] = {0};
	char *best_text = NULL;
	Location best_slice_start = {0};
	bool have_best = false;
	size_t i = 0;

	if (buf == NULL || pattern == NULL) {
		return -1;
	}
	Location scan = { .line = 1, .offset = 0 };
	for (; i < ReMaxCaptures; i++) {
		best_caps[i].start = -1;
		best_caps[i].length = -1;
	}

	for (;;) {
		Location cand_begin = {0};
		Location cand_end = {0};
		int found = 0;
		found = find_next_regex_match_from(buf, scan, pattern, &cand_begin, &cand_end);
		if (found < 0) {
			free(best_text);
			search_clear_re_slice();
			return -1;
		}
		if (found == 0) {
			break;
		}
		if (location_compare(cand_end, limit) > 0) {
			search_clear_re_slice();
			break;
		}
		free(best_text);
		best_text = g_search.re_slice;
		g_search.re_slice = NULL;
		best_slice_start = g_search.re_slice_start;
		best_begin = cand_begin;
		best_end = cand_end;
		for (i = 0; i < ReMaxCaptures; i++) {
			best_caps[i] = g_search.captures[i];
		}
		have_best = true;
		scan = location_advance_bytes(buf, cand_begin, 1);
	}

	if (!have_best) {
		return 0;
	}
	search_clear_re_slice();
	g_search.re_slice = best_text;
	g_search.re_slice_start = best_slice_start;
	search_store_captures(best_caps);
	*begin = best_begin;
	*end = best_end;
	return 1;
}

static int search_regex_from(Buffer *buf, Location from, bool forward, char *pattern,
	Location *begin, Location *end) {
	if (forward) {
		return find_next_regex_match_from(buf, from, pattern, begin, end);
	}
	return find_prev_regex_match_from(buf, from, pattern, begin, end);
}

void search_begin(bool forward, bool regexp) {
	Window *win = window_current();
	search_reset();
	g_search.active = true;
	g_search.forward = forward;
	g_search.regexp = regexp;
	if (win != NULL) {
		g_search.start = win.cursor;
	}
}

void search_end() {
	Window *win = window_current();
	g_search.active = false;
	g_search.have_match = false;
	search_clear_re_slice();
	if (win != NULL) {
		window_clear_search_spans(win);
		if (win.mark.line != 0) {
			win.should_redraw = true;
		}
		win.mark.line = 0;
		win.mark.offset = 0;
	}
}

/* Current match uses Mark↔Cursor (selection paint); other hits use SearchSpans. */
static void search_highlight_match(Window *win, Location begin, Location end) {
	if (win == NULL) {
		return;
	}
	if (g_search.forward) {
		win.mark = begin;
	} else {
		win.mark = end;
	}
	win.should_redraw = true;
}

static void search_collect_visible_spans_literal(Window *win) {
	SearchSpan spans[SearchSpanMax] = {0};
	int n = 0;
	int top = 0;
	int line_n = top;
	if (win == NULL || win.buffer == NULL || g_search.needle_len == 0 || win.height <= 0) {
		window_clear_search_spans(win);
		return;
	}
	top = win.top_line;
	if (top < 1) {
		top = 1;
	}
	int bot = top + win.height - 1;
	if (bot > win.buffer.line_count_i()) {
		bot = win.buffer.line_count_i();
	}
	for (; line_n <= bot && n < (int)SearchSpanMax; line_n++) {
		Line *line = win.buffer.line(line_n);
		size_t lim = 0;
		size_t off = 0;
		if (line == NULL) {
			continue;
		}
		lim = line.byte_len();
		while (g_search.needle_len <= lim && off <= lim - g_search.needle_len &&
			n < (int)SearchSpanMax) {
			if (line_match_at(line, off, g_search.needle, g_search.needle_len)) {
				spans[n].line = line_n;
				spans[n].start = off;
				spans[n].end = off + g_search.needle_len;
				n++;
				off += g_search.needle_len;
				if (g_search.needle_len == 0) {
					break;
				}
			} else {
				off++;
			}
		}
	}
	window_set_search_spans(win, spans, n);
}

static void search_collect_visible_spans_regex(Window *win) {
	SearchSpan spans[SearchSpanMax] = {0};
	int n = 0;
	int top = 0;
	int line_n = top;
	if (win == NULL || win.buffer == NULL || g_search.needle_len == 0 || win.height <= 0) {
		window_clear_search_spans(win);
		return;
	}
	top = win.top_line;
	if (top < 1) {
		top = 1;
	}
	int bot = top + win.height - 1;
	if (bot > win.buffer.line_count_i()) {
		bot = win.buffer.line_count_i();
	}
	for (; line_n <= bot && n < (int)SearchSpanMax; line_n++) {
		Line *line = win.buffer.line(line_n);
		size_t lim = 0;
		char *copy = NULL;
		size_t cursor = 0;
		if (line == NULL) {
			continue;
		}
		lim = line.byte_len();
		copy = malloc(lim + 1);
		if (copy == NULL) {
			continue;
		}
		if (lim > 0) {
			memcpy(copy, ptr(line.data), lim);
		}
		{
			char[..] cv = ranged(copy, lim + 1);
			cv[lim] = 0;
		}
		cursor = 0;
		while (cursor <= lim && n < (int)SearchSpanMax) {
			size_t moff = 0;
			size_t mlen = 0;
			ReStatus st = ReNone;
			{
				auto (rst, roff, rlen) = re_find(ranged(g_search.needle, g_search.needle_len),
					ranged(copy + cursor, lim - cursor));
				st = rst;
				moff = roff;
				mlen = rlen;
			}
			if (st == ReInvalid) {
				free(copy);
				window_clear_search_spans(win);
				return;
			}
			if (st != ReFound || mlen == 0) {
				break;
			}
			spans[n].line = line_n;
			spans[n].start = cursor + moff;
			spans[n].end = cursor + moff + mlen;
			n++;
			cursor = cursor + moff + mlen;
			if (mlen == 0) {
				break;
			}
		}
		free(copy);
	}
	window_set_search_spans(win, spans, n);
}

static void search_collect_visible_spans(Window *win) {
	if (g_search.regexp) {
		search_collect_visible_spans_regex(win);
	} else {
		search_collect_visible_spans_literal(win);
	}
}

static void search_apply_highlights(Window *win) {
	if (win == NULL) {
		return;
	}
	if (!g_search.have_match || g_search.needle_len == 0) {
		window_clear_search_spans(win);
		win.mark.line = 0;
		win.mark.offset = 0;
		win.should_redraw = true;
		return;
	}
	search_highlight_match(win, g_search.match_begin, g_search.match_end);
	search_collect_visible_spans(win);
}

bool search_set_needle(char *s) {
	if (s == NULL) {
		g_search.needle_len = 0;
		g_search.needle[0] = 0;
		return true;
	}
	snprintf(g_search.needle, sizeof(g_search.needle), "%s", s);
	g_search.needle_len = strlen(g_search.needle);
	return true;
}

bool search_append_char(uint32_t ch) {
	char utf[8] = {0};
	int n = 0;
	if (ch < 32 || ch == 127) {
		return false;
	}
	if (ch < 128) {
		utf[0] = (char)ch;
		n = 1;
	} else {
		return false;
	}
	if (g_search.needle_len + (size_t)n >= sizeof(g_search.needle)) {
		return false;
	}
	memcpy(g_search.needle + g_search.needle_len, utf, n);
	g_search.needle_len = g_search.needle_len + (size_t)n;
	g_search.needle[g_search.needle_len] = 0;
	return true;
}

bool search_backspace() {
	if (g_search.needle_len == 0) {
		return false;
	}
	g_search.needle_len--;
	g_search.needle[g_search.needle_len] = 0;
	return true;
}

static bool search_apply_hit(Window *win, Location begin, Location end, bool wrapped) {
	g_search.match_begin = begin;
	g_search.match_end = end;
	g_search.have_match = true;
	g_search.wrap = wrapped;
	if (g_search.forward) {
		win.set_cursor(end);
	} else {
		win.set_cursor(begin);
	}
	win.ensure_visible();
	search_apply_highlights(win);
	return true;
}

bool search_next() {
	Window *win = window_current();
	Location begin = {0};
	Location end = {0};
	if (win == NULL || win.buffer == NULL || g_search.needle_len == 0) {
		g_search.have_match = false;
		search_apply_highlights(win);
		return false;
	}
	Location from = win.cursor;
	if (g_search.have_match && g_search.forward) {
		from = g_search.match_end;
	} else if (g_search.have_match && !g_search.forward) {
		from = g_search.match_begin;
		if (from.offset > 0) {
			from.offset--;
		} else if (g_search.regexp) {
			/* stay; prev engine uses end-limit compare */
		}
	}
	if (g_search.regexp) {
		int found = search_regex_from(win.buffer, from, g_search.forward, g_search.needle, &begin, &end);
		if (found > 0) {
			return search_apply_hit(win, begin, end, false);
		}
		if (found < 0) {
			g_search.have_match = false;
			search_apply_highlights(win);
			return false;
		}
	} else if (search_from(win.buffer, from, g_search.forward, g_search.needle, g_search.needle_len, &begin, &end)) {
		return search_apply_hit(win, begin, end, false);
	}
	if (g_search.replace_mode) {
		g_search.have_match = false;
		search_apply_highlights(win);
		return false;
	}
	/* wrap once */
	if (g_search.forward) {
		from.line = 1;
		from.offset = 0;
	} else {
		from.line = win.buffer.line_count_i();
		from.offset = 0;
		{
			Line *ll = win.buffer.line(from.line);
			if (ll != NULL) {
				from.offset = ll.byte_len();
			}
		}
	}
	if (g_search.regexp) {
		int found = search_regex_from(win.buffer, from, g_search.forward, g_search.needle, &begin, &end);
		if (found > 0) {
			return search_apply_hit(win, begin, end, true);
		}
		g_search.have_match = false;
		search_apply_highlights(win);
		return false;
	}
	if (search_from(win.buffer, from, g_search.forward, g_search.needle, g_search.needle_len, &begin, &end)) {
		return search_apply_hit(win, begin, end, true);
	}
	g_search.have_match = false;
	search_apply_highlights(win);
	return false;
}

bool search_update() {
	Window *win = window_current();
	Location begin = {0};
	Location end = {0};
	if (win == NULL || g_search.needle_len == 0) {
		g_search.have_match = false;
		search_apply_highlights(win);
		return false;
	}
	Location from = g_search.start;
	if (g_search.regexp) {
		int found = search_regex_from(win.buffer, from, g_search.forward, g_search.needle, &begin, &end);
		if (found > 0) {
			return search_apply_hit(win, begin, end, false);
		}
		g_search.have_match = false;
		search_apply_highlights(win);
		return false;
	}
	if (search_from(win.buffer, from, g_search.forward, g_search.needle, g_search.needle_len, &begin, &end)) {
		return search_apply_hit(win, begin, end, false);
	}
	g_search.have_match = false;
	search_apply_highlights(win);
	return false;
}

void search_begin_replace() {
	search_begin(true, false);
	g_search.replace_mode = true;
}

void search_begin_replace_re() {
	search_begin(true, true);
	g_search.replace_mode = true;
}

static size_t expand_regex_replacement_size(char *repl, ReCapture[..] caps) {
	size_t size = 0;
	size_t i = 0;
	if (repl == NULL) {
		return 0;
	}
	const char[..] rv = ranged(repl, strlen(repl));
	for (; i < len(rv); i++) {
		if (rv[i] == '\\' && i + 1 < len(rv)) {
			char esc = 0;
			i++;
			esc = rv[i];
			if (esc >= '0' && esc <= '9') {
				size_t idx = esc - '0';
				if (idx < len(caps)) {
					ReCapture cap = caps[idx];
					if (cap.start >= 0 && cap.length >= 0) {
						size = size + cap.length;
					}
				}
			} else if (esc == 'n') {
				size++;
			} else {
				size++;
			}
		} else {
			size++;
		}
	}
	return size;
}

static void expand_regex_replacement_into(char *repl, char *match_text, ReCapture[..] caps,
	char[..] expanded, size_t *out_len) {
	size_t used = 0;
	size_t i = 0;
	const char[..] rv = ranged(repl, strlen(repl));
	for (; i < len(rv); i++) {
		if (rv[i] == '\\' && i + 1 < len(rv)) {
			char esc = 0;
			i++;
			esc = rv[i];
			if (esc >= '0' && esc <= '9') {
				size_t idx = esc - '0';
				if (idx < len(caps)) {
					ReCapture cap = caps[idx];
					if (cap.start >= 0 && cap.length >= 0) {
						memcpy(ptr(expanded) + used, match_text + cap.start, cap.length);
						used = used + cap.length;
					}
				}
			} else if (esc == 'n') {
				expanded[used] = '\n';
				used++;
			} else {
				expanded[used] = esc;
				used++;
			}
		} else {
			expanded[used] = rv[i];
			used++;
		}
	}
	expanded[used] = 0;
	*out_len = used;
}

bool search_replace_one() {
	Window *win = window_current();
	if (win == NULL || !g_search.have_match) {
		return false;
	}
	Location begin = g_search.match_begin;
	Line *line = win.buffer.line(begin.line);
	if (line == NULL || g_search.match_end.line != begin.line ||
		g_search.match_end.offset < begin.offset) {
		/* Multi-line regex matches: still replace via set_text range. */
		if (line == NULL) {
			return false;
		}
	}
	if (g_search.match_end.line < begin.line ||
		(g_search.match_end.line == begin.line && g_search.match_end.offset < begin.offset)) {
		return false;
	}
	/* Capture deleted text for undo — flatten range via save when multi-line. */
	size_t old_len = 0;
	if (g_search.match_end.line == begin.line) {
		old_len = g_search.match_end.offset - begin.offset;
	}
	pool p {
		char[..] old = p.alloc(char, old_len + 1);
		if (ptr(old) == NULL) {
			return false;
		}
		if (g_search.match_end.line == begin.line) {
			memcpy(ptr(old), ptr(line.data) + begin.offset, old_len);
		}
		old[old_len] = 0;
		char *repl = g_search.replacement;
		size_t repl_len = g_search.replacement_len;
		if (g_search.regexp) {
			if (g_search.re_slice == NULL) {
				return false;
			}
			size_t need = expand_regex_replacement_size(g_search.replacement, g_search.captures);
			char[..] expanded = p.alloc(char, need + 1);
			if (ptr(expanded) == NULL) {
				return false;
			}
			expand_regex_replacement_into(g_search.replacement, g_search.re_slice,
				g_search.captures, expanded, &repl_len);
			repl = ptr(expanded);
		}
		buffer_undo.begin_command(win.buffer, win.cursor);
		if (!win.set_text(g_search.match_begin, g_search.match_end, repl, repl_len)) {
			buffer_undo.end_command();
			return false;
		}
		buffer_undo.record_edit(UndoDelete, begin.line, begin.offset, ptr(old), old_len);
		buffer_undo.record_edit(UndoInsert, begin.line, begin.offset, repl, repl_len);
		buffer_undo.end_command();
		search_clear_re_slice();
		if (!g_search.forward) {
			win.set_cursor(begin);
		}
		g_search.have_match = false;
		(void)search_next();
		return true;
	}
}

bool search_set_replacement(char *s) {
	if (s == NULL) {
		g_search.replacement_len = 0;
		g_search.replacement[0] = 0;
		return true;
	}
	snprintf(g_search.replacement, sizeof(g_search.replacement), "%s", s);
	g_search.replacement_len = strlen(g_search.replacement);
	return true;
}

char *search_needle() {
	return g_search.needle;
}

int search_scope_get() {
	return g_search_scope;
}

void search_scope_set(int scope) {
	if (scope == (int)SearchScopeAllBuffers) {
		g_search_scope = SearchScopeAllBuffers;
	} else {
		g_search_scope = SearchScopeBuffer;
	}
}

bool search_scope_is_all() {
	return g_search_scope == (int)SearchScopeAllBuffers;
}

// search_format_prompt builds a minibuffer label like Go SearchPromptLabel
// ("Search [all]: " / "Search: ").
void search_format_prompt(char *dst, size_t cap, const char *label) {
	if (dst == NULL || cap == 0) {
		return;
	}
	if (label == NULL) {
		label = "Search";
	}
	if (search_scope_is_all()) {
		snprintf(dst, cap, "%s [all]: ", label);
	} else {
		snprintf(dst, cap, "%s: ", label);
	}
}

// search_format_isearch_prompt builds Go/C isearch prefixes
// ("I-search[all *scratch*]: " / "I-search: ").
void search_format_isearch_prompt(char *dst, size_t cap, const char *label,
	Buffer *buf) {
	char name[128] = {0};
	if (dst == NULL || cap == 0) {
		return;
	}
	if (label == NULL) {
		label = "I-search";
	}
	if (search_scope_is_all()) {
		if (buf != NULL && len(buf.name) > 0) {
			snprintf(name, sizeof(name), "%s", buf.name);
		} else {
			snprintf(name, sizeof(name), "?");
		}
		snprintf(dst, cap, "%s[all %s]: ", label, name);
	} else {
		snprintf(dst, cap, "%s: ", label);
	}
}

bool search_toggle_scope() {
	if (g_search_scope == (int)SearchScopeAllBuffers) {
		g_search_scope = SearchScopeBuffer;
	} else {
		g_search_scope = SearchScopeAllBuffers;
	}
	return search_scope_is_all();
}

void search_set_last_pattern(const char *s) {
	if (s == NULL) {
		g_last_pattern[0] = 0;
		return;
	}
	snprintf(g_last_pattern, sizeof(g_last_pattern), "%s", s);
}

const char *search_last_pattern() {
	return g_last_pattern;
}

static bool search_plain_in_buffer(Window *win, Buffer *buf, bool forward, char *needle, size_t nlen) {
	Location begin = {0};
	Location end = {0};
	if (win == NULL || buf == NULL || nlen == 0) {
		return false;
	}
	Location from = win.cursor;
	if (forward) {
		if (win.buffer != buf) {
			from.line = 1;
			from.offset = 0;
		}
	} else if (win.buffer != buf) {
		from.line = buf.eof_line();
		from.offset = 0;
		if (buf.line_count_i() >= 1) {
			Line *last = buf.line(buf.line_count_i());
			from.line = buf.line_count_i();
			from.offset = last != NULL ? last.byte_len() : 0;
		}
	}
	if (!search_from(buf, from, forward, needle, nlen, &begin, &end)) {
		return false;
	}
	if (win.buffer != buf) {
		window_switch_buffer(buf);
		win = window_current();
		if (win == NULL) {
			return false;
		}
	}
	if (forward) {
		win.set_cursor(end);
	} else {
		win.set_cursor(begin);
	}
	win.did_move = true;
	win.ensure_visible();
	return true;
}

// search_plain_find runs a non-incremental plain search using g_last_pattern.
bool search_plain_find(bool forward) {
	Window *win = window_current();
	char needle[256] = {0};
	size_t i = 0;
	int origin_idx = 0;
	if (win == NULL || win.buffer == NULL) {
		return false;
	}
	snprintf(needle, sizeof(needle), "%s", g_last_pattern);
	size_t nlen = strlen(needle);
	if (nlen == 0) {
		return true;
	}
	Buffer *origin = win.buffer;
	if (search_plain_in_buffer(win, origin, forward, needle, nlen)) {
		return true;
	}
	if (!search_scope_is_all()) {
		return false;
	}
	size_t nbuf = len(buffer_all);
	for (; i < nbuf; i++) {
		if (buffer_all[i] == origin) {
			origin_idx = (int)i;
			break;
		}
	}
	for (i = 1; i < nbuf; i++) {
		Buffer *buf = buffer_all[(origin_idx + (int)i) % (int)nbuf];
		Location start = {0};
		if (buf == NULL || buf == origin) {
			continue;
		}
		win = window_current();
		if (forward) {
			start.line = 1;
			start.offset = 0;
			window_switch_buffer(buf);
			win = window_current();
			if (win == NULL) {
				continue;
			}
			win.set_cursor(start);
		} else {
			Line *last = NULL;
			window_switch_buffer(buf);
			win = window_current();
			if (win == NULL) {
				continue;
			}
			if (buf.line_count_i() >= 1) {
				last = buf.line(buf.line_count_i());
				start.line = buf.line_count_i();
				start.offset = last != NULL ? last.byte_len() : 0;
			} else {
				start.line = 1;
				start.offset = 0;
			}
			win.set_cursor(start);
		}
		if (search_plain_in_buffer(win, buf, forward, needle, nlen)) {
			return true;
		}
	}
	/* restore origin if nothing found */
	window_switch_buffer(origin);
	return false;
}


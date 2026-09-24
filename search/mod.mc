// search — incremental search and query-replace (simplified).
import "buffer";
import "window";

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>
#include <string.h>
#include <stdlib.h>
#include <stdio.h>
#include <ctype.h>

struct SearchState {
	bool active;
	bool forward;
	bool regexp; /* unused for now; literal only */
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
};

static SearchState g_search;

void search_reset() {
	memset(&g_search, 0, sizeof(g_search));
	g_search.forward = true;
}

SearchState *search_state() {
	return &g_search;
}

bool search_active() {
	return g_search.active;
}

static bool line_match_at(Line *line, size_t offset, char *needle, size_t nlen) {
	char[..] v = {0};
	size_t i = 0;
	if (line == NULL || needle == NULL || nlen == 0) {
		return false;
	}
	v = line.view();
	if (offset + nlen > len(v)) {
		return false;
	}
	{
		char *vp = ptr(v);
		for (i = 0; i < nlen; i++) {
			if (vp[offset + i] != needle[i]) {
				return false;
			}
		}
	}
	return true;
}

static bool search_from(Buffer *buf, Location from, bool forward, char *needle, size_t nlen, Location *begin, Location *end) {
	int line_n = 0;
	int nlines = 0;
	if (buf == NULL || nlen == 0) {
		return false;
	}
	nlines = buf.line_count_i();
	if (forward) {
		line_n = from.line;
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
		line_n = from.line;
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

void search_begin(bool forward) {
	Window *win = window_current();
	search_reset();
	g_search.active = true;
	g_search.forward = forward;
	if (win != NULL) {
		g_search.start = win.cursor;
	}
}

void search_end() {
	Window *win = window_current();
	g_search.active = false;
	g_search.have_match = false;
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

static void search_collect_visible_spans(Window *win) {
	SearchSpan spans[SearchSpanMax] = {0};
	int n = 0;
	int top = 0;
	int bot = 0;
	int line_n = 0;
	if (win == NULL || win.buffer == NULL || g_search.needle_len == 0 || win.height <= 0) {
		window_clear_search_spans(win);
		return;
	}
	top = win.top_line;
	if (top < 1) {
		top = 1;
	}
	bot = top + win.height - 1;
	if (bot > win.buffer.line_count_i()) {
		bot = win.buffer.line_count_i();
	}
	for (line_n = top; line_n <= bot && n < (int)SearchSpanMax; line_n++) {
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
		return false; /* keep needle ASCII for MVP regex-less search */
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

bool search_next() {
	Window *win = window_current();
	Location from = {0};
	Location begin = {0};
	Location end = {0};
	if (win == NULL || win.buffer == NULL || g_search.needle_len == 0) {
		g_search.have_match = false;
		search_apply_highlights(win);
		return false;
	}
	from = win.cursor;
	if (g_search.have_match && g_search.forward) {
		from = g_search.match_end;
	} else if (g_search.have_match && !g_search.forward) {
		from = g_search.match_begin;
		if (from.offset > 0) {
			from.offset--;
		}
	}
	if (search_from(win.buffer, from, g_search.forward, g_search.needle, g_search.needle_len, &begin, &end)) {
		g_search.match_begin = begin;
		g_search.match_end = end;
		g_search.have_match = true;
		if (g_search.forward) {
			win.set_cursor(end);
		} else {
			win.set_cursor(begin);
		}
		win.ensure_visible();
		search_apply_highlights(win);
		return true;
	}
	if (g_search.replace_mode) {
		g_search.have_match = false;
		search_apply_highlights(win);
		return false;
	}
	/* wrap once */
	if (g_search.forward) {
		from.line = 1;
	} else {
		from.line = win.buffer.line_count_i();
	}
	from.offset = 0;
	if (!g_search.forward) {
		Line *ll = win.buffer.line(from.line);
		if (ll != NULL) {
		from.offset = ll.byte_len();
	} else {
		from.offset = 0;
	}
	}
	if (search_from(win.buffer, from, g_search.forward, g_search.needle, g_search.needle_len, &begin, &end)) {
		g_search.match_begin = begin;
		g_search.match_end = end;
		g_search.have_match = true;
		g_search.wrap = true;
		if (g_search.forward) {
			win.set_cursor(end);
		} else {
			win.set_cursor(begin);
		}
		win.ensure_visible();
		search_apply_highlights(win);
		return true;
	}
	g_search.have_match = false;
	search_apply_highlights(win);
	return false;
}

bool search_update() {
	Window *win = window_current();
	Location from = {0};
	Location begin = {0};
	Location end = {0};
	if (win == NULL || g_search.needle_len == 0) {
		g_search.have_match = false;
		search_apply_highlights(win);
		return false;
	}
	from = g_search.start;
	if (search_from(win.buffer, from, g_search.forward, g_search.needle, g_search.needle_len, &begin, &end)) {
		g_search.match_begin = begin;
		g_search.match_end = end;
		g_search.have_match = true;
		if (g_search.forward) {
			win.set_cursor(end);
		} else {
			win.set_cursor(begin);
		}
		win.ensure_visible();
		search_apply_highlights(win);
		return true;
	}
	g_search.have_match = false;
	search_apply_highlights(win);
	return false;
}

void search_begin_replace() {
	search_begin(true);
	g_search.replace_mode = true;
}

bool search_replace_one() {
	Window *win = window_current();
	Line *line = NULL;
	char *old = NULL;
	size_t old_len = 0;
	Location begin = {0};
	if (win == NULL || !g_search.have_match) {
		return false;
	}
	begin = g_search.match_begin;
	line = win.buffer.line(begin.line);
	if (line == NULL || g_search.match_end.line != begin.line ||
		g_search.match_end.offset < begin.offset) {
		return false;
	}
	old_len = g_search.match_end.offset - begin.offset;
	old = malloc(old_len + 1);
	if (old == NULL) {
		return false;
	}
	memcpy(old, ptr(line.data) + begin.offset, old_len);
	old[old_len] = 0;
	buffer_undo.begin_command(win.buffer, win.cursor);
	if (!win.set_text(g_search.match_begin, g_search.match_end, g_search.replacement, g_search.replacement_len)) {
		buffer_undo.end_command();
		free(old);
		return false;
	}
	buffer_undo.record_edit(UndoDelete, begin.line, begin.offset, old, old_len);
	buffer_undo.record_edit(UndoInsert, begin.line, begin.offset,
		g_search.replacement, g_search.replacement_len);
	free(old);
	buffer_undo.end_command();
	if (!g_search.forward) {
		win.set_cursor(begin);
	}
	g_search.have_match = false;
	return search_next();
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

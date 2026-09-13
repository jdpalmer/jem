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

void search_reset(void) {
	memset(&g_search, 0, sizeof(g_search));
	g_search.forward = true;
}

SearchState *search_state(void) {
	return &g_search;
}

bool search_active(void) {
	return g_search.active;
}

static bool line_match_at(Line *line, int offset, char *needle, size_t nlen) {
	char[..] v = {0};
	size_t i = 0;
	if (line == NULL || needle == NULL || nlen == 0) {
		return false;
	}
	v = line.view();
	if ((size_t)offset + nlen > len(v)) {
		return false;
	}
	{
		char *vp = ptr(v);
		for (i = 0; i < nlen; i++) {
			if (vp[(size_t)offset + i] != needle[i]) {
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
			int off = 0;
			if (line_n == from.line) {
				off = from.offset;
			}
			int lim = 0;
			if (line != NULL) {
				lim = line.byte_len();
			}
			while (off <= lim - (int)nlen) {
				if (line_match_at(line, off, needle, nlen)) {
					begin.line = line_n;
					begin.offset = off;
					end.line = line_n;
					end.offset = off + (int)nlen;
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
			int lim = 0;
			if (line != NULL) {
				lim = line.byte_len();
			}
			int off = lim;
			if (line_n == from.line) {
				off = from.offset;
			}
			if (off > lim) {
				off = lim;
			}
			off = off - (int)nlen;
			while (off >= 0) {
				if (line_match_at(line, off, needle, nlen)) {
					begin.line = line_n;
					begin.offset = off;
					end.line = line_n;
					end.offset = off + (int)nlen;
					return true;
				}
				off--;
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

void search_end(void) {
	g_search.active = false;
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
	memcpy(g_search.needle + g_search.needle_len, utf, (size_t)n);
	g_search.needle_len = g_search.needle_len + (size_t)n;
	g_search.needle[g_search.needle_len] = 0;
	return true;
}

bool search_backspace(void) {
	if (g_search.needle_len == 0) {
		return false;
	}
	g_search.needle_len--;
	g_search.needle[g_search.needle_len] = 0;
	return true;
}

bool search_next(void) {
	Window *win = window_current();
	Location from = {0};
	Location begin = {0};
	Location end = {0};
	if (win == NULL || win.buffer == NULL || g_search.needle_len == 0) {
		g_search.have_match = false;
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
		return true;
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
		return true;
	}
	g_search.have_match = false;
	return false;
}

bool search_update(void) {
	Window *win = window_current();
	Location from = {0};
	Location begin = {0};
	Location end = {0};
	if (win == NULL || g_search.needle_len == 0) {
		g_search.have_match = false;
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
		return true;
	}
	g_search.have_match = false;
	return false;
}

void search_begin_replace(void) {
	search_begin(true);
	g_search.replace_mode = true;
}

bool search_replace_one(void) {
	Window *win = window_current();
	if (win == NULL || !g_search.have_match) {
		return false;
	}
	buffer_undo.begin_command(win.buffer, win.cursor);
	if (!win.set_text(g_search.match_begin, g_search.match_end, g_search.replacement, g_search.replacement_len)) {
		buffer_undo.end_command();
		return false;
	}
	buffer_undo.record_edit(UndoInsert, g_search.match_begin.line, g_search.match_begin.offset,
		g_search.replacement, g_search.replacement_len);
	buffer_undo.end_command();
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

char *search_needle(void) {
	return g_search.needle;
}

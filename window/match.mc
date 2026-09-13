// window/match.mc — *match* popup window for fuzzy pickers (jem.go/window/match.go).
#include <stddef.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

static char MatchBufferName[] = "*match*";

Window *window_match(void) {
	Buffer *mb = buffer_find_by_name(MatchBufferName);
	size_t i = 0;
	if (mb == NULL) {
		return NULL;
	}
	for (i = 0; i < window_active.count; i++) {
		Window *win = window_active.windows[i];
		if (win != NULL && win.buffer == mb) {
			return win;
		}
	}
	return NULL;
}

static Window *window_showing_buffer(Buffer *buf) {
	size_t i = 0;
	if (buf == NULL) {
		return NULL;
	}
	for (i = 0; i < window_active.count; i++) {
		Window *win = window_active.windows[i];
		if (win != NULL && win.buffer == buf) {
			return win;
		}
	}
	return NULL;
}

/* Ensure a window shows buf without selecting it (jem.go EnsureBufferWindow). */
static Window *window_ensure_buffer(Buffer *buf) {
	Window *win = {0};
	Window *cur = window_current();
	if (buf == NULL) {
		return NULL;
	}
	win = window_showing_buffer(buf);
	if (win != NULL) {
		return win;
	}
	win = window_create();
	if (win == NULL) {
		return NULL;
	}
	win.buffer = buf;
	win.top_line = 1;
	win.cursor.line = 1;
	win.cursor.offset = 0;
	win.mark.line = 0;
	win.mark.offset = 0;
	win.should_redraw = true;
	win.should_update_mode_line = true;
	window_retile();
	if (cur != NULL) {
		window_active.current = cur;
	}
	return win;
}

static void window_hide_match(void) {
	Window *mw = window_match();
	size_t i = 0;
	size_t j = 0;
	if (mw == NULL || window_active.count <= (size_t)1) {
		return;
	}
	for (i = 0; i < window_active.count; i++) {
		if (window_active.windows[i] != mw) {
			continue;
		}
		if (window_active.current == mw) {
			Window *sel = window_active.windows[0];
			if (i == 0 && window_active.count > 1) {
				sel = window_active.windows[1];
			}
			window_select(sel);
		}
		free(mw);
		for (j = i; j + 1 < window_active.count; j++) {
			window_active.windows[j] = window_active.windows[j + 1];
		}
		window_active.count--;
		window_active.windows[window_active.count] = NULL;
		window_retile();
		return;
	}
}

void window_discard_match_buffer(void) {
	Buffer *mb = {0};
	window_hide_match();
	mb = buffer_find_by_name(MatchBufferName);
	if (mb != NULL) {
		buffer_release(mb);
	}
}

static void window_scroll_match_to_selection(int selected) {
	Window *mw = window_match();
	int line = 0;
	if (mw == NULL) {
		return;
	}
	if (selected < 0) {
		selected = 0;
	}
	line = selected + 1;
	if (mw.buffer != NULL) {
		int n = mw.buffer.line_count_i();
		if (n > 0 && line > n) {
			line = n;
		}
	}
	{
		Location loc = {0};
		loc.line = line;
		loc.offset = 0;
		mw.set_cursor(loc);
	}
	if (line < mw.top_line) {
		mw.top_line = line;
	} else if (mw.height > 0) {
		int last = mw.top_line + mw.height - 1;
		if (line > last) {
			mw.top_line = line - mw.height + 1;
			if (mw.top_line < 1) {
				mw.top_line = 1;
			}
		}
	}
	mw.should_redraw = true;
}

static Buffer *ensure_match_buffer(void) {
	Buffer *mb = buffer_find_by_name(MatchBufferName);
	if (mb != NULL) {
		return mb;
	}
	mb = buffer_new();
	if (mb == NULL) {
		return NULL;
	}
	mb.set_name(MatchBufferName);
	mb.lang_mode = LModeNone;
	if (!buffer_list_append(mb)) {
		mb.free();
		return NULL;
	}
	return mb;
}

static void show_match_window(void) {
	Buffer *mb = buffer_find_by_name(MatchBufferName);
	Window *win = {0};
	if (mb == NULL) {
		return;
	}
	win = window_ensure_buffer(mb);
	if (win == NULL) {
		return;
	}
	win.bottom_align = true;
	win.no_mode_line = true;
	win.should_redraw = true;
}

/* Replace *match* contents and show/scroll. Empty text hides the window. */
void window_set_match_buffer_text(char *text, size_t n, int selected) {
	Buffer *mb = {0};
	Location begin = {0};
	Location end = {0};
	Location new_end = {0};
	bool ro = false;
	if (text == NULL || n == 0) {
		window_discard_match_buffer();
		return;
	}
	mb = ensure_match_buffer();
	if (mb == NULL) {
		return;
	}
	ro = mb.is_readonly;
	mb.is_readonly = false;
	begin.line = 1;
	begin.offset = 0;
	end.line = mb.line_count_i();
	if (end.line < 1) {
		end.line = 1;
		end.offset = 0;
	} else {
		Line *last = mb.line(end.line);
		end.offset = last != NULL ? last.byte_len() : 0;
	}
	mb.replace_raw(begin, end, text, n, &new_end);
	mb.is_readonly = true;
	mb.is_changed = false;
	(void)ro;
	show_match_window();
	window_scroll_match_to_selection(selected);
}

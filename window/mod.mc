// window — viewports, cursor motion, splits.
import "buffer";
import "term";

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

enum WindowLimits {
	WindowMax = 8,
	GoalColUnset = -1,
	SearchSpanMax = 256
};

struct SearchSpan {
	int line;
	int start;
	int end;
};

struct Window {
	Buffer *buffer;
	int top_line;
	Location cursor;
	Location mark;
	int goal_col;
	int h_scroll;
	int screen_top_row;
	int height;
	bool should_redraw;
	bool should_update_mode_line;
	bool no_mode_line;
	bool did_move;
	bool did_edit;
	bool bottom_align;
	SearchSpan search_spans[SearchSpanMax];
	int search_span_count;
};

struct WindowState {
	Window *windows[WindowMax];
	size_t count;
	Window *current;
};

WindowState window_active;

void window_select(Window *win) {
	Window *old = {0};
	if (win == NULL) {
		return;
	}
	old = window_active.current;
	if (old != NULL && old != win) {
		old.should_update_mode_line = true;
	}
	window_active.current = win;
	buffer_set_current(win.buffer);
	win.should_redraw = true;
	win.should_update_mode_line = true;
}

Window *window_create(void) {
	Window *win = {0};
	if (window_active.count >= (size_t)WindowMax) {
		return NULL;
	}
	win = calloc(1, sizeof(Window));
	if (win == NULL) {
		return NULL;
	}
	win.buffer = buffer_current();
	win.top_line = 1;
	win.cursor.line = 1;
	win.cursor.offset = 0;
	win.mark.line = 0;
	win.mark.offset = 0;
	win.goal_col = GoalColUnset;
	win.should_redraw = true;
	win.should_update_mode_line = true;
	window_active.windows[window_active.count++] = win;
	return win;
}

void window_retile(void) {
	size_t n = window_active.count;
	size_t i = 0;
	int mode_lines = 0;
	int usable = 0;
	int base_rows = 0;
	int extra = 0;
	int top = 0;
	if (n == 0) {
		return;
	}
	for (i = 0; i < n; i++) {
		if (window_active.windows[i] != NULL && !window_active.windows[i].no_mode_line) {
			mode_lines++;
		}
	}
	usable = term_rows() - mode_lines;
	if (usable < 0) {
		usable = 0;
	}
	base_rows = usable / (int)n;
	extra = usable % (int)n;
	for (i = 0; i < n; i++) {
		Window *win = window_active.windows[i];
		int rows = base_rows;
		if (win == NULL) {
			continue;
		}
		if (extra > 0) {
			rows++;
			extra--;
		}
		win.screen_top_row = top;
		win.height = rows;
		win.should_redraw = true;
		win.should_update_mode_line = true;
		top = top + rows;
		if (!win.no_mode_line) {
			top++;
		}
	}
}

Window *window_current(void) {
	return window_active.current;
}

void (Window *win).save_state(void) {
	if (win == NULL || win.buffer == NULL) {
		return;
	}
	win.buffer.cursor = win.cursor;
	win.buffer.mark = win.mark;
}

// window_switch_buffer makes buf current in the active window (jem.go SwitchBuffer).
void window_switch_buffer(Buffer *buf) {
	Window *cw = window_current();
	size_t i = 0;
	if (buf == NULL || cw == NULL) {
		return;
	}
	cw.save_state();
	buffer_set_current(buf);
	cw.buffer = buf;
	cw.should_update_mode_line = true;
	cw.should_redraw = true;
	cw.top_line = 1;
	cw.h_scroll = 0;
	for (i = 0; i < window_active.count; i++) {
		Window *win = window_active.windows[i];
		if (win != NULL && win != cw && win.buffer == buf) {
			cw.top_line = win.top_line;
			cw.cursor = win.cursor;
			cw.mark = win.mark;
			cw.goal_col = win.goal_col;
			cw.h_scroll = win.h_scroll;
			return;
		}
	}
	if (buf.cursor.line >= 1) {
		cw.set_cursor(buf.cursor);
	} else {
		Location loc = {0};
		loc.line = 1;
		loc.offset = 0;
		cw.set_cursor(loc);
	}
	cw.mark = buf.mark;
}

int window_gutter_width(Window *win) {
	int n = 1;
	int digits = 1;
	int width = 0;
	int cols = 0;
	if (win == NULL || win.buffer == NULL) {
		return 3;
	}
	n = win.buffer.line_count_i();
	if (n < 1) {
		n = 1;
	}
	while (n >= 10) {
		digits++;
		n /= 10;
	}
	width = digits + 2;
	cols = term_cols();
	if (width >= cols) {
		width = cols - 1;
	}
	if (width < 3) {
		width = 3;
	}
	return width;
}

int window_content_row_offset(Window *win) {
	int visible = 0;
	if (win == NULL || win.buffer == NULL || !win.bottom_align || win.height <= 0) {
		return 0;
	}
	visible = win.buffer.line_count_i() - win.top_line + 1;
	if (visible < 0) {
		visible = 0;
	}
	if (visible >= win.height) {
		return 0;
	}
	return win.height - visible;
}

static void sync_goal_col(Window *win) {
	Line *line = {0};
	if (win == NULL || win.buffer == NULL) {
		return;
	}
	line = win.buffer.line(win.cursor.line);
	if (line == NULL) {
		win.goal_col = 0;
		return;
	}
	win.goal_col = line.display_col_at_offset(win.cursor.offset);
}

static void ensure_goal_col(Window *win) {
	if (win == NULL || win.goal_col >= 0) {
		return;
	}
	sync_goal_col(win);
}

static void apply_goal_col(Window *win) {
	Line *line = {0};
	if (win == NULL || win.buffer == NULL) {
		return;
	}
	line = win.buffer.line(win.cursor.line);
	if (line == NULL) {
		win.cursor.offset = 0;
		return;
	}
	if (win.goal_col < 0) {
		win.goal_col = line.display_col_at_offset(win.cursor.offset);
	}
	win.cursor.offset = line.offset_at_display_col(win.goal_col);
}

static void mark_moved(Window *win) {
	win.did_move = true;
	win.should_redraw = true;
}

void (Window *win).set_cursor(Location loc) {
	if (win == NULL) {
		return;
	}
	win.cursor = loc;
	win.goal_col = GoalColUnset;
	mark_moved(win);
}

void (Window *win).move_vertical(int delta) {
	int nlines = 0;
	if (win == NULL || win.buffer == NULL || delta == 0) {
		return;
	}
	ensure_goal_col(win);
	nlines = win.buffer.line_count_i();
	if (delta > 0) {
		if (win.cursor.line + delta <= nlines) {
			win.cursor.line = win.cursor.line + delta;
		} else {
			win.cursor.line = nlines;
		}
	} else {
		int n = -delta;
		if (win.cursor.line > n) {
			win.cursor.line = win.cursor.line - n;
		} else {
			win.cursor.line = 1;
		}
	}
	apply_goal_col(win);
	mark_moved(win);
}

void (Window *win).move_forward_char(int n) {
	int i = 0;
	if (win == NULL || win.buffer == NULL || n <= 0) {
		return;
	}
	for (i = 0; i < n; i++) {
		Line *line = win.buffer.line(win.cursor.line);
		if (line != NULL && win.cursor.offset < line.byte_len()) {
			win.cursor.offset = utf8_next_offset(line.view(), win.cursor.offset);
		} else if (win.cursor.line < win.buffer.line_count_i()) {
			win.cursor.line++;
			win.cursor.offset = 0;
		} else {
			break;
		}
	}
	sync_goal_col(win);
	mark_moved(win);
}

void (Window *win).move_backward_char(int n) {
	int i = 0;
	if (win == NULL || win.buffer == NULL || n <= 0) {
		return;
	}
	for (i = 0; i < n; i++) {
		Line *line = win.buffer.line(win.cursor.line);
		if (line != NULL && win.cursor.offset > 0) {
			win.cursor.offset = utf8_prev_offset(line.view(), win.cursor.offset);
		} else if (win.cursor.line > 1) {
			win.cursor.line--;
			line = win.buffer.line(win.cursor.line);
			if (line != NULL) {
				win.cursor.offset = line.byte_len();
			} else {
				win.cursor.offset = 0;
			}
		} else {
			break;
		}
	}
	sync_goal_col(win);
	mark_moved(win);
}

static bool is_word_char(unsigned char b) {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_';
}

static Location forward_word_loc(Buffer *buf, Location loc) {
	Line *line = {0};
	char[..] v = {0};
	int off = 0;
	int nlines = 0;
	if (buf == NULL) {
		return loc;
	}
	nlines = buf.line_count_i();
	line = buf.line(loc.line);
	if (line == NULL) {
		return loc;
	}
	v = line.view();
	off = loc.offset;
	if (off >= (int)len(v)) {
		if (loc.line >= nlines) {
			loc.offset = line.byte_len();
			return loc;
		}
		loc.line++;
		off = 0;
	} else {
		{
			char *vp = ptr(v);
			while (off < (int)len(v) && !is_word_char((unsigned char)vp[off])) {
				off++;
			}
		}
	}
	while (loc.line <= nlines) {
		line = buf.line(loc.line);
		if (line == NULL) {
			return loc;
		}
		v = line.view();
		{
			char *vp = ptr(v);
			while (off < (int)len(v) && !is_word_char((unsigned char)vp[off])) {
				off++;
			}
			if (off < (int)len(v)) {
				while (off < (int)len(v) && is_word_char((unsigned char)vp[off])) {
					off++;
				}
				loc.offset = off;
				return loc;
			}
		}
		if (loc.line >= nlines) {
			loc.offset = 0;
			return loc;
		}
		loc.line++;
		off = 0;
	}
	loc.line = nlines;
	loc.offset = 0;
	return loc;
}

static Location backward_word_loc(Buffer *buf, Location loc) {
	Line *line = {0};
	char[..] v = {0};
	int off = 0;
	if (buf == NULL) {
		return loc;
	}
	if (loc.line == 1 && loc.offset == 0) {
		return loc;
	}
	line = buf.line(loc.line);
	if (line == NULL) {
		return loc;
	}
	v = line.view();
	off = loc.offset;
	if (off == 0) {
		if (loc.line > 1) {
			loc.line--;
			line = buf.line(loc.line);
			if (line != NULL) {
				v = line.view();
				off = (int)len(v);
			} else {
				off = 0;
			}
		} else {
			return loc;
		}
	}
	while (off > 0) {
		int prev = utf8_prev_offset(v, off);
		unsigned char b = 0;
		if (prev == off) {
			off--;
		} else {
			off = prev;
		}
		{
			char *vp = ptr(v);
			if (off < (int)len(v)) {
				b = (unsigned char)vp[off];
			}
			if (is_word_char(b) || off == 0) {
				break;
			}
		}
	}
	{
		char *vp = ptr(v);
		while (off > 0 && !is_word_char((unsigned char)vp[off - 1])) {
			off--;
		}
		while (off > 0 && is_word_char((unsigned char)vp[off - 1])) {
			off--;
		}
	}
	loc.offset = off;
	return loc;
}

void (Window *win).move_forward_word(int n) {
	int i = 0;
	if (win == NULL || win.buffer == NULL || n <= 0) {
		return;
	}
	for (i = 0; i < n; i++) {
		win.cursor = forward_word_loc(win.buffer, win.cursor);
	}
	sync_goal_col(win);
	mark_moved(win);
}

void (Window *win).move_backward_word(int n) {
	int i = 0;
	if (win == NULL || win.buffer == NULL || n <= 0) {
		return;
	}
	for (i = 0; i < n; i++) {
		win.cursor = backward_word_loc(win.buffer, win.cursor);
	}
	sync_goal_col(win);
	mark_moved(win);
}

void (Window *win).move_bol(void) {
	if (win == NULL) {
		return;
	}
	win.cursor.offset = 0;
	sync_goal_col(win);
	mark_moved(win);
}

void (Window *win).move_eol(void) {
	Line *line = {0};
	if (win == NULL || win.buffer == NULL) {
		return;
	}
	line = win.buffer.line(win.cursor.line);
	if (line != NULL) {
				win.cursor.offset = line.byte_len();
			} else {
				win.cursor.offset = 0;
			}
	sync_goal_col(win);
	mark_moved(win);
}

void (Window *win).ensure_visible(void) {
	int bottom = 0;
	if (win == NULL || win.height <= 0) {
		return;
	}
	if (win.cursor.line < win.top_line) {
		win.top_line = win.cursor.line;
		win.should_redraw = true;
	}
	bottom = win.top_line + win.height - 1;
	if (win.cursor.line > bottom) {
		win.top_line = win.cursor.line - win.height + 1;
		if (win.top_line < 1) {
			win.top_line = 1;
		}
		win.should_redraw = true;
	}
}

bool window_split(void) {
	Window *cur = window_current();
	Window *nw = {0};
	if (cur == NULL) {
		return false;
	}
	nw = window_create();
	if (nw == NULL) {
		return false;
	}
	nw.buffer = cur.buffer;
	nw.cursor = cur.cursor;
	nw.mark = cur.mark;
	nw.top_line = cur.top_line;
	window_retile();
	window_select(nw);
	return true;
}

bool window_only(void) {
	Window *cur = window_current();
	size_t i = 0;
	if (cur == NULL) {
		return false;
	}
	for (i = 0; i < window_active.count; i++) {
		if (window_active.windows[i] != cur) {
			free(window_active.windows[i]);
		}
	}
	window_active.windows[0] = cur;
	window_active.count = 1;
	window_retile();
	return true;
}

bool window_delete(void) {
	Window *cur = window_current();
	size_t i = 0;
	size_t j = 0;
	if (cur == NULL || window_active.count <= (size_t)1) {
		return false;
	}
	for (i = 0; i < window_active.count; i++) {
		if (window_active.windows[i] == cur) {
			free(cur);
			for (j = i; j + 1 < window_active.count; j++) {
				window_active.windows[j] = window_active.windows[j + 1];
			}
			window_active.count--;
			window_active.windows[window_active.count] = NULL;
			{
				size_t sel = 0;
				if (i != 0) {
					sel = i - 1;
				}
				window_select(window_active.windows[sel]);
			}
			window_retile();
			return true;
		}
	}
	return false;
}

bool window_next(void) {
	size_t i = 0;
	if (window_active.count <= (size_t)1) {
		return false;
	}
	for (i = 0; i < window_active.count; i++) {
		if (window_active.windows[i] == window_active.current) {
			window_select(window_active.windows[(i + 1) % window_active.count]);
			return true;
		}
	}
	return false;
}

void window_init(void) {
	Window *win = {0};
	memset(&window_active, 0, sizeof(window_active));
	win = window_create();
	if (win != NULL) {
		window_select(win);
		window_retile();
	}
}

static int windows_showing(Buffer *buf) {
	size_t i = 0;
	int n = 0;
	if (buf == NULL) {
		return 0;
	}
	for (i = 0; i < window_active.count; i++) {
		if (window_active.windows[i] != NULL && window_active.windows[i].buffer == buf) {
			n++;
		}
	}
	return n;
}

static bool text_has_nl(char *text, size_t n) {
	size_t i = 0;
	if (text == NULL) {
		return false;
	}
	for (i = 0; i < n; i++) {
		if (text[i] == '\n') {
			return true;
		}
	}
	return false;
}

/* jem.go NoteBufferEditOnWindows: structural / multi-window edits redraw; else DidEdit. */
static void note_buffer_edit(Buffer *buf, bool structural, bool was_changed) {
	size_t i = 0;
	int showing = 0;
	bool redraw = false;
	if (buf == NULL) {
		return;
	}
	showing = windows_showing(buf);
	redraw = structural || showing != 1;
	for (i = 0; i < window_active.count; i++) {
		Window *w = window_active.windows[i];
		if (w == NULL || w.buffer != buf) {
			continue;
		}
		if (redraw) {
			w.should_redraw = true;
		} else {
			w.did_edit = true;
		}
		if (!was_changed) {
			w.should_update_mode_line = true;
		}
	}
}

bool (Window *win).set_text(Location begin, Location end, char *text, size_t n) {
	Location new_end = {0};
	bool ok = false;
	bool was_changed = false;
	bool structural = false;
	if (win == NULL || win.buffer == NULL) {
		return false;
	}
	was_changed = win.buffer.is_changed;
	structural = begin.line != end.line || text_has_nl(text, n);
	ok = win.buffer.replace_raw(begin, end, text, n, &new_end);
	if (!ok) {
		return false;
	}
	win.cursor = new_end;
	win.goal_col = GoalColUnset;
	note_buffer_edit(win.buffer, structural, was_changed);
	win.ensure_visible();
	return true;
}

void window_clear_search_spans(Window *win) {
	if (win == NULL || win.search_span_count == 0) {
		return;
	}
	win.search_span_count = 0;
	win.should_redraw = true;
}

void window_set_search_spans(Window *win, SearchSpan *spans, int n) {
	int i = 0;
	if (win == NULL) {
		return;
	}
	if (n < 0) {
		n = 0;
	}
	if (n > SearchSpanMax) {
		n = SearchSpanMax;
	}
	win.search_span_count = n;
	for (i = 0; i < n; i++) {
		win.search_spans[i] = spans[i];
	}
	win.should_redraw = true;
}

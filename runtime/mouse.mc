// runtime/mouse.mc — click, drag-select, wheel viewport scroll (jem.go/runtime/mouse.go).
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>

enum {
	MouseWheelLines = 3
};

static Window *g_mouse_anchor_win;
static int g_mouse_anchor_line;
static size_t g_mouse_anchor_offset;

/* Drag anchor is ephemeral across edits — clear rather than remap. */
void mouse_after_buffer_edit(Buffer *buf) {
	if (g_mouse_anchor_win == NULL || g_mouse_anchor_win.buffer != buf) {
		return;
	}
	g_mouse_anchor_win = NULL;
	g_mouse_anchor_line = 0;
	g_mouse_anchor_offset = 0;
}

static int window_last_visible_line(Window *win) {
	if (win == NULL || win.buffer == NULL) {
		return 1;
	}
	int nlines = win.buffer.line_count_i();
	int last = win.top_line + win.height - 1;
	if (last > nlines) {
		last = nlines;
	}
	if (last < win.top_line) {
		return win.top_line;
	}
	return last;
}

static void window_set_top_line(Window *win, int line) {
	if (win == NULL || win.buffer == NULL) {
		return;
	}
	int nlines = win.buffer.line_count_i();
	if (line < 1) {
		line = 1;
	}
	if (nlines < 1) {
		nlines = 1;
	}
	if (line > nlines) {
		line = nlines;
	}
	win.top_line = line;
	win.should_redraw = true;
}

/* Scroll viewport by n lines (positive = down). Keeps cursor visible. */
static void window_scroll(Window *win, int n) {
	if (win == NULL || win.buffer == NULL || n == 0) {
		return;
	}
	int nlines = win.buffer.line_count_i();
	int line_number = win.top_line + n;
	if (line_number < 1) {
		line_number = 1;
	}
	if (line_number > nlines) {
		line_number = nlines;
	}
	window_set_top_line(win, line_number);
	if (win == window_current()) {
		int c = win.cursor.line;
		int last = window_last_visible_line(win);
		if (c < win.top_line || c > last) {
			Location loc = {0};
			if (n > 0) {
				loc.line = last;
			} else {
				loc.line = win.top_line;
			}
			loc.offset = 0;
			win.set_cursor(loc);
		}
	}
}

static void apply_wheel_ticks(int net) {
	if (net == 0) {
		return;
	}
	Window *win = display_win_at(term_mouse_row());
	if (win == NULL) {
		win = window_current();
	}
	window_scroll(win, net * MouseWheelLines);
}

// cmd_mouse_left moves point to the click; switches window if needed.
bool cmd_mouse_left(bool f, int n) {
	Location loc = {0};
	(void)f;
	(void)n;
	Window *win = display_win_at(term_mouse_row());
	if (win == NULL) {
		return true;
	}
	if (win != window_current()) {
		window_select(win);
	}
	Window *cur = window_current();
	if (cur == NULL) {
		return true;
	}
	loc = display_mouse_loc_in_window(win);
	cur.set_cursor(loc);
	cur.mark.line = 0;
	cur.mark.offset = 0;
	g_mouse_anchor_win = cur;
	g_mouse_anchor_line = loc.line;
	g_mouse_anchor_offset = loc.offset;
	cur.should_redraw = true;
	cur.should_update_mode_line = true;
	return true;
}

// cmd_mouse_drag extends the selection from the click anchor to the pointer.
bool cmd_mouse_drag(bool f, int n) {
	Location loc = {0};
	(void)f;
	(void)n;
	Window *win = display_win_at(term_mouse_row());
	Window *cur = window_current();
	if (win == NULL || cur == NULL || win != cur || g_mouse_anchor_win != cur) {
		return false;
	}
	loc = display_mouse_loc_in_window(win);
	cur.mark.line = g_mouse_anchor_line;
	cur.mark.offset = g_mouse_anchor_offset;
	cur.set_cursor(loc);
	cur.should_redraw = true;
	cur.should_update_mode_line = true;
	return true;
}

bool cmd_mouse_wheel_up(bool f, int n) {
	(void)f;
	(void)n;
	apply_wheel_ticks(-1);
	return true;
}

bool cmd_mouse_wheel_down(bool f, int n) {
	(void)f;
	(void)n;
	apply_wheel_ticks(1);
	return true;
}

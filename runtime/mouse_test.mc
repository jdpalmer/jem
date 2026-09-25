import "runtime";
import "buffer";
import "window";
import "term";
import "display";

#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

static Window *g_win;

static Window *setup_window(const char *text, int height) {
	Buffer *buf = NULL;
	buffer_list_init();
	buf = buffer_current();
	check(buf != NULL);
	check(buf.load_bytes(text, strlen(text), EModeLF));
	if (g_win == NULL) {
		g_win = window_create();
		check(g_win != NULL);
		window_select(g_win);
	} else {
		memset(&window_active, 0, sizeof(window_active));
		window_active.windows[0] = g_win;
		window_active.count = 1;
		g_win.buffer = buf;
		g_win.cursor.line = 1;
		g_win.cursor.offset = 0;
		g_win.mark.line = 0;
		g_win.mark.offset = 0;
		g_win.top_line = 1;
		g_win.h_scroll = 0;
		window_select(g_win);
	}
	g_win.height = height;
	g_win.screen_top_row = 0;
	g_win.top_line = 1;
	return g_win;
}

static void test_mouse_left_moves_point() {
	Window *win = setup_window("hello world", 10);
	int gutter = window_gutter_width(win);
	term_set_mouse_pos(gutter + 5, 0);
	check(cmd_mouse_left(false, 1));
	check(win.cursor.line == 1);
	check(win.cursor.offset == 5);
	check(win.mark.line == 0);
}

static void test_mouse_drag_selects() {
	Window *win = setup_window("hello world", 10);
	int gutter = window_gutter_width(win);
	term_set_mouse_pos(gutter + 0, 0);
	check(cmd_mouse_left(false, 1));
	term_set_mouse_pos(gutter + 5, 0);
	check(cmd_mouse_drag(false, 1));
	check(win.mark.line == 1);
	check(win.mark.offset == 0);
	check(win.cursor.line == 1);
	check(win.cursor.offset == 5);
}

static void test_mouse_wheel_scrolls_viewport() {
	Window *win = NULL;
	char text[512] = {0};
	int i = 0;
	int off = 0;
	for (i = 0; i < 40; i++) {
		off += snprintf(text + off, sizeof(text) - (size_t)off, "line %d\n", i + 1);
	}
	win = setup_window(text, 10);
	check(win.top_line == 1);
	term_set_mouse_pos(0, 0);
	check(cmd_mouse_wheel_down(false, 1));
	check(win.top_line == 1 + 3);
	check(cmd_mouse_wheel_up(false, 1));
	check(win.top_line == 1);
}

static void test_mouse_click_other_window() {
	Buffer *buf = NULL;
	Window *w2 = NULL;
	buffer_list_init();
	buf = buffer_current();
	check(buf.load_bytes("aaa\nbbb\nccc", strlen("aaa\nbbb\nccc"), EModeLF));
	check(g_win != NULL);
	memset(&window_active, 0, sizeof(window_active));
	w2 = window_create();
	check(w2 != NULL);
	window_active.windows[0] = g_win;
	window_active.windows[1] = w2;
	window_active.count = 2;
	g_win.screen_top_row = 0;
	g_win.height = 5;
	g_win.buffer = buf;
	w2.screen_top_row = 6;
	w2.height = 5;
	w2.buffer = buf;
	window_select(g_win);
	term_set_mouse_pos(window_gutter_width(w2), 7);
	check(cmd_mouse_left(false, 1));
	check(window_current() == w2);
}

int main() {
	test_mouse_left_moves_point();
	test_mouse_drag_selects();
	test_mouse_wheel_scrolls_viewport();
	test_mouse_click_other_window();
	return 0;
}

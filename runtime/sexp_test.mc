import "runtime";
import "buffer";
import "window";
import "mode";

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

static Window *g_win;

static Window *setup(const char *text, LangMode lang, int line, size_t offset) {
	Buffer *buf = NULL;
	Location loc = {0};
	buffer_list_init();
	buf = buffer_current();
	check(buf != NULL);
	check(buf.load_bytes(text, strlen(text), EModeLF));
	buf.lang_mode = lang;
	if (g_win == NULL) {
		g_win = window_create();
		check(g_win != NULL);
		window_select(g_win);
	} else {
		memset(&window_active, 0, sizeof(window_active));
		window_active.windows[0] = g_win;
		window_active.count = 1;
		g_win.buffer = buf;
		window_select(g_win);
	}
	loc.line = line;
	loc.offset = offset;
	g_win.set_cursor(loc);
	g_win.height = 20;
	g_win.screen_top_row = 0;
	g_win.top_line = 1;
	return g_win;
}

static void test_forward_sexp_paren() {
	Window *win = setup("(abc)", LModeC, 1, 0);
	check(cmd_forward_sexp(false, 1));
	check(win.cursor.offset == 5);
}

static void test_backward_sexp_paren() {
	Window *win = setup("(abc)", LModeC, 1, 5);
	check(cmd_backward_sexp(false, 1));
	check(win.cursor.offset == 0);
}

static void test_forward_sexp_word() {
	Window *win = setup("foo bar", LModeC, 1, 0);
	check(cmd_forward_sexp(false, 1));
	check(win.cursor.offset == 3);
}

static void test_forward_sexp_repeat() {
	Window *win = setup("(a) (b)", LModeC, 1, 0);
	check(cmd_forward_sexp(false, 2));
	check(win.cursor.offset == 7);
}

int main() {
	test_forward_sexp_paren();
	test_backward_sexp_paren();
	test_forward_sexp_word();
	test_forward_sexp_repeat();
	return 0;
}

import "runtime";
import "buffer";
import "window";
import "display";
import "search";
import "minibuffer";

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

static Window *g_win;

static Window *setup(const char *text) {
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
		window_select(g_win);
	}
	search_scope_set(0);
	search_set_last_pattern("");
	return g_win;
}

static void test_toggle_search_scope() {
	(void)setup("x");
	check(cmd_toggle_search_scope(false, 1));
	check(search_scope_is_all());
	check(strstr(display_echo_text(), "all buffers") != NULL);
	check(cmd_toggle_search_scope(false, 1));
	check(!search_scope_is_all());
	check(strstr(display_echo_text(), "current buffer") != NULL);
}

static void test_prompted_search() {
	Window *win = setup("aaa bbb aaa");
	Location loc = {0};
	loc.line = 1;
	loc.offset = 0;
	win.set_cursor(loc);
	check(cmd_run_prompted_search(true, "bbb"));
	check(win.cursor.offset == 7);
	check(strcmp(search_last_pattern(), "bbb") == 0);
	check(cmd_run_prompted_search(false, "aaa"));
	check(win.cursor.offset == 0);
	check(cmd_run_prompted_search(true, "zzz"));
	check(strstr(display_echo_text(), "not found") != NULL);
}

static void test_search_forward_prompt() {
	char prompt[64] = {0};
	(void)setup("hello");
	search_scope_set(0);
	search_format_prompt(prompt, sizeof(prompt), "Search");
	check(strcmp(prompt, "Search: ") == 0);
	search_scope_set(1);
	search_format_prompt(prompt, sizeof(prompt), "Search");
	check(strcmp(prompt, "Search [all]: ") == 0);
	search_format_prompt(prompt, sizeof(prompt), "Reverse search");
	check(strcmp(prompt, "Reverse search [all]: ") == 0);
	search_format_isearch_prompt(prompt, sizeof(prompt), "I-search",
		buffer_current());
	check(strstr(prompt, "[all ") != NULL);
	search_scope_set(0);
	search_set_last_pattern("ell");
	check(cmd_search_forward(false, 1));
	check(minibuffer_active());
	minibuffer_cancel();
	pop_listener();
}

int main() {
	test_toggle_search_scope();
	test_prompted_search();
	test_search_forward_prompt();
	return 0;
}

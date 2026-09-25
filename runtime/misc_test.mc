import "runtime";
import "buffer";
import "window";
import "display";
import "term";

#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

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
		g_win.mark.line = 0;
		g_win.mark.offset = 0;
		g_win.top_line = 1;
		g_win.h_scroll = 0;
		g_win.screen_top_row = 0;
		g_win.height = 10;
		window_select(g_win);
	}
	g_win.height = 10;
	g_win.screen_top_row = 0;
	return g_win;
}

static void expect_contains(const char *hay, const char *needle) {
	if (hay == NULL || needle == NULL || strstr(hay, needle) == NULL) {
		abort();
	}
}

static void test_insert_date() {
	Window *win = setup("");
	check(cmd_insert_date(false, 1));
	{
		auto (got, got_len, ok) = win.buffer.save_bytes();
		check(ok);
		check(got_len >= 10);
		check(got[0] >= 'A' && got[0] <= 'Z');
		check(strstr(got, ", 20") != NULL || strstr(got, ", 19") != NULL);
		free(got);
	}
}

static void test_show_position() {
	Window *win = setup("hello");
	Location loc = {0};
	loc.line = 1;
	loc.offset = 1;
	win.set_cursor(loc);
	check(cmd_show_position(false, 1));
	expect_contains(display_echo_text(), "X=");
	expect_contains(display_echo_text(), "CH=0x");
	expect_contains(display_echo_text(), "of ");
}

static void test_set_eol_mode() {
	Window *win = setup("a\nb");
	Buffer *buf = win.buffer;
	buf.set_name("eoltest");
	check(buf.eol_mode == EModeLF);
	check(cmd_set_eol_mode(false, 1));
	check(cmd_choose_handle_key('c'));
	check(buf.eol_mode == EModeCRLF);
	check(buf.is_changed);
	expect_contains(display_echo_text(), "CRLF");
	check(cmd_set_eol_mode(false, 1));
	check(cmd_choose_handle_key('l'));
	check(buf.eol_mode == EModeLF);
	expect_contains(display_echo_text(), "LF");
}

static void test_kill_buffer_fuzzy() {
	Buffer *other = NULL;
	size_t before = 0;
	int i = 0;
	Buffer *sel = NULL;
	(void)setup("x");
	other = buffer_new();
	check(other != NULL);
	other.set_name("other");
	check(buffer_list_add(other));
	before = len(buffer_all);
	check(before >= 2);
	check(cmd_kill_buffer_fuzzy(false, 1));
	for (i = 0; i < 8; i++) {
		sel = fuzzy_buffer_selected();
		if (sel != NULL && !sel.is_scratch()) {
			break;
		}
		(void)fuzzy_buffer_handle_key(term_key_value(TermKeyDown));
	}
	sel = fuzzy_buffer_selected();
	check(sel != NULL && !sel.is_scratch());
	check(cmd_kill_buffer_fuzzy_on_accept());
	misc_kill_pending_yes();
	check(len(buffer_all) == before - 1);
	check(buffer_find_by_name("other") == NULL);
}

int main() {
	test_insert_date();
	test_show_position();
	test_set_eol_mode();
	test_kill_buffer_fuzzy();
	return 0;
}

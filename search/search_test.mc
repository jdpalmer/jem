import "search";
import "buffer";
import "window";

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

static void test_literal_replace() {
	Buffer *buf = NULL;
	Buffer *undone = NULL;
	Window *win = NULL;
	Location cursor = {0};
	char original[] = "a a";
	char needle[] = "a";
	char replacement[] = "aa";

	buffer_list_init();
	buf = buffer_current();
	check(buf != NULL);
	check(buf.load_bytes(original, strlen(original), EModeLF));
	win = window_create();
	check(win != NULL);
	window_select(win);

	search_begin_replace();
	check(search_set_needle(needle));
	check(search_set_replacement(replacement));
	check(search_update());
	check(search_replace_one());
	check(search_replace_one());
	check(!search_replace_one());
	auto (saved, saved_len, saved_ok) = buf.save_bytes();
	check(saved_ok);
	check(saved_len == 5);
	check(memcmp(saved, "aa aa", 5) == 0);
	free(saved);

	check(buffer_undo.undo_once(&cursor, &undone));
	check(buffer_undo.undo_once(&cursor, &undone));
	auto (restored, restored_len, restored_ok) = buf.save_bytes();
	check(restored_ok);
	check(restored_len == strlen(original));
	check(memcmp(restored, original, restored_len) == 0);
	free(restored);
	search_end();
}

static void test_regex_forward() {
	Buffer *buf = NULL;
	Window *win = NULL;
	Location loc = {0};
	char text[] = "xxaaayy";
	char pat[] = "a+";

	buffer_list_init();
	buf = buffer_current();
	check(buf.load_bytes(text, strlen(text), EModeLF));
	win = window_create();
	check(win != NULL);
	window_select(win);
	loc.line = 1;
	loc.offset = 0;
	win.set_cursor(loc);

	search_begin(true, true);
	check(search_regexp());
	check(search_set_needle(pat));
	check(search_update());
	check(search_state().have_match);
	check(search_state().match_begin.line == 1);
	check(search_state().match_begin.offset == 2);
	check(search_state().match_end.offset == 5);
	search_end();
}

static void test_regex_invalid() {
	Buffer *buf = NULL;
	Window *win = NULL;
	char text[] = "abc";
	char pat[] = "[";

	buffer_list_init();
	buf = buffer_current();
	check(buf.load_bytes(text, strlen(text), EModeLF));
	win = window_create();
	window_select(win);

	search_begin(true, true);
	check(search_set_needle(pat));
	check(!search_update());
	check(search_last_error()[0] != 0);
	search_end();
}

static void test_regex_zero_length() {
	Buffer *buf = NULL;
	Window *win = NULL;
	Location loc = {0};
	char text[] = "abc";
	char pat[] = "a*";

	buffer_list_init();
	buf = buffer_current();
	check(buf.load_bytes(text, strlen(text), EModeLF));
	win = window_create();
	window_select(win);
	loc.line = 1;
	loc.offset = 1;
	win.set_cursor(loc);
	search_begin(true, true);
	check(search_set_needle(pat));
	check(!search_update());
	check(strstr(search_last_error(), "zero-length") != NULL);
	search_end();
}

static void test_regex_capture_replace() {
	Buffer *buf = NULL;
	Window *win = NULL;
	char text[] = "ab";
	char pat[] = "(a)(b)";
	char repl[] = "\\2\\1";

	buffer_list_init();
	buf = buffer_current();
	check(buf.load_bytes(text, strlen(text), EModeLF));
	win = window_create();
	window_select(win);

	search_begin_replace_re();
	check(search_set_needle(pat));
	check(search_set_replacement(repl));
	check(search_update());
	check(search_replace_one());
	auto (saved, saved_len, saved_ok) = buf.save_bytes();
	check(saved_ok);
	check(saved_len == 2);
	check(memcmp(saved, "ba", 2) == 0);
	free(saved);
	search_end();
}

static void test_regex_dot_newline() {
	Buffer *buf = NULL;
	Window *win = NULL;
	Location loc = {0};
	char text[] = "a\nb";
	char pat[] = "a.b";

	buffer_list_init();
	buf = buffer_current();
	check(buf.load_bytes(text, strlen(text), EModeLF));
	win = window_create();
	window_select(win);
	loc.line = 1;
	loc.offset = 0;
	win.set_cursor(loc);

	search_begin(true, true);
	check(search_set_needle(pat));
	check(search_update());
	check(search_state().have_match);
	check(search_state().match_begin.line == 1);
	check(search_state().match_begin.offset == 0);
	check(search_state().match_end.line == 2);
	check(search_state().match_end.offset == 1);
	search_end();
}

int main() {
	test_literal_replace();
	test_regex_forward();
	test_regex_invalid();
	test_regex_zero_length();
	test_regex_capture_replace();
	test_regex_dot_newline();
	return 0;
}

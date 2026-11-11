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
	char original[] = "a a";
	char needle[] = "a";
	char replacement[] = "aa";
	Location cursor = {0};
	Buffer *undone = NULL;

	buffer_list_init();
	Buffer *buf = buffer_current();
	check(buf != NULL);
	check(buf.load_bytes(original, strlen(original), EModeLF));
	Window *win = window_create();
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
	char text[] = "xxaaayy";
	char pat[] = "a+";

	buffer_list_init();
	Buffer *buf = buffer_current();
	check(buf.load_bytes(text, strlen(text), EModeLF));
	Window *win = window_create();
	check(win != NULL);
	window_select(win);
	Location loc = { .line = 1, .offset = 0 };
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
	char text[] = "abc";
	char pat[] = "[";

	buffer_list_init();
	Buffer *buf = buffer_current();
	check(buf.load_bytes(text, strlen(text), EModeLF));
	Window *win = window_create();
	window_select(win);

	search_begin(true, true);
	check(search_set_needle(pat));
	check(!search_update());
	check(*search_last_error() != 0);
	search_end();
}

static void test_regex_zero_length() {
	char text[] = "abc";
	char pat[] = "a*";

	buffer_list_init();
	Buffer *buf = buffer_current();
	check(buf.load_bytes(text, strlen(text), EModeLF));
	Window *win = window_create();
	window_select(win);
	Location loc = { .line = 1, .offset = 1 };
	win.set_cursor(loc);
	search_begin(true, true);
	check(search_set_needle(pat));
	check(!search_update());
	check(strstr(search_last_error(), "zero-length") != NULL);
	search_end();
}

static void test_regex_capture_replace() {
	char text[] = "ab";
	char pat[] = "(a)(b)";
	char repl[] = "\\2\\1";

	buffer_list_init();
	Buffer *buf = buffer_current();
	check(buf.load_bytes(text, strlen(text), EModeLF));
	Window *win = window_create();
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
	char text[] = "a\nb";
	char pat[] = "a.b";

	buffer_list_init();
	Buffer *buf = buffer_current();
	check(buf.load_bytes(text, strlen(text), EModeLF));
	Window *win = window_create();
	window_select(win);
	Location loc = { .line = 1, .offset = 0 };
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

static void test_plain_find_and_scope() {
	buffer_list_init();
	Buffer *buf = buffer_current();
	check(buf.load_bytes("hello world", strlen("hello world"), EModeLF));
	Window *win = window_create();
	check(win != NULL);
	window_select(win);
	Location loc = { .line = 1, .offset = 0 };
	win.set_cursor(loc);

	search_scope_set(0);
	search_set_last_pattern("world");
	check(search_plain_find(true));
	check(win.cursor.offset == 11);

	loc.offset = 0;
	win.set_cursor(loc);
	search_set_last_pattern("missing");
	check(!search_plain_find(true));

	Buffer *other = buffer_new();
	check(other != NULL);
	other.set_name("other");
	check(other.load_bytes("zzz target zzz", strlen("zzz target zzz"), EModeLF));
	check(buffer_list_add(other));

	search_scope_set(0);
	search_set_last_pattern("target");
	check(!search_plain_find(true));

	search_scope_set(1);
	check(search_plain_find(true));
	check(window_current().buffer == other);

	check(search_toggle_scope() == false);
	check(search_scope_get() == 0);
	check(search_toggle_scope() == true);
	check(search_scope_is_all());
	search_scope_set(0);
}

int main() {
	test_literal_replace();
	test_regex_forward();
	test_regex_invalid();
	test_regex_zero_length();
	test_regex_capture_replace();
	test_regex_dot_newline();
	test_plain_find_and_scope();
	return 0;
}

import "runtime";
import "buffer";
import "window";

#include <stdio.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

static void expect_text(Buffer *buf, const char *want) {
	auto (saved, saved_len, saved_ok) = buf.save_bytes();
	check(saved_ok);
	if (saved_len != strlen(want) || memcmp(saved, want, saved_len) != 0) {
		fprintf(stderr, "expect fail got=[%.*s] want=[%s]\n", (int)saved_len, saved, want);
		abort();
	}
	free(saved);
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
		g_win.buffer = buf;
		g_win.cursor.line = 1;
		g_win.cursor.offset = 0;
		g_win.mark.line = 0;
		g_win.mark.offset = 0;
		window_select(g_win);
	}
	return g_win;
}

static void test_delete_blank_lines() {
	Window *win = setup("a\n\n\nb");
	Location loc = {0};
	loc.line = 1;
	loc.offset = 0;
	win.set_cursor(loc);
	check(cmd_delete_blank_lines(false, 1));
	expect_text(win.buffer, "a\nb");
}

static void test_delete_blank_lines_on_blank() {
	Window *win = setup("a\n\n\n\nb");
	Location loc = {0};
	loc.line = 3;
	loc.offset = 0;
	win.set_cursor(loc);
	check(cmd_delete_blank_lines(false, 1));
	expect_text(win.buffer, "a\n\nb");
}

static void test_transpose_lines() {
	Window *win = setup("one\ntwo\nthree");
	Location loc = {0};
	loc.line = 2;
	loc.offset = 0;
	win.set_cursor(loc);
	check(cmd_transpose_lines(false, 1));
	expect_text(win.buffer, "two\none\nthree");
	check(win.cursor.line == 1);
}

static void test_region_case() {
	Window *win = setup("Hello World");
	Location loc = {0};
	win.mark.line = 1;
	win.mark.offset = 0;
	loc.line = 1;
	loc.offset = 11;
	win.set_cursor(loc);
	check(cmd_upper_region(false, 1));
	expect_text(win.buffer, "HELLO WORLD");
	win.mark.line = 1;
	win.mark.offset = 0;
	loc.offset = 11;
	win.set_cursor(loc);
	check(cmd_lower_region(false, 1));
	expect_text(win.buffer, "hello world");
}

static void test_sort_region() {
	Window *win = setup("c\na\nb\n");
	Location loc = {0};
	win.mark.line = 1;
	win.mark.offset = 0;
	loc.line = 3;
	loc.offset = 1;
	win.set_cursor(loc);
	check(cmd_sort_region(false, 1));
	expect_text(win.buffer, "a\nb\nc\n");
}

static void test_snake_case() {
	Window *win = setup("fooBar baz");
	Location loc = {0};
	loc.line = 1;
	loc.offset = 3;
	win.set_cursor(loc);
	check(cmd_snake_case(false, 1));
	expect_text(win.buffer, "foo_bar baz");
}

static void test_pascal_case() {
	Window *win = setup("foo_bar");
	Location loc = {0};
	loc.line = 1;
	loc.offset = 0;
	win.set_cursor(loc);
	check(cmd_pascal_case(false, 1));
	expect_text(win.buffer, "FooBar");
}

static void test_camel_from_xml() {
	Window *win = setup("XMLHttpRequest");
	Location loc = {0};
	loc.line = 1;
	loc.offset = 0;
	win.set_cursor(loc);
	check(cmd_camel_case(false, 1));
	expect_text(win.buffer, "xmlHttpRequest");
}

static void test_constant_case() {
	Window *win = setup("fooBar");
	Location loc = {0};
	loc.line = 1;
	loc.offset = 0;
	win.set_cursor(loc);
	check(cmd_constant_case(false, 1));
	expect_text(win.buffer, "FOO_BAR");
}

int main() {
	test_delete_blank_lines();
	test_delete_blank_lines_on_blank();
	test_transpose_lines();
	test_region_case();
	test_sort_region();
	test_snake_case();
	test_pascal_case();
	test_camel_from_xml();
	test_constant_case();
	return 0;
}

import "mode";
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

static void expect_text(Buffer *buf, const char *want) {
	auto (saved, saved_len, saved_ok) = buf.save_bytes();
	check(saved_ok);
	check(saved_len == strlen(want));
	check(memcmp(saved, want, saved_len) == 0);
	free(saved);
}

static void test_comment_dwim_line() {
	Buffer *buf = NULL;
	Window *win = NULL;
	Location loc = {0};

	buffer_list_init();
	buf = buffer_current();
	check(buf != NULL);
	buf.lang_mode = LModeGo;
	check(buf.load_bytes("\tx := 1", strlen("\tx := 1"), EModeLF));
	win = window_create();
	check(win != NULL);
	window_select(win);
	loc.line = 1;
	loc.offset = 1;
	win.set_cursor(loc);
	win.mark.line = 0;
	win.mark.offset = 0;

	check(mode_comment_dwim(false, 1));
	expect_text(buf, "\t// x := 1");
	check(mode_comment_dwim(false, 1));
	expect_text(buf, "\tx := 1");
}

static void test_comment_dwim_python() {
	Buffer *buf = NULL;
	Window *win = NULL;
	Location loc = {0};

	buffer_list_init();
	buf = buffer_current();
	buf.lang_mode = LModePython;
	check(buf.load_bytes("    x = 1", strlen("    x = 1"), EModeLF));
	win = window_create();
	window_select(win);
	loc.line = 1;
	loc.offset = 4;
	win.set_cursor(loc);
	win.mark.line = 0;

	check(mode_comment_dwim(false, 1));
	expect_text(buf, "    # x = 1");
	check(mode_comment_dwim(false, 1));
	expect_text(buf, "    x = 1");
}

static void test_comment_dwim_unsupported() {
	Buffer *buf = NULL;
	Window *win = NULL;
	Location loc = {0};

	buffer_list_init();
	buf = buffer_current();
	buf.lang_mode = LModeCSS;
	check(buf.load_bytes("color: red;", strlen("color: red;"), EModeLF));
	win = window_create();
	window_select(win);
	loc.line = 1;
	loc.offset = 0;
	win.set_cursor(loc);
	win.mark.line = 0;

	check(!mode_comment_dwim(false, 1));
	expect_text(buf, "color: red;");
}

static void test_comment_dwim_region() {
	Buffer *buf = NULL;
	Window *win = NULL;
	Location loc = {0};

	buffer_list_init();
	buf = buffer_current();
	buf.lang_mode = LModeGo;
	check(buf.load_bytes("a := 1\nb := 2\nc := 3", strlen("a := 1\nb := 2\nc := 3"), EModeLF));
	win = window_create();
	window_select(win);
	win.mark.line = 1;
	win.mark.offset = 0;
	loc.line = 2;
	loc.offset = 0;
	win.set_cursor(loc);

	check(mode_comment_dwim(false, 1));
	expect_text(buf, "// a := 1\n// b := 2\nc := 3");

	win.mark.line = 1;
	win.mark.offset = 0;
	loc.line = 2;
	loc.offset = 0;
	win.set_cursor(loc);
	check(mode_comment_dwim(false, 1));
	expect_text(buf, "a := 1\nb := 2\nc := 3");
}

static void test_fill_paragraph() {
	Buffer *buf = NULL;
	Window *win = NULL;
	Location loc = {0};
	char src[] = "one two three four five six seven eight nine ten eleven twelve";

	buffer_list_init();
	buf = buffer_current();
	buf.fill_col = 20;
	check(buf.load_bytes(src, strlen(src), EModeLF));
	win = window_create();
	window_select(win);
	loc.line = 1;
	loc.offset = 0;
	win.set_cursor(loc);

	check(mode_fill_paragraph(false, 1));
	check(buf.line_count_i() >= 2);
	{
		Line *line1 = buf.line(1);
		check(line1 != NULL);
		check(line1.byte_len() <= 20);
	}
}

static void test_line_comment_prefix() {
	check(strcmp(mode_line_comment(LModeGo), "//") == 0);
	check(strcmp(mode_line_comment(LModePython), "#") == 0);
	check(strcmp(mode_line_comment(LModeLisp), ";") == 0);
	check(strcmp(mode_line_comment(LModeLua), "--") == 0);
	check(mode_line_comment(LModeCSS)[0] == 0);
}

static void test_goto_match() {
	Buffer *buf = NULL;
	Window *win = NULL;
	Location loc = {0};
	Location match = {0};

	buffer_list_init();
	buf = buffer_current();
	check(buf.load_bytes("{ }", strlen("{ }"), EModeLF));
	buf.lang_mode = LModeC;
	win = window_create();
	window_select(win);
	loc.line = 1;
	loc.offset = 0;
	win.set_cursor(loc);
	check(mode_find_matching_delimiter(buf, loc, &match));
	check(match.line == 1 && match.offset == 2);
	check(mode_goto_match(false, 1));
	check(win.cursor.offset == 2);
}

static void test_c_top_end_function() {
	Buffer *buf = NULL;
	Window *win = NULL;
	Location loc = {0};
	const char *src = "int f(void)\n{\n  return 1;\n}\n";

	buffer_list_init();
	buf = buffer_current();
	check(buf.load_bytes(src, strlen(src), EModeLF));
	buf.lang_mode = LModeC;
	win = window_create();
	check(win != NULL);
	window_select(win);
	loc.line = 3;
	loc.offset = 2;
	win.set_cursor(loc);
	check(mode_end_of_function(false, 1));
	check(win.cursor.line == 4);
	check(win.cursor.offset == 0);
	loc.line = 3;
	loc.offset = 2;
	win.set_cursor(loc);
	check(mode_top_of_function(false, 1));
	check(win.cursor.line == 1);
}

static void test_py_top_end_function() {
	Buffer *buf = NULL;
	Window *win = NULL;
	Location loc = {0};
	const char *src = "def f():\n  a = 1\n  b = 2\nx = 3\n";

	buffer_list_init();
	buf = buffer_current();
	check(buf.load_bytes(src, strlen(src), EModeLF));
	buf.lang_mode = LModePython;
	win = window_create();
	window_select(win);
	loc.line = 2;
	loc.offset = 0;
	win.set_cursor(loc);
	check(mode_top_of_function(false, 1));
	check(win.cursor.line == 1);
	check(mode_end_of_function(false, 1));
	check(win.cursor.line == 3);
}

int main() {
	test_line_comment_prefix();
	test_comment_dwim_line();
	test_comment_dwim_python();
	test_comment_dwim_unsupported();
	test_comment_dwim_region();
	test_fill_paragraph();
	test_goto_match();
	test_c_top_end_function();
	test_py_top_end_function();
	return 0;
}

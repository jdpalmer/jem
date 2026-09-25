import "runtime";
import "buffer";
import "window";
import "display";

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

static void setup() {
	Buffer *buf = NULL;
	vars_init();
	buffer_list_init();
	buf = buffer_current();
	check(buf != NULL);
	check(buf.load_bytes("hello", strlen("hello"), EModeLF));
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
}

static void test_vars_fill_column() {
	Buffer *buf = NULL;
	setup();
	buf = buffer_current();
	check(vars_count() >= 5);
	check(vars_set_named("fill-column", 72));
	check(vars_get_named("fill-column") == 72);
	check(buf.fill_col == 72);
	check(!vars_set_named("fill-column", -1));
	check(!vars_set_named("fill-column", 1001));
}

static void test_vars_theme_and_auto_indent() {
	setup();
	check(vars_set_named("theme-mode", 1));
	check(display_theme_mode() == 1);
	check(vars_get_named("theme-mode") == 1);
	check(vars_set_named("theme-mode", 0));
	check(display_theme_mode() == 0);
	check(vars_set_named("auto-indent", 0));
	check(vars_get_named("auto-indent") == 0);
	check(vars_set_named("auto-indent", 1));
	check(vars_get_named("auto-indent") == 1);
}

static void test_vars_indent() {
	Buffer *buf = NULL;
	setup();
	buf = buffer_current();
	check(vars_set_named("indent-width", 4));
	check(buf.indent.width == 4);
	check(vars_set_named("indent-continued", 8));
	check(buf.indent.continued == 8);
}

static void test_set_variable_value_text() {
	setup();
	check(cmd_set_variable_value_from_text("fill-column", "90"));
	check(vars_get_named("fill-column") == 90);
	check(!cmd_set_variable_value_from_text("fill-column", "nope"));
	check(!cmd_set_variable_value_from_text("no-such-var", "1"));
}

static void test_describe_command_entry() {
	Command *c = NULL;
	/* init_commands not run in test — describe uses g_cmds from package;
	   register locally via find after vars-only. Manually check find on empty. */
	(void)c;
	check(find_cmd_entry("nope") == NULL);
}

int main() {
	test_vars_fill_column();
	test_vars_theme_and_auto_indent();
	test_vars_indent();
	test_set_variable_value_text();
	test_describe_command_entry();
	return 0;
}

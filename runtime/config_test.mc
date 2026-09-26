import "runtime";
import "buffer";
import "window";
import "display";
import "term";

#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

static Window *g_win;
static bool g_cmds_ready;

static void setup() {
	Buffer *buf = NULL;
	vars_init();
	buffer_list_init();
	buf = buffer_current();
	check(buf != NULL);
	check(buf.load_bytes("x", 1, EModeLF));
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
	if (!g_cmds_ready) {
		init_commands();
		g_cmds_ready = true;
	}
}

static void test_parse_keys() {
	{
		auto (k, ok) = config_parse_key_sequence("C-f");
		check(ok);
		check(k == (term_key_value(TermCTL) | (uint32_t)'F'));
	}
	{
		auto (k, ok) = config_parse_key_sequence("M-x");
		check(ok);
		check(k == (term_key_value(TermMETA) | (uint32_t)'X'));
	}
	{
		auto (k, ok) = config_parse_key_sequence("C-x C-f");
		check(ok);
		check(k == (term_key_value(TermCTLX) | term_key_value(TermCTL) |
			(uint32_t)'F'));
	}
	{
		auto (k, ok) = config_parse_key_sequence("S-TAB");
		check(ok);
		check(k == (term_key_value(TermSHIFT) | term_key_value(TermKeyTab)));
	}
	{
		auto (k, ok) = config_parse_key_sequence("");
		check(!ok);
		(void)k;
	}
}

static void test_vars_from_json() {
	char json[] =
		"{\n"
		"  \"fill-column\": 100,\n"
		"  \"theme-mode\": 1,\n"
		"  \"search-scope\": \"1\",\n"
		"  \"auto-revert-mode\": 0\n"
		"}\n";
	setup();
	check(config_apply_json(json, strlen(json)));
	check(vars_get_named("fill-column") == 100);
	check(vars_get_named("theme-mode") == 1);
	check(vars_get_named("search-scope") == 1);
	check(vars_get_named("auto-revert-mode") == 0);
}

static void test_legacy_theme_and_keybindings() {
	char json[] =
		"{\n"
		"  \"theme\": \"light\",\n"
		"  \"keybindings\": {\n"
		"    \"C-x f\": \"set-variable\",\n"
		"    \"C-l\": null\n"
		"  }\n"
		"}\n";
	uint32_t cx_f = 0;
	uint32_t c_l = 0;
	CommandFn set_var = NULL;
	setup();
	check(config_apply_json(json, strlen(json)));
	check(vars_get_named("theme-mode") == 1);
	{
		auto (k, ok) = config_parse_key_sequence("C-x f");
		check(ok);
		cx_f = k;
	}
	{
		auto (k, ok) = config_parse_key_sequence("C-l");
		check(ok);
		c_l = k;
	}
	set_var = runtime_find_cmd("set-variable");
	check(set_var != NULL);
	check(runtime_find_bind(cx_f) == set_var);
	check(runtime_find_bind(c_l) == NULL);
}

static void test_invalid_json() {
	char bad[] = "{ \"fill-column\": ";
	setup();
	check(!config_apply_json(bad, strlen(bad)));
}

int main() {
	test_parse_keys();
	test_vars_from_json();
	test_legacy_theme_and_keybindings();
	test_invalid_json();
	return 0;
}

// fnpick_test.mc — fuzzy filename picker smoke tests (no TTY).
import "fnpick";
import "file";
import "display";
import "term";
import "minibuffer";
import "window";
import "buffer";
import "fs";
import "path";
import "str";

#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>
#include <stdint.h>
#include <stdbool.h>
#include <sys/stat.h>

static void check(bool ok) {
	if (!ok) {
		abort();
	}
}

static Window *g_win;

static Window *setup_window(Buffer *buf) {
	if (g_win == NULL) {
		g_win = window_create();
		check(g_win != NULL);
	}
	memset(&window_active, 0, sizeof(window_active));
	window_active.windows[0] = g_win;
	window_active.count = 1;
	g_win.buffer = buf;
	g_win.cursor.line = 1;
	g_win.cursor.offset = 0;
	window_select(g_win);
	return g_win;
}

static void write_path(char *path, const char *text) {
	auto (w, wok) = writer_open(str_from_cstr(path), OpenWrite | OpenCreate | OpenTrunc, 0);
	check(wok);
	(void)w.write(str_from_cstr(text));
	w.close();
}

static void test_parent_from_cwd() {
	char out[512] = {0};
	check(file_apply_filename_selection(str_empty(), "../", out, sizeof(out)));
	check(out[0] != 0);
	check(out[strlen(out) - 1] == '/');
}

static void test_open_tab_parent_and_accept_file() {
	char dir[512] = {0};
	char path[512] = {0};
	char oldcwd[512] = {0};
	Buffer *buf = buffer_new();

	check(buf != NULL);
	setup_window(buf);
	check(getcwd(oldcwd, sizeof(oldcwd)) != NULL);

	pool p {
		auto (d, ok) = fs_mkdir_temp(&p, "", "jem-fnpick-");
		check(ok && len(d) > 0 && len(d) < sizeof(dir));
		snprintf(dir, sizeof(dir), "%.*s", (int)len(d), ptr(d));
	}
	snprintf(path, sizeof(path), "%s/alpha.txt", dir);
	write_path(path, "hi\n");
	check(chdir(dir) == 0);

	fuzzy_filename_open("Find file: ", str_empty());
	check(fuzzy_filename_active());

	/* Tab on ../ should climb out of the temp dir. */
	check(fuzzy_filename_handle_key(term_key_value(TermKeyTab)) == FuzzyContinue);
	{
		char *text = minibuffer_text();
		check(text != NULL);
		check(strcmp(text, dir) != 0);
		check(strlen(text) > 0);
	}

	/* Re-open in temp dir and accept alpha.txt via filter + Enter. */
	fuzzy_filename_close();
	memset(&window_active, 0, sizeof(window_active));
	window_active.windows[0] = g_win;
	window_active.count = 1;
	window_select(g_win);

	fuzzy_filename_open("Find file: ", str_empty());
	check(fuzzy_filename_handle_key('a') == FuzzyContinue);
	check(fuzzy_filename_handle_key('l') == FuzzyContinue);
	check(fuzzy_filename_handle_key(term_key_value(TermKeyEnter)) == FuzzyAccept);
	{
		char *text = fuzzy_filename_text();
		check(text != NULL);
		check(strstr(text, "alpha.txt") != NULL);
	}
	/* Close must not corrupt the main window (prior hide-match bug). */
	fuzzy_filename_close();
	check(window_active.count >= 1);
	check(window_active.windows[0] == g_win);
	check(window_current() == g_win);

	check(chdir(oldcwd) == 0);
}

void main() {
	test_parent_from_cwd();
	test_open_tab_parent_and_accept_file();
	printf("ok\n");
}

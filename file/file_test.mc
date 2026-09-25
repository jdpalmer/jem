import "file";
import "buffer";
import "window";
import "display";
import "fs";
import "os";
import "path";
import "str";

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include "temp.h"

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

static void test_complete_path() {
	char dir[512] = {0};
	char path[512] = {0};
	char buf[512] = {0};
	int n = 0;

	n = snprintf(dir, sizeof(dir), "/tmp/jem-fcomp-XXXXXX");
	check(n > 0);
	check(mkdtemp(dir) != NULL);

	snprintf(path, sizeof(path), "%s/alpha.txt", dir);
	write_path(path, "hi");
	snprintf(path, sizeof(path), "%s/alphabet.go", dir);
	write_path(path, "");
	snprintf(path, sizeof(path), "%s/subdir", dir);
	check(fs_mkdir(str_from_cstr(path)));

	snprintf(buf, sizeof(buf), "%s/alp", dir);
	check(file_complete_path(buf, sizeof(buf)));
	check(strstr(buf, "/alpha") != NULL);

	snprintf(buf, sizeof(buf), "%s/sub", dir);
	check(file_complete_path(buf, sizeof(buf)));
	check(strlen(buf) > 0 && buf[strlen(buf) - 1] == '/');

	snprintf(buf, sizeof(buf), "~");
	check(file_complete_path(buf, sizeof(buf)));
	check(strcmp(buf, "~/") == 0);
}

static void test_check_reload_clean_buffer() {
	char dir[512] = {0};
	char path[512] = {0};
	Buffer *buf = NULL;
	Window *win = NULL;
	Line *line = NULL;
	int ask_line = 0;

	snprintf(dir, sizeof(dir), "/tmp/jem-arevert-XXXXXX");
	check(mkdtemp(dir) != NULL);
	snprintf(path, sizeof(path), "%s/watch.txt", dir);
	write_path(path, "first\n");

	buffer_list_init();
	{
		auto (loaded, ok) = file_load(path);
		check(ok && loaded != NULL);
		buf = loaded;
	}
	win = setup_window(buf);
	win.cursor.line = 1;
	win.cursor.offset = 3;
	check(buf.file_mtime_ns != 0);
	buf.file_mtime_ns = 1; /* force stale so rewrite looks newer */

	write_path(path, "alpha\nbeta\ngamma\n");

	check(!file_check_reload_current(false, false, false, &ask_line));
	line = buf.line(1);
	check(line != NULL);
	check(len(line.data) == 5 && memcmp(ptr(line.data), "alpha", 5) == 0);
	check(buf.line_count_i() == 4); /* trailing newline → empty last line */
	check(win.cursor.line == 1);
	check(!buf.is_changed);
	check(buf.notified_mtime_ns == 0);
}

static void test_check_reload_skips_dirty_when_notified() {
	char dir[512] = {0};
	char path[512] = {0};
	Buffer *buf = NULL;
	Window *win = NULL;
	Line *line = NULL;
	int ask_line = 0;

	snprintf(dir, sizeof(dir), "/tmp/jem-arevert2-XXXXXX");
	check(mkdtemp(dir) != NULL);
	snprintf(path, sizeof(path), "%s/dirty.txt", dir);
	write_path(path, "first\n");

	buffer_list_init();
	{
		auto (loaded, ok) = file_load(path);
		check(ok && loaded != NULL);
		buf = loaded;
	}
	win = setup_window(buf);
	(void)win;
	buf.is_changed = true;
	buf.file_mtime_ns = 1;

	write_path(path, "second\n");
	buf.notified_mtime_ns = file_mtime_ns(path);

	check(!file_check_reload_current(false, false, false, &ask_line));
	line = buf.line(1);
	check(line != NULL);
	check(len(line.data) == 5 && memcmp(ptr(line.data), "first", 5) == 0);
}

static void test_check_reload_dirty_sets_notified() {
	char dir[512] = {0};
	char path[512] = {0};
	Buffer *buf = NULL;
	int ask_line = 0;

	snprintf(dir, sizeof(dir), "/tmp/jem-arevert3-XXXXXX");
	check(mkdtemp(dir) != NULL);
	snprintf(path, sizeof(path), "%s/dirty2.txt", dir);
	write_path(path, "first\n");

	buffer_list_init();
	{
		auto (loaded, ok) = file_load(path);
		check(ok && loaded != NULL);
		buf = loaded;
	}
	(void)setup_window(buf);
	buf.is_changed = true;
	buf.file_mtime_ns = 1;

	write_path(path, "second\n");

	check(!file_check_reload_current(false, false, false, &ask_line));
	check(buf.notified_mtime_ns != 0);
	check(buf.notified_mtime_ns == file_mtime_ns(path));
	check(file_needs_overwrite_confirm(buf, NULL));
}

static void test_check_reload_auto_revert_dirty() {
	char dir[512] = {0};
	char path[512] = {0};
	Buffer *buf = NULL;
	Line *line = NULL;
	int ask_line = 0;

	snprintf(dir, sizeof(dir), "/tmp/jem-arevert4-XXXXXX");
	check(mkdtemp(dir) != NULL);
	snprintf(path, sizeof(path), "%s/auto.txt", dir);
	write_path(path, "first\n");

	buffer_list_init();
	{
		auto (loaded, ok) = file_load(path);
		check(ok && loaded != NULL);
		buf = loaded;
	}
	(void)setup_window(buf);
	buf.is_changed = true;
	buf.file_mtime_ns = 1;

	write_path(path, "reverted\n");

	check(!file_check_reload_current(false, true, false, &ask_line));
	line = buf.line(1);
	check(line != NULL);
	check(len(line.data) == 8 && memcmp(ptr(line.data), "reverted", 8) == 0);
	check(!buf.is_changed);
	check(buf.notified_mtime_ns == 0);
}

int main() {
	test_complete_path();
	test_check_reload_clean_buffer();
	test_check_reload_skips_dirty_when_notified();
	test_check_reload_dirty_sets_notified();
	test_check_reload_auto_revert_dirty();
	return 0;
}

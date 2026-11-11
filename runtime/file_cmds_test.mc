import "runtime";
import "buffer";
import "window";
import "display";
import "fs";
import "str";

#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include "../file/temp.h"

static void check(bool value) {
	if (!value) {
		abort();
	}
}

static Window *g_win;

static Window *setup(const char *text) {
	buffer_list_init();
	Buffer *buf = buffer_current();
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
	return g_win;
}

static void test_kill_buffer_confirm() {
	(void)setup("x");
	Buffer *other = buffer_new();
	check(other != NULL);
	other.set_name("victim");
	check(buffer_list_add(other));
	buffer_set_current(other);
	g_win.buffer = other;
	size_t before = len(buffer_all);
	check(cmd_kill_buffer(false, 0));
	misc_kill_pending_yes();
	check(len(buffer_all) == before - 1);
	check(buffer_find_by_name("victim") == NULL);
}

static void test_kill_scratch_rejected() {
	(void)setup("scratch");
	check(!cmd_kill_buffer(false, 0));
}

static void test_delete_this_file() {
	char dir[512] = {0};
	char path[512] = {0};

	int n = snprintf(dir, sizeof(dir), "/tmp/jem-del-XXXXXX");
	check(n > 0);
	check(mkdtemp(dir) != NULL);
	snprintf(path, sizeof(path), "%s/victim.txt", dir);
	{
		auto (w, wok) = writer_open(ranged(path, strlen(path)), OpenWrite | OpenCreate | OpenTrunc, 0);
		check(wok);
		(void)w.write("bye\n");
		w.close();
	}

	(void)setup("bye");
	Buffer *other = buffer_new();
	check(other != NULL);
	other.set_name("keeper");
	check(buffer_list_add(other));

	Buffer *buf = buffer_current();
	buf.set_name("victim.txt");
	buf.set_file_name(ranged(path, strlen(path)));

	check(cmd_delete_this_file(false, 1));
	misc_delete_file_yes();
	{
		auto (info, ok) = fs_stat(ranged(path, strlen(path)));
		(void)info;
		check(!ok);
	}
	check(buffer_find_by_name("victim.txt") == NULL);
	check(buffer_current() != NULL);
}

int main() {
	test_kill_scratch_rejected();
	test_kill_buffer_confirm();
	test_delete_this_file();
	return 0;
}

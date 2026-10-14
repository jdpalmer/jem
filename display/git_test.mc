import "display";
import "buffer";
import "window";
import "fs";
import "os";
import "str";
import "path";

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <unistd.h>
#include "../file/temp.h"

static void check(bool value) {
	if (!value) {
		abort();
	}
}

static void test_parse_hunk() {
	int old_start = 0;
	int old_count = 0;
	int new_start = 0;
	int new_count = 0;
	check(git_parse_hunk_header("@@ -10,2 +20,3 @@", &old_start, &old_count, &new_start, &new_count));
	check(old_start == 10 && old_count == 2 && new_start == 20 && new_count == 3);
	check(git_parse_hunk_header("@@ -1 +1,2 @@", &old_start, &old_count, &new_start, &new_count));
	check(old_start == 1 && old_count == 1 && new_start == 1 && new_count == 2);
	check(git_parse_hunk_header("@@ -5,3 +8 @@", &old_start, &old_count, &new_start, &new_count));
	check(old_start == 5 && old_count == 3 && new_start == 8 && new_count == 1);
	check(git_parse_hunk_header("@@ -1 +1 @@", &old_start, &old_count, &new_start, &new_count));
	check(old_start == 1 && old_count == 1 && new_start == 1 && new_count == 1);
	check(!git_parse_hunk_header("not a hunk", &old_start, &old_count, &new_start, &new_count));
}

static int run_git(const char *dir, const char *a0, const char *a1, const char *a2) {
	Cmd c = {0};
	const char[..] args[6] = {0};
	size_t n = 0;
	char gitbin[512] = {0};
	auto (found, ok) = os_look_path(ranged(gitbin, 0, sizeof(gitbin)), "git");
	(void)found;
	if (!ok) {
		return -1;
	}
	args[n++] = "-C";
	args[n++] = str_from_cstr(dir);
	args[n++] = str_from_cstr(a0);
	if (a1 != NULL) {
		args[n++] = str_from_cstr(a1);
	}
	if (a2 != NULL) {
		args[n++] = str_from_cstr(a2);
	}
	c.init(str_from_cstr(gitbin), args, n);
	(void)c.stdio(StdOut, IoNull);
	(void)c.stdio(StdErr, IoNull);
	if (!c.start()) {
		c.close();
		return -1;
	}
	{
		auto (code, wok) = c.wait();
		c.close();
		if (!wok) {
			return -1;
		}
		return code;
	}
}

static void test_modeline_and_gutter() {
	char dir[512] = {0};
	char path[512] = {0};
	Buffer *buf = NULL;
	Window *win = NULL;
	const char *git = NULL;
	int n = 0;

	n = snprintf(dir, sizeof(dir), "/tmp/jem-git-XXXXXX");
	check(n > 0);
	check(mkdtemp(dir) != NULL);
	check(run_git(dir, "init", NULL, NULL) == 0);
	check(run_git(dir, "config", "user.email", "t@example.com") == 0);
	check(run_git(dir, "config", "user.name", "t") == 0);

	snprintf(path, sizeof(path), "%s/f.txt", dir);
	{
		auto (w, wok) = writer_open(str_from_cstr(path), OpenWrite | OpenCreate | OpenTrunc, 0);
		check(wok);
		(void)w.write(str_from_cstr("one\n"));
		w.close();
	}
	check(run_git(dir, "add", "f.txt", NULL) == 0);
	check(run_git(dir, "commit", "-m", "init") == 0);
	{
		auto (w, wok) = writer_open(str_from_cstr(path), OpenWrite | OpenCreate | OpenTrunc, 0);
		check(wok);
		(void)w.write(str_from_cstr("one\ntwo\n"));
		w.close();
	}

	buffer_list_init();
	buf = buffer_current();
	check(buf != NULL);
	check(buf.load_bytes("one\ntwo\n", 8, EModeLF));
	buf.set_name("f.txt");
	buf.set_file_name(str_from_cstr(path));
	win = window_create();
	check(win != NULL);
	window_select(win);

	git = git_modeline_text(buf);
	check(git != NULL && git[0] != 0);
	check(strstr(git, "[") != NULL); /* dirty status */
	check(git_line_diff(buf, 2) == (int)GitLineDiffAdded ||
		git_line_diff(buf, 2) == (int)GitLineDiffModified);
	/* Second call must reuse cache (no serial/mtime change). */
	check(git_modeline_text(buf) == git);
}

int main() {
	test_parse_hunk();
	test_modeline_and_gutter();
	return 0;
}

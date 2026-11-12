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
#include <sys/stat.h>
#include <unistd.h>

static void check(bool ok) {
	if (!ok) {
		abort();
	}
}

static void make_temp_dir(char *dir, size_t cap, const char[..] pattern) {
	pool p {
		auto (d, ok) = fs_mkdir_temp(&p, "", pattern);
		check(ok && len(d) > 0 && len(d) < cap);
		snprintf(dir, cap, "%.*s", (int)len(d), ptr(d));
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

static void test_apply_filename_selection() {
	char out[512] = {0};

	check(file_apply_filename_selection(str_empty(), "foo.txt", out, sizeof(out)));
	check(strcmp(out, "foo.txt") == 0);

	check(file_apply_filename_selection("dir/", "bar.go", out, sizeof(out)));
	check(strcmp(out, "dir/bar.go") == 0);

	check(file_apply_filename_selection("dir/", "sub/", out, sizeof(out)));
	check(strcmp(out, "dir/sub/") == 0);

	check(file_apply_filename_selection("a/b/", "../", out, sizeof(out)));
	check(strcmp(out, "a/") == 0);

	check(file_apply_filename_selection("a/", "../", out, sizeof(out)));
	check(out[0] == 0);

	/* From cwd listing, "../" must climb to an absolute/~/ parent, not no-op. */
	check(file_apply_filename_selection(str_empty(), "../", out, sizeof(out)));
	check(out[0] != 0);
	check(out[strlen(out) - 1] == '/');
}

static void test_complete_path() {
	char dir[512] = {0};
	char path[512] = {0};
	char buf[512] = {0};

	make_temp_dir(dir, sizeof(dir), "jem-fcomp-");

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
	int ask_line = 0;

	make_temp_dir(dir, sizeof(dir), "jem-arevert-");
	snprintf(path, sizeof(path), "%s/watch.txt", dir);
	write_path(path, "first\n");

	buffer_list_init();
	auto (buf, ok) = file_load(ranged(path, strlen(path)));
	check(ok && buf != NULL);
	Window *win = setup_window(buf);
	win.cursor.line = 1;
	win.cursor.offset = 3;
	check(buf.file_mtime_ns != 0);
	buf.file_mtime_ns = 1; /* force stale so rewrite looks newer */

	write_path(path, "alpha\nbeta\ngamma\n");

	check(!file_check_reload_current(false, false, false, &ask_line));
	Line *line = buf.line(1);
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
	int ask_line = 0;

	make_temp_dir(dir, sizeof(dir), "jem-arevert2-");
	snprintf(path, sizeof(path), "%s/dirty.txt", dir);
	write_path(path, "first\n");

	buffer_list_init();
	auto (buf, ok) = file_load(ranged(path, strlen(path)));
	check(ok && buf != NULL);
	Window *win = setup_window(buf);
	(void)win;
	buf.is_changed = true;
	buf.file_mtime_ns = 1;

	write_path(path, "second\n");
	buf.notified_mtime_ns = file_mtime_ns(ranged(path, strlen(path)));

	check(!file_check_reload_current(false, false, false, &ask_line));
	Line *line = buf.line(1);
	check(line != NULL);
	check(len(line.data) == 5 && memcmp(ptr(line.data), "first", 5) == 0);
}

static void test_check_reload_dirty_sets_notified() {
	char dir[512] = {0};
	char path[512] = {0};
	int ask_line = 0;

	make_temp_dir(dir, sizeof(dir), "jem-arevert3-");
	snprintf(path, sizeof(path), "%s/dirty2.txt", dir);
	write_path(path, "first\n");

	buffer_list_init();
	auto (buf, ok) = file_load(ranged(path, strlen(path)));
	check(ok && buf != NULL);
	(void)setup_window(buf);
	buf.is_changed = true;
	buf.file_mtime_ns = 1;

	write_path(path, "second\n");

	check(!file_check_reload_current(false, false, false, &ask_line));
	check(buf.notified_mtime_ns != 0);
	check(buf.notified_mtime_ns == file_mtime_ns(ranged(path, strlen(path))));
	check(file_needs_overwrite_confirm(buf, ""));
}

static void test_check_reload_auto_revert_dirty() {
	char dir[512] = {0};
	char path[512] = {0};
	int ask_line = 0;

	make_temp_dir(dir, sizeof(dir), "jem-arevert4-");
	snprintf(path, sizeof(path), "%s/auto.txt", dir);
	write_path(path, "first\n");

	buffer_list_init();
	auto (buf, ok) = file_load(ranged(path, strlen(path)));
	check(ok && buf != NULL);
	(void)setup_window(buf);
	buf.is_changed = true;
	buf.file_mtime_ns = 1;

	write_path(path, "reverted\n");

	check(!file_check_reload_current(false, true, false, &ask_line));
	Line *line = buf.line(1);
	check(line != NULL);
	check(len(line.data) == 8 && memcmp(ptr(line.data), "reverted", 8) == 0);
	check(!buf.is_changed);
	check(buf.notified_mtime_ns == 0);
}

static void test_save_preserves_mode_and_symlink() {
	char dir[512] = {0};
	char target[512] = {0};
	char linkpath[512] = {0};
	char text[64] = {0};
	char linkbuf[512] = {0};
	struct stat st = {0};

	make_temp_dir(dir, sizeof(dir), "jem-fsave-");
	snprintf(target, sizeof(target), "%s/script.sh", dir);
	snprintf(linkpath, sizeof(linkpath), "%s/alias.sh", dir);
	write_path(target, "old\n");
	check(chmod(target, 0x1ed) == 0); /* 0755 */
	check(symlink(target, linkpath) == 0);

	buffer_list_init();
	Buffer *buf = buffer_current();
	check(buf != NULL);
	check(buf.load_bytes("new\n", 4, EModeLF));
	buf.set_file_name(str_from_cstr(linkpath));
	(void)setup_window(buf);
	check(file_save(buf, ""));

	check(lstat(linkpath, &st) == 0);
	check(readlink(linkpath, linkbuf, sizeof(linkbuf) - 1) > 0);
	check(stat(target, &st) == 0);
	check((st.st_mode & 0x1ff) == 0x1ed); /* 0777 / 0755 */

	{
		auto (r, rok) = reader_open(str_from_cstr(target), OpenRead, 0);
		check(rok);
		auto (n, ok) = r.read(ranged(text, sizeof(text) - 1));
		check(ok && n > 0);
		size_t got = n;
		r.close();
		text[got] = 0;
	}
	check(strcmp(text, "new\n") == 0);
}

int main() {
	test_apply_filename_selection();
	test_complete_path();
	test_check_reload_clean_buffer();
	test_check_reload_skips_dirty_when_notified();
	test_check_reload_dirty_sets_notified();
	test_check_reload_auto_revert_dirty();
	test_save_preserves_mode_and_symlink();
	return 0;
}

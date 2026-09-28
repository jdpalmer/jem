// file — load and save buffers from the filesystem.
import "buffer";
import "mode";
import "display";
import "window";
import "fs";
import "os";
import "path";
import "str";

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <unistd.h>
#include "temp.h"

static EolMode detect_eol(char *data, size_t n) {
	size_t i = 0;
	for (i = 0; i + 1 < n; i++) {
		if (data[i] == '\r' && data[i + 1] == '\n') {
			return EModeCRLF;
		}
	}
	return EModeLF;
}

enum {
	FilePathMax = 4096 };

// file_resolve_path writes a cleaned logical path into dst (~, relative → cwd).
bool file_resolve_path(char *dst, size_t dstcap, char *path) {
	char tmp[FilePathMax] = {
		0 };
	char joined[FilePathMax] = {
		0 };
	char[..] logical = str_empty();
	char[..] to_clean = str_empty();

	if (dst == NULL || dstcap < 2 || path == NULL || path[0] == 0) {
		return false;
	}
	logical = str_from_cstr(path);
	if (path[0] == '~') {
		char[..] home = str_empty();
		bool hok = false;
#ifdef _WIN32
		char home_buf[FilePathMax] = {
			0 };
		{
			auto (have, hv) = os_env("USERPROFILE");
			if (have && len(hv) > 0) {
				home = hv;
				hok = true;
			} else {
				auto (hd, drive) = os_env("HOMEDRIVE");
				auto (hp, pathv) = os_env("HOMEPATH");
				if (hd && hp && len(drive) > 0 && len(pathv) > 0) {
					snprintf(home_buf, sizeof(home_buf), "%.*s%.*s",
						(int)len(drive), ptr(drive),
						(int)len(pathv), ptr(pathv));
					home = str_from_cstr(home_buf);
					hok = true;
				}
			}
		}
#else
		{
			auto (have, hv) = os_env("HOME");
			hok = have;
			home = hv;
		}
#endif
		if (!hok || len(home) == 0) {
			return false;
		}
		if (path[1] == 0) {
			to_clean = home;
		} else if (path[1] == '/'
#ifdef _WIN32
			|| path[1] == '\\'
#endif
			) {
			auto (jv, jok) = path_join(ranged(joined, 0, sizeof(joined)), home,
				str_from_cstr(path + 2));
			if (!jok) {
				return false;
			}
			to_clean = jv;
		} else {
			return false;
		}
	} else if (!path_is_abs(logical)) {
		char cwd[FilePathMax] = {
			0 };
		auto (cw, cwok) = os_cwd(ranged(cwd, 0, sizeof(cwd)));
		if (!cwok || len(cw) == 0) {
			return false;
		}
		auto (jv, jok) = path_join(ranged(joined, 0, sizeof(joined)), cw, logical);
		if (!jok) {
			return false;
		}
		to_clean = jv;
	} else {
		to_clean = logical;
	}
	{
		auto (view, ok) = path_clean(ranged(tmp, 0, sizeof(tmp)), to_clean);
		if (!ok || len(view) == 0 || ptr(view) == NULL) {
			return false;
		}
		{
			int n = snprintf(dst, dstcap, "%s", view);
			if (n < 0 || (size_t)n >= dstcap) {
				return false;
			}
		}
	}
	return true;
}

static void file_apply_path_meta(Buffer *buf, char *path) {
	char[..] pv = str_from_cstr(path);
	char[..] base = path_base(pv);
	buf.set_file_name(str_from_cstr(path));
	if (len(base) > 0 && ptr(base) != NULL) {
		buf.set_name(base);
	} else {
		buf.set_name(str_from_cstr(path));
	}
	mode_apply_from_path(buf, path);
}

// file_mtime_ns returns the modification time of path in nanoseconds, or 0 on error.
int64_t file_mtime_ns(char *path) {
	if (path == NULL || path[0] == 0) {
		return 0;
	}
	{
		auto (info, ok) = fs_stat(str_from_cstr(path));
		if (!ok) {
			return 0;
		}
		return info.mtime_ns;
	}
}

// file_sync_mtime refreshes FileModTime from the buffer's path and clears the D marker.
void file_sync_mtime(Buffer *buf) {
	char path[FilePathMax] = {0};
	if (buf == NULL) {
		return;
	}
	if (len(buf.file_name) == 0) {
		buf.file_mtime_ns = 0;
		buf.notified_mtime_ns = 0;
		return;
	}
	{
		int n = snprintf(path, sizeof(path), "%s", buf.file_name);
		if (n < 0 || (size_t)n >= sizeof(path)) {
			buf.file_mtime_ns = 0;
			buf.notified_mtime_ns = 0;
			return;
		}
	}
	buf.file_mtime_ns = file_mtime_ns(path);
	buf.notified_mtime_ns = 0;
}

// file_needs_overwrite_confirm is true when path's mtime differs from the buffer's.
bool file_needs_overwrite_confirm(Buffer *buf, char *path) {
	char resolved[FilePathMax] = {0};
	char *use = NULL;
	int64_t cur = 0;

	if (buf == NULL || buf.file_mtime_ns == 0) {
		return false;
	}
	if (path != NULL && path[0] != 0) {
		if (file_resolve_path(resolved, sizeof(resolved), path)) {
			use = resolved;
		} else {
			use = path;
		}
	} else {
		if (len(buf.file_name) == 0) {
			return false;
		}
		{
			int n = snprintf(resolved, sizeof(resolved), "%s", buf.file_name);
			if (n < 0 || (size_t)n >= sizeof(resolved)) {
				return false;
			}
		}
		use = resolved;
	}
	cur = file_mtime_ns(use);
	return cur != 0 && cur != buf.file_mtime_ns;
}

// Read entire file into a malloc'd buffer (NUL-terminated). False on I/O error.
static (char *, size_t, bool) file_read_all(char *path) {
	char[..] pv = str_from_cstr(path);
	char *data = NULL;
	size_t cap = 0;
	size_t len = 0;
	char chunk[4096] = {
		0 };
	auto (r, ok) = reader_open(pv, OpenRead, 0);
	if (!ok) {
		return (NULL, 0, false);
	}
	for (;;) {
		auto (n, rok) = r.read(ranged(chunk, sizeof(chunk)));
		if (!rok) {
			r.close();
			free(data);
			return (NULL, 0, false);
		}
		if (n == 0) {
			break;
		}
		if (len + (size_t)n + 1 > cap) {
			size_t ncap = cap != 0 ? cap * 2 : 8192;
			char *p = NULL;
			while (ncap < len + (size_t)n + 1) {
				ncap = ncap * 2;
			}
			p = realloc(data, ncap);
			if (p == NULL) {
				r.close();
				free(data);
				return (NULL, 0, false);
			}
			data = p;
			cap = ncap;
		}
		memcpy(data + len, chunk, n);
		len = len + (size_t)n;
	}
	r.close();
	if (data == NULL) {
		data = malloc(1);
		if (data == NULL) {
			return (NULL, 0, false);
		}
		data[0] = 0;
		len = 0;
	} else {
		data[len] = 0;
	}
	return (data, len, true);
}

static bool file_path_missing(char *path) {
	char[..] pv = str_from_cstr(path);
	{
		auto (info, ok) = fs_stat(pv);
		(void)info;
		return !ok;
	}
}

// file_load_new creates an empty buffer bound to path (missing on-disk file).
static (Buffer *, bool) file_load_new(char *path) {
	Buffer *buf = buffer_new();
	if (buf == NULL) {
		return (NULL, false);
	}
	file_apply_path_meta(buf, path);
	buf.is_changed = false;
	buf.file_mtime_ns = 0;
	buf.notified_mtime_ns = 0;
	if (!buffer_list_add(buf)) {
		buf.free();
		return (NULL, false);
	}
	buffer_set_current(buf);
	display_set_echo("New file");
	return (buf, true);
}

// file_load reads path into a new buffer and adds it to the list.
// Missing paths become an empty "[New file]" buffer (jem.go LoadCurrentBuffer).
(Buffer *, bool) file_load(char *path) {
	Buffer *buf = {0};
	char resolved[FilePathMax] = {
		0 };

	if (path == NULL || path[0] == 0) {
		return (NULL, false);
	}
	if (!file_resolve_path(resolved, sizeof(resolved), path)) {
		snprintf(resolved, sizeof(resolved), "%s", path);
	}
	path = resolved;
	auto (data, n, ok) = file_read_all(path);
	if (!ok) {
		if (file_path_missing(path)) {
			return file_load_new(path);
		}
		return (NULL, false);
	}

	buf = buffer_new();
	if (buf == NULL) {
		free(data);
		return (NULL, false);
	}
	buf.load_bytes(data, n, detect_eol(data, n));
	free(data);
	file_apply_path_meta(buf, path);
	file_sync_mtime(buf);
	buf.saved_undo_serial = buf.serial;
	if (!buffer_list_add(buf)) {
		buf.free();
		return (NULL, false);
	}
	buffer_set_current(buf);
	display_set_echo("Reading file");
	return (buf, true);
}

// file_save writes buf to its file_name (or path if provided).
bool file_save(Buffer *buf, char *path) {
	char[..] usev = str_empty();
	char[..] pv = str_empty();
	char resolved[FilePathMax] = {
		0 };
	char temp[FilePathMax] = {
		0 };
	int fd = -1;
	int temp_len = 0;

	if (buf == NULL) {
		return false;
	}
	if (path != NULL && path[0] != 0) {
		if (file_resolve_path(resolved, sizeof(resolved), path)) {
			usev = str_from_cstr(resolved);
		} else {
			usev = str_from_cstr(path);
		}
	} else {
		usev = buf.file_name;
	}
	if (len(usev) == 0) {
		return false;
	}
	auto (data, n, ok) = buf.save_bytes();
	if (!ok || data == NULL) {
		return false;
	}
	temp_len = snprintf(temp, sizeof(temp), "%s.jem-save-XXXXXX", usev);
	if (temp_len < 0 || (size_t)temp_len >= sizeof(temp)) {
		free(data);
		return false;
	}
	fd = mkstemp(temp);
	if (fd < 0) {
		free(data);
		return false;
	}
	close(fd);
	pv = str_from_cstr(temp);
	auto (w, wok) = writer_open(pv, OpenWrite | OpenTrunc, 0);
	if (!wok) {
		fs_remove(pv);
		free(data);
		return false;
	}
	if (n != 0) {
		auto (wn, wrok) = w.write(str_from_bytes(data, n));
		if (!wrok || (size_t)wn != n) {
			w.close();
			fs_remove(pv);
			free(data);
			return false;
		}
	}
	if (!w.flush()) {
		w.close();
		fs_remove(pv);
		free(data);
		return false;
	}
	w.close();
	free(data);
	if (!fs_rename(pv, usev)) {
		fs_remove(pv);
		return false;
	}
	if (path != NULL && path[0] != 0) {
		file_apply_path_meta(buf, path);
	}
	buf.is_changed = false;
	buf.saved_undo_serial = buf.serial;
	file_sync_mtime(buf);
	return true;
}

// file_visit loads path or switches to an existing buffer with that path.
(Buffer *, bool) file_visit(char *path) {
	char[..] want = str_empty();
	char resolved[FilePathMax] = {
		0 };
	if (path == NULL) {
		return (NULL, false);
	}
	if (!file_resolve_path(resolved, sizeof(resolved), path)) {
		snprintf(resolved, sizeof(resolved), "%s", path);
	}
	path = resolved;
	want = str_from_cstr(path);
	for (auto b : buffer_all) {
		if (b != NULL && str_eq(want, b.file_name)) {
			buffer_set_current(b);
			return (b, true);
		}
	}
	return file_load(path);
}

// file_visit_location opens path and moves the cursor to 1-based line/column.
bool file_visit_location(char *path, int line, int column) {
	Window *w = {0};
	Line *bline = {0};
	Location loc = {0};
	size_t off = 0;
	auto (buf, ok) = file_visit(path);

	if (!ok || buf == NULL || line < 1) {
		return false;
	}
	window_switch_buffer(buf);
	w = window_current();
	if (w == NULL || w.buffer == NULL) {
		return false;
	}
	if (line > w.buffer.line_count_i()) {
		display_set_echo("File line out of range");
		return false;
	}
	bline = w.buffer.line(line);
	if (column > 0) {
		off = column - 1;
	} else {
		off = 0;
	}
	if (bline != NULL && off > bline.byte_len()) {
		off = bline.byte_len();
	}
	loc.line = line;
	loc.offset = off;
	w.set_cursor(loc);
	w.ensure_visible();
	/* Rough center when room allows. */
	if (w.height > 2 && w.cursor.line > w.height / 2) {
		w.top_line = w.cursor.line - w.height / 2;
		if (w.top_line < 1) {
			w.top_line = 1;
		}
		w.should_redraw = true;
	}
	w.should_update_mode_line = true;
	return true;
}

// file_read_into_current loads path into the current buffer (jem.go LoadCurrentBuffer).
bool file_read_into_current(char *path) {
	Buffer *buf = buffer_current();
	char resolved[FilePathMax] = {
		0 };

	if (buf == NULL || path == NULL || path[0] == 0) {
		return false;
	}
	if (!file_resolve_path(resolved, sizeof(resolved), path)) {
		snprintf(resolved, sizeof(resolved), "%s", path);
	}
	path = resolved;
	if (buf.is_readonly) {
		display_set_echo("Read-only buffer");
		return false;
	}
	auto (data, n, ok) = file_read_all(path);
	if (!ok) {
		if (file_path_missing(path)) {
			buf.load_bytes(NULL, 0, EModeLF);
			file_apply_path_meta(buf, path);
			buf.is_changed = false;
			buf.file_mtime_ns = 0;
			buf.notified_mtime_ns = 0;
			buf.saved_undo_serial = buf.serial;
			display_set_echo("New file");
			return true;
		}
		return false;
	}
	buf.load_bytes(data, n, detect_eol(data, n));
	free(data);
	file_apply_path_meta(buf, path);
	buf.is_changed = false;
	file_sync_mtime(buf);
	buf.saved_undo_serial = buf.serial;
	display_set_echo("Reading file");
	return true;
}

// file_reload_current reloads the buffer's on-disk file and restores line (1-based).
bool file_reload_current(int line_number) {
	Buffer *buf = buffer_current();
	Window *w = window_current();
	char path[FilePathMax] = {0};
	Location loc = {0};
	Line *bline = NULL;

	if (buf == NULL || len(buf.file_name) == 0) {
		return false;
	}
	{
		int n = snprintf(path, sizeof(path), "%s", buf.file_name);
		if (n < 0 || (size_t)n >= sizeof(path)) {
			return false;
		}
	}
	if (!file_read_into_current(path)) {
		return false;
	}
	if (w == NULL || w.buffer == NULL) {
		return true;
	}
	if (line_number < 1) {
		line_number = 1;
	}
	if (line_number > w.buffer.line_count_i()) {
		line_number = w.buffer.line_count_i();
	}
	if (line_number < 1) {
		line_number = 1;
	}
	bline = w.buffer.line(line_number);
	loc.line = line_number;
	loc.offset = 0;
	(void)bline;
	w.set_cursor(loc);
	w.ensure_visible();
	w.should_redraw = true;
	w.should_update_mode_line = true;
	return true;
}

// file_check_reload_current mirrors jem.go CheckReloadCurrentBuffer.
// When ask_prompt is false and a dirty buffer needs a decision, sets NotifiedModTime
// (D on the modeline) and leaves the buffer alone — same as Go with askYesNo == nil.
// When ask_prompt is true, returns true so the caller can AskYesNo (line in *out_line).
bool file_check_reload_current(bool dispatching, bool auto_revert, bool ask_prompt,
	int *out_line) {
	Buffer *buf = buffer_current();
	Window *w = window_current();
	char fname[FilePathMax] = {0};
	int64_t cur = 0;
	int line = 1;

	if (dispatching) {
		return false;
	}
	if (buf == NULL || w == NULL || buf.is_readonly) {
		return false;
	}
	if (len(buf.file_name) == 0 || buf.file_mtime_ns == 0) {
		return false;
	}
	{
		int n = snprintf(fname, sizeof(fname), "%s", buf.file_name);
		if (n < 0 || (size_t)n >= sizeof(fname)) {
			return false;
		}
	}
	cur = file_mtime_ns(fname);
	if (cur == 0 || cur == buf.file_mtime_ns) {
		if (buf.notified_mtime_ns != 0) {
			buf.notified_mtime_ns = 0;
			w.should_update_mode_line = true;
		}
		return false;
	}
	line = w.cursor.line;
	if (buf.is_changed) {
		if (auto_revert) {
			(void)file_reload_current(line);
			return false;
		}
		if (cur == buf.notified_mtime_ns) {
			return false;
		}
		buf.notified_mtime_ns = cur;
		w.should_update_mode_line = true;
		if (!ask_prompt) {
			return false;
		}
		if (out_line != NULL) {
			*out_line = line;
		}
		return true;
	}
	(void)file_reload_current(line);
	return false;
}

static void file_path_split(char *path, char *dir, size_t dircap, char *base, size_t basecap) {
	size_t n = 0;
	size_t i = 0;
	size_t last = 0;
	bool have_sep = false;

	if (dir != NULL && dircap > 0) {
		dir[0] = 0;
	}
	if (base != NULL && basecap > 0) {
		base[0] = 0;
	}
	if (path == NULL) {
		return;
	}
	n = strlen(path);
	for (i = 0; i < n; i++) {
		if (path[i] == '/') {
			last = i;
			have_sep = true;
		}
	}
	if (!have_sep) {
		if (base != NULL && basecap > 0) {
			snprintf(base, basecap, "%s", path);
		}
		return;
	}
	if (dir != NULL && dircap > 0) {
		size_t dlen = last + 1;
		if (dlen >= dircap) {
			dlen = dircap - 1;
		}
		memcpy(dir, path, dlen);
		dir[dlen] = 0;
	}
	if (base != NULL && basecap > 0) {
		snprintf(base, basecap, "%s", path + last + 1);
	}
}

static bool file_entry_is_dir(char *dir, char *name) {
	char joined[FilePathMax] = {0};
	char[..] jv = str_empty();
	char[..] dv = str_empty();
	char[..] nv = str_empty();

	if (name == NULL || name[0] == 0) {
		return false;
	}
	dv = str_from_cstr(dir != NULL && dir[0] != 0 ? dir : ".");
	nv = str_from_cstr(name);
	{
		auto (view, ok) = path_join(ranged(joined, 0, sizeof(joined)), dv, nv);
		if (!ok) {
			return false;
		}
		jv = view;
	}
	{
		auto (info, ok) = fs_stat(jv);
		if (!ok) {
			return false;
		}
		return info.kind == KindDir;
	}
}

// file_complete_path mutates buf in place with longest common filename prefix.
// Returns true when the text changed. Used by Tab in find/write/read-file prompts.
bool file_complete_path(char *buf, size_t cap) {
	char typed[FilePathMax] = {0};
	char tdir[FilePathMax] = {0};
	char tprefix[FilePathMax] = {0};
	char edir[FilePathMax] = {0};
	char eprefix[FilePathMax] = {0};
	char common[FilePathMax] = {0};
	char newbuf[FilePathMax] = {0};
	char dir_for_open[FilePathMax] = {0};
	int match_count = 0;
	bool match_is_dir = false;
	size_t prefix_len = 0;
	Dir d = {0};
	bool dok = false;

	if (buf == NULL || cap < 2) {
		return false;
	}
	snprintf(typed, sizeof(typed), "%s", buf);
	if (strcmp(typed, "~") == 0) {
		snprintf(buf, cap, "~/");
		return true;
	}
	file_path_split(typed, tdir, sizeof(tdir), tprefix, sizeof(tprefix));
	if (tdir[0] == 0) {
		snprintf(dir_for_open, sizeof(dir_for_open), ".");
	} else if (!file_resolve_path(dir_for_open, sizeof(dir_for_open), tdir)) {
		snprintf(dir_for_open, sizeof(dir_for_open), "%s", tdir);
	}
	{
		size_t dl = strlen(dir_for_open);
		if (dl > 1 && dir_for_open[dl - 1] == '/') {
			dir_for_open[dl - 1] = 0;
		}
	}
	snprintf(edir, sizeof(edir), "%s", dir_for_open);
	snprintf(eprefix, sizeof(eprefix), "%s", tprefix);
	prefix_len = strlen(eprefix);
	{
		auto (opened, ok) = dir_open(str_from_cstr(edir));
		d = opened;
		dok = ok;
	}
	if (!dok) {
		return false;
	}
	for (;;) {
		auto (namev, nok) = d.next();
		char name[FilePathMax] = {0};
		size_t nl = 0;
		if (!nok) {
			d.close();
			return false;
		}
		if (len(namev) == 0) {
			break;
		}
		nl = len(namev);
		if (nl >= sizeof(name)) {
			nl = sizeof(name) - 1;
		}
		memcpy(name, ptr(namev), nl);
		name[nl] = 0;
		if (prefix_len == 0 && name[0] == '.') {
			continue;
		}
		if (strncmp(name, eprefix, prefix_len) != 0) {
			continue;
		}
		if (match_count == 0) {
			snprintf(common, sizeof(common), "%s", name);
			match_is_dir = file_entry_is_dir(edir, name);
		} else {
			size_t i = 0;
			while (common[i] != 0 && name[i] != 0 && common[i] == name[i]) {
				i++;
			}
			common[i] = 0;
			match_is_dir = false;
		}
		match_count++;
	}
	d.close();
	if (match_count == 0 || common[0] == 0) {
		return false;
	}
	if (tdir[0] != 0) {
		snprintf(newbuf, sizeof(newbuf), "%s%s", tdir, common);
	} else {
		snprintf(newbuf, sizeof(newbuf), "%s", common);
	}
	if (match_count == 1 && match_is_dir) {
		size_t nl = strlen(newbuf);
		if (nl + 1 < sizeof(newbuf) && (nl == 0 || newbuf[nl - 1] != '/')) {
			newbuf[nl] = '/';
			newbuf[nl + 1] = 0;
		}
	}
	if (strlen(newbuf) >= cap) {
		return false;
	}
	if (strcmp(buf, newbuf) == 0) {
		return false;
	}
	snprintf(buf, cap, "%s", newbuf);
	return true;
}

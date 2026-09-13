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
	char[..] view = str_empty();

	if (dst == NULL || dstcap < 2 || path == NULL || path[0] == 0) {
		return false;
	}
	logical = str_from_cstr(path);
	if (path[0] == '~') {
		auto (hok, home) = os_env(str_from_cstr((char *)"HOME"));
		if (!hok || len(home) == 0) {
			return false;
		}
		if (path[1] == 0) {
			auto (cv, cok) = path_clean(ranged(tmp, 0, sizeof(tmp)), home);
			if (!cok) {
				return false;
			}
			view = cv;
		} else if (path[1] == '/') {
			auto (jv, jok) = path_join(ranged(joined, 0, sizeof(joined)), home, str_from_cstr(path + 2));
			if (!jok) {
				return false;
			}
			auto (cv, cok) = path_clean(ranged(tmp, 0, sizeof(tmp)), jv);
			if (!cok) {
				return false;
			}
			view = cv;
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
		auto (cv, cok) = path_clean(ranged(tmp, 0, sizeof(tmp)), jv);
		if (!cok) {
			return false;
		}
		view = cv;
	} else {
		auto (cv, cok) = path_clean(ranged(tmp, 0, sizeof(tmp)), logical);
		if (!cok) {
			return false;
		}
		view = cv;
	}
	if (len(view) == 0 || len(view) >= dstcap || ptr(view) == NULL) {
		return false;
	}
	memcpy(dst, ptr(view), len(view));
	dst[len(view)] = 0;
	return true;
}

static void file_apply_path_meta(Buffer *buf, char *path) {
	char[..] pv = str_from_cstr(path);
	char[..] base = path_base(pv);
	buf.set_file_name(path);
	if (len(base) > 0 && ptr(base) != NULL) {
		buf.set_name(ptr(base));
	} else {
		buf.set_name(path);
	}
	mode_apply_from_path(buf, path);
}

// Read entire file into a malloc'd buffer (NUL-terminated). False on I/O error.
static (char *, size_t, bool) file_read_all(char *path) {
	char[..] pv = str_from_cstr(path);
	Reader r = {
		0 };
	char *data = NULL;
	size_t cap = 0;
	size_t len = 0;
	char chunk[4096] = {
		0 };

	{
		auto (rd, ok) = reader_open(pv, OpenRead, 0);
		if (!ok) {
			return (NULL, 0, false);
		}
		r = rd;
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
		memcpy(data + len, chunk, (size_t)n);
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
	char *data = {0};
	size_t n = 0;
	bool ok = false;
	char resolved[FilePathMax] = {
		0 };

	if (path == NULL || path[0] == 0) {
		return (NULL, false);
	}
	if (!file_resolve_path(resolved, sizeof(resolved), path)) {
		snprintf(resolved, sizeof(resolved), "%s", path);
	}
	path = resolved;
	{
		auto (d, nn, ook) = file_read_all(path);
		data = d;
		n = nn;
		ok = ook;
	}
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
	char *use = {0};
	char *data = {0};
	size_t n = 0;
	bool ok = false;
	Writer w = {
		0 };
	char[..] pv = str_empty();
	char resolved[FilePathMax] = {
		0 };

	if (buf == NULL) {
		return false;
	}
	use = (path != NULL && path[0] != 0) ? path : buf.file_name;
	if (use == NULL || use[0] == 0) {
		return false;
	}
	if (path != NULL && path[0] != 0) {
		if (file_resolve_path(resolved, sizeof(resolved), path)) {
			use = resolved;
		}
	}
	auto (d, nn, ook) = buf.save_bytes();
	data = d;
	n = nn;
	ok = ook;
	if (!ok || data == NULL) {
		return false;
	}
	pv = str_from_cstr(use);
	{
		auto (wr, wok) = writer_open(pv, OpenWrite | OpenCreate | OpenTrunc, 0);
		if (!wok) {
			free(data);
			return false;
		}
		w = wr;
	}
	if (n != 0) {
		auto (wn, wrok) = w.write(str_from_bytes(data, n));
		if (!wrok || (size_t)wn != n) {
			w.close();
			free(data);
			return false;
		}
	}
	if (!w.flush()) {
		w.close();
		free(data);
		return false;
	}
	w.close();
	free(data);
	if (path != NULL && path[0] != 0) {
		file_apply_path_meta(buf, path);
	}
	buf.is_changed = false;
	buf.saved_undo_serial = buf.serial;
	return true;
}

// file_visit loads path or switches to an existing buffer with that path.
(Buffer *, bool) file_visit(char *path) {
	size_t i = 0;
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
	for (i = 0; i < buffer_all.count; i++) {
		Buffer *b = buffer_all.bufs[i];
		if (b != NULL && b.file_name != NULL && str_eq(want, str_from_cstr(b.file_name))) {
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
	int off = 0;
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
		display_set_echo(str_from_cstr((char *)"File line out of range"));
		return false;
	}
	bline = w.buffer.line(line);
	off = column;
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
	char *data = {0};
	size_t n = 0;
	bool ok = false;
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
	{
		auto (d, nn, ook) = file_read_all(path);
		data = d;
		n = nn;
		ok = ook;
	}
	if (!ok) {
		if (file_path_missing(path)) {
			buf.load_bytes(NULL, 0, EModeLF);
			file_apply_path_meta(buf, path);
			buf.is_changed = false;
			display_set_echo("New file");
			return true;
		}
		return false;
	}
	buf.load_bytes(data, n, detect_eol(data, n));
	free(data);
	file_apply_path_meta(buf, path);
	buf.is_changed = false;
	display_set_echo("Reading file");
	return true;
}

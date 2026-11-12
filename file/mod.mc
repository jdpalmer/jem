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

static EolMode detect_eol(char *data, size_t n) {
	size_t i = 0;
	const char[..] bytes = ranged(data != NULL ? data : "", n);
	for (; i + 1 < n; i++) {
		if (bytes[i] == '\r' && bytes[i + 1] == '\n') {
			return EModeCRLF;
		}
	}
	return EModeLF;
}

enum {
	FilePathMax = 4096 };

// file_resolve_path writes a cleaned logical path into dst (~, relative → cwd).
bool file_resolve_path(char *dst, size_t dstcap, const char[..] path) {
	if (dst == NULL || dstcap < 2 || len(path) == 0) {
		return false;
	}
	pool p {
		const char[..] logical = path;
		const char[..] to_clean = str_empty();
		if (logical[0] == '~') {
			auto (home, hok) = os_home(&p);
			if (!hok || len(home) == 0) {
				return false;
			}
			if (len(logical) == 1) {
				to_clean = home;
			} else if (len(logical) > 1 && (logical[1] == '/'
#ifdef _WIN32
				|| logical[1] == '\\'
#endif
				)) {
				auto (jv, jok) = path_join(&p, home, logical[2..]);
				if (!jok) {
					return false;
				}
				to_clean = jv;
			} else {
				return false;
			}
		} else if (!path_is_abs(logical)) {
			auto (cw, cwok) = os_cwd(&p);
			if (!cwok || len(cw) == 0) {
				return false;
			}
			auto (jv, jok) = path_join(&p, cw, logical);
			if (!jok) {
				return false;
			}
			to_clean = jv;
		} else {
			to_clean = logical;
		}
		auto (view, ok) = path_clean(&p, to_clean);
		if (!ok || len(view) == 0 || ptr(view) == NULL) {
			return false;
		}
		int n = snprintf(dst, dstcap, "%.*s", (int)len(view), ptr(view));
		if (n < 0 || (size_t)n >= dstcap) {
			return false;
		}
		return true;
	}
}

static void file_apply_path_meta(Buffer *buf, const char[..] path) {
	const char[..] base = path_base(path);
	buf.set_file_name(path);
	if (len(base) > 0 && ptr(base) != NULL) {
		buf.set_name(base);
	} else {
		buf.set_name(path);
	}
	mode_apply_from_path(buf, path);
}

// file_mtime_ns returns the modification time of path in nanoseconds, or 0 on error.
int64_t file_mtime_ns(const char[..] path) {
	if (len(path) == 0) {
		return 0;
	}
	{
		auto (info, ok) = fs_stat(path);
		if (!ok) {
			return 0;
		}
		return info.mtime_ns;
	}
}

// file_sync_mtime refreshes FileModTime from the buffer's path and clears the D marker.
void file_sync_mtime(Buffer *buf) {
	if (buf == NULL) {
		return;
	}
	if (len(buf.file_name) == 0) {
		buf.file_mtime_ns = 0;
		buf.notified_mtime_ns = 0;
		return;
	}
	buf.file_mtime_ns = file_mtime_ns(buf.file_name);
	buf.notified_mtime_ns = 0;
}

// file_needs_overwrite_confirm is true when path's mtime differs from the buffer's.
// Empty path means use the buffer's file_name.
bool file_needs_overwrite_confirm(Buffer *buf, const char[..] path) {
	char resolved[FilePathMax] = {0};

	if (buf == NULL || buf.file_mtime_ns == 0) {
		return false;
	}
	if (len(path) > 0) {
		const char[..] use = path;
		if (file_resolve_path(resolved, sizeof(resolved), path)) {
			use = ranged(resolved, strlen(resolved));
		}
		int64_t cur = file_mtime_ns(use);
		return cur != 0 && cur != buf.file_mtime_ns;
	}
	if (len(buf.file_name) == 0) {
		return false;
	}
	int64_t cur = file_mtime_ns(buf.file_name);
	return cur != 0 && cur != buf.file_mtime_ns;
}

// Read entire file into a malloc'd buffer (NUL-terminated). False on I/O error.
static (char *, size_t, bool) file_read_all(const char[..] path) {
	char[..] data = {0};
	char chunk[4096] = {0};
	auto (r, ok) = reader_open(path, OpenRead, 0);
	if (!ok) {
		return (NULL, 0, false);
	}
	for (;;) {
		auto (n, rok) = r.read(ranged(chunk, sizeof(chunk)));
		if (!rok) {
			r.close();
			str_free(&data);
			return (NULL, 0, false);
		}
		if (n == 0) {
			break;
		}
		if (!str_append(&data, ranged(chunk, n))) {
			r.close();
			str_free(&data);
			return (NULL, 0, false);
		}
	}
	r.close();
	if (!str_reserve(&data, len(data) + 1)) {
		str_free(&data);
		return (NULL, 0, false);
	}
	{
		char[..] z = ranged(ptr(data), len(data) + 1, cap(data));
		z[len(data)] = 0;
	}
	return (ptr(data), len(data), true);
}

static bool file_path_missing(const char[..] path) {
	auto (info, ok) = fs_stat(path);
	(void)info;
	return !ok;
}

// file_load_new creates an empty buffer bound to path (missing on-disk file).
static (Buffer *, bool) file_load_new(const char[..] path) {
	Buffer *buf = buffer_new();
	if (buf == NULL) {
		return (NULL, false);
	}
	file_apply_path_meta(buf, path);
	buf.is_changed = false;
	buf.file_mtime_ns = 0;
	buf.notified_mtime_ns = 0;
	if (!buffer_list_add(buf)) {
		return (NULL, false);
	}
	display_set_echo("New file");
	return (buffer_current(), true);
}

// file_load reads path into a new buffer and adds it to the list.
// Missing paths become an empty "[New file]" buffer (jem.go LoadCurrentBuffer).
(Buffer *, bool) file_load(const char[..] path) {
	char resolved[FilePathMax] = {
		0 };

	if (len(path) == 0) {
		return (NULL, false);
	}
	const char[..] use = path;
	if (file_resolve_path(resolved, sizeof(resolved), path)) {
		use = ranged(resolved, strlen(resolved));
	}
	auto (data, n, ok) = file_read_all(use);
	if (!ok) {
		if (file_path_missing(use)) {
			return file_load_new(use);
		}
		return (NULL, false);
	}

	Buffer *buf = buffer_new();
	if (buf == NULL) {
		free(data);
		return (NULL, false);
	}
	buf.load_bytes(data, n, detect_eol(data, n));
	free(data);
	file_apply_path_meta(buf, use);
	file_sync_mtime(buf);
	buf.saved_undo_serial = buf.serial;
	if (!buffer_list_add(buf)) {
		return (NULL, false);
	}
	display_set_echo("Reading file");
	return (buffer_current(), true);
}

// file_save writes buf to its file_name (or path if provided).
// Empty path means use the buffer's file_name.
// Opens the destination in place (like jem.go): follows symlinks and keeps
// existing file modes. mkstemp+rename would replace a symlink and force 0600.
bool file_save(Buffer *buf, const char[..] path) {
	char resolved[FilePathMax] = {
		0 };

	if (buf == NULL) {
		return false;
	}
	const char[..] usev = buf.file_name;
	if (len(path) > 0) {
		if (file_resolve_path(resolved, sizeof(resolved), path)) {
			usev = ranged(resolved, strlen(resolved));
		} else {
			usev = path;
		}
	}
	if (len(usev) == 0) {
		return false;
	}
	auto (data, n, ok) = buf.save_bytes();
	if (!ok || data == NULL) {
		return false;
	}
	{
		auto (w, wok) = writer_open(usev, OpenWrite | OpenCreate | OpenTrunc, 0);
		if (!wok) {
			free(data);
			return false;
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
	}
	free(data);
	if (len(path) > 0) {
		file_apply_path_meta(buf, path);
	}
	buf.is_changed = false;
	buf.saved_undo_serial = buf.serial;
	file_sync_mtime(buf);
	return true;
}

// file_visit loads path or switches to an existing buffer with that path.
(Buffer *, bool) file_visit(const char[..] path) {
	char resolved[FilePathMax] = {
		0 };
	if (len(path) == 0) {
		return (NULL, false);
	}
	const char[..] use = path;
	if (file_resolve_path(resolved, sizeof(resolved), path)) {
		use = ranged(resolved, strlen(resolved));
	}
	for (auto b : buffer_all) {
		if (b != NULL && str_eq(use, b.file_name)) {
			/* Re-apply language/indent defaults (e.g. .mc → tabs) in case the
			 * buffer was opened under an older mode mapping. */
			file_apply_path_meta(b, use);
			buffer_set_current(b);
			return (b, true);
		}
	}
	return file_load(use);
}

// file_visit_location opens path and moves the cursor to 1-based line/column.
bool file_visit_location(const char[..] path, int line, int column) {
	auto (buf, ok) = file_visit(path);

	if (!ok || buf == NULL || line < 1) {
		return false;
	}
	window_switch_buffer(buf);
	Window *w = window_current();
	if (w == NULL || w.buffer == NULL) {
		return false;
	}
	if (line > w.buffer.line_count_i()) {
		display_set_echo("File line out of range");
		return false;
	}
	Line *bline = w.buffer.line(line);
	size_t off = column > 0 ? (size_t)(column - 1) : 0;
	if (bline != NULL && off > bline.byte_len()) {
		off = bline.byte_len();
	}
	Location loc = { .line = line, .offset = off };
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
bool file_read_into_current(const char[..] path) {
	Buffer *buf = buffer_current();
	char resolved[FilePathMax] = {
		0 };

	if (buf == NULL || len(path) == 0) {
		return false;
	}
	const char[..] use = path;
	if (file_resolve_path(resolved, sizeof(resolved), path)) {
		use = ranged(resolved, strlen(resolved));
	}
	if (buf.is_readonly) {
		display_set_echo("Read-only buffer");
		return false;
	}
	auto (data, n, ok) = file_read_all(use);
	if (!ok) {
		if (file_path_missing(use)) {
			buf.load_bytes(NULL, 0, EModeLF);
			file_apply_path_meta(buf, use);
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
	file_apply_path_meta(buf, use);
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

	if (buf == NULL || len(buf.file_name) == 0) {
		return false;
	}
	if (!file_read_into_current(buf.file_name)) {
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
	Line *bline = w.buffer.line(line_number);
	Location loc = { .line = line_number, .offset = 0 };
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

	if (dispatching) {
		return false;
	}
	if (buf == NULL || w == NULL || buf.is_readonly) {
		return false;
	}
	if (len(buf.file_name) == 0 || buf.file_mtime_ns == 0) {
		return false;
	}
	int64_t cur = file_mtime_ns(buf.file_name);
	if (cur == 0 || cur == buf.file_mtime_ns) {
		if (buf.notified_mtime_ns != 0) {
			buf.notified_mtime_ns = 0;
			w.should_update_mode_line = true;
		}
		return false;
	}
	int line = w.cursor.line;
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

static void file_path_split(const char[..] path, char *dir, size_t dircap, char *base,
	size_t basecap) {
	size_t i = 0;
	size_t last = 0;
	bool have_sep = false;

	if (dir != NULL && dircap > 0) {
		char[..] dv = ranged(dir, dircap);
		dv[0] = 0;
	}
	if (base != NULL && basecap > 0) {
		char[..] bv = ranged(base, basecap);
		bv[0] = 0;
	}
	if (len(path) == 0) {
		return;
	}
	size_t n = len(path);
	for (; i < n; i++) {
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
		memcpy(dir, ptr(path), dlen);
		char[..] dv = ranged(dir, dircap);
		dv[dlen] = 0;
	}
	if (base != NULL && basecap > 0) {
		snprintf(base, basecap, "%s", path[last + 1 ..]);
	}
}

static bool file_entry_is_dir(const char[..] dir, const char[..] name) {
	if (len(name) == 0) {
		return false;
	}
	pool p {
		const char[..] dv = len(dir) > 0 ? dir : ".";
		auto (jv, jok) = path_join(&p, dv, name);
		if (!jok) {
			return false;
		}
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

	if (buf == NULL || cap < 2) {
		return false;
	}
	snprintf(typed, sizeof(typed), "%s", buf);
	if (strcmp(typed, "~") == 0) {
		snprintf(buf, cap, "~/");
		return true;
	}
	file_path_split(ranged(typed, strlen(typed)), tdir, sizeof(tdir), tprefix,
		sizeof(tprefix));
	if (tdir[0] == 0) {
		snprintf(dir_for_open, sizeof(dir_for_open), ".");
	} else if (!file_resolve_path(dir_for_open, sizeof(dir_for_open),
		ranged(tdir, strlen(tdir)))) {
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
	size_t prefix_len = strlen(eprefix);
	auto (opened, dok) = dir_open(ranged(edir, strlen(edir)));
	Dir d = opened;
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
			match_is_dir = file_entry_is_dir(ranged(edir, strlen(edir)),
				ranged(name, strlen(name)));
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

// file_open_dir_from_prompt resolves a prompt directory prefix for dir_open.
// Empty dirPart → "."; otherwise expand/resolve like file_complete_path.
bool file_open_dir_from_prompt(char *dst, size_t dstcap, const char[..] dirPart) {
	char resolved[FilePathMax] = {0};

	if (dst == NULL || dstcap < 2) {
		return false;
	}
	if (len(dirPart) == 0) {
		snprintf(dst, dstcap, ".");
		return true;
	}
	if (file_resolve_path(resolved, sizeof(resolved), dirPart)) {
		size_t dl = strlen(resolved);
		if (dl > 1 && resolved[dl - 1] == '/') {
			resolved[dl - 1] = 0;
		}
		snprintf(dst, dstcap, "%s", resolved);
		return true;
	}
	{
		size_t n = len(dirPart);
		char[..] dv = ranged(dst, dstcap);
		if (n >= dstcap) {
			n = dstcap - 1;
		}
		memcpy(dst, ptr(dirPart), n);
		dv[n] = 0;
		size_t dl = strlen(dst);
		if (dl > 1 && dv[dl - 1] == '/') {
			dv[dl - 1] = 0;
		}
	}
	return true;
}

static bool file_contract_home(char *dst, size_t dstcap, const char[..] absPath) {
	if (dst == NULL || dstcap < 3 || len(absPath) == 0) {
		return false;
	}
	pool p {
		auto (home, hok) = os_home(&p);
		if (!hok || len(home) == 0) {
			return false;
		}
		auto (hv, hok2) = path_clean(&p, home);
		if (!hok2 || len(hv) == 0) {
			return false;
		}
		auto (av, aok) = path_clean(&p, absPath);
		if (!aok || len(av) == 0) {
			return false;
		}
		size_t hlen = len(hv);
		size_t alen = len(av);
		if (alen == hlen && memcmp(ptr(av), ptr(hv), hlen) == 0) {
			snprintf(dst, dstcap, "~/");
			return true;
		}
		if (alen > hlen && memcmp(ptr(av), ptr(hv), hlen) == 0 && av[hlen] == '/') {
			snprintf(dst, dstcap, "~/%.*s/", (int)(alen - hlen - 1), ptr(av) + hlen + 1);
			return true;
		}
		return false;
	}
}

// file_prompt_parent_dir writes the prompt directory one level above dirPart.
// Empty dirPart means cwd: navigate to the absolute parent (so "../" from the
// initial find-file listing is selectable).
bool file_prompt_parent_dir(char *dst, size_t dstcap, const char[..] dirPart) {
	char open_dir[FilePathMax] = {0};
	char parent[FilePathMax] = {0};
	char contracted[FilePathMax] = {0};

	if (dst == NULL || dstcap < 2) {
		return false;
	}
	snprintf(dst, dstcap, "");
	if (!file_open_dir_from_prompt(open_dir, sizeof(open_dir), dirPart)) {
		return false;
	}
	/* path_dir(".") is empty — resolve to an absolute path first. */
	{
		char abs_open[FilePathMax] = {0};
		if (file_resolve_path(abs_open, sizeof(abs_open),
			ranged(open_dir, strlen(open_dir)))) {
			snprintf(open_dir, sizeof(open_dir), "%s", abs_open);
		}
	}
	{
		const char[..] pd = path_dir(ranged(open_dir, strlen(open_dir)));
		if (len(pd) == 0) {
			return true;
		}
		size_t n = len(pd);
		if (n >= sizeof(parent)) {
			n = sizeof(parent) - 1;
		}
		memcpy(parent, ptr(pd), n);
		parent[n] = 0;
	}
	if (strcmp(parent, open_dir) == 0) {
		if (len(dirPart) == 0) {
			snprintf(dst, dstcap, "/");
		} else {
			snprintf(dst, dstcap, "%s", dirPart);
		}
		return true;
	}
	if (len(dirPart) == 0 ||
		(len(dirPart) >= 2 && dirPart[0] == '~' && dirPart[1] == '/')) {
		if (file_contract_home(contracted, sizeof(contracted),
			ranged(parent, strlen(parent)))) {
			snprintf(dst, dstcap, "%s", contracted);
			return true;
		}
		if (strcmp(parent, "/") == 0) {
			snprintf(dst, dstcap, "/");
		} else {
			snprintf(dst, dstcap, "%s/", parent);
		}
		return true;
	}
	if (path_is_abs(dirPart)) {
		if (strcmp(parent, "/") == 0) {
			snprintf(dst, dstcap, "/");
		} else {
			snprintf(dst, dstcap, "%s/", parent);
		}
		return true;
	}
	{
		char trimmed[FilePathMax] = {0};
		size_t n = len(dirPart);
		int last = -1;
		size_t i = 0;
		if (n >= sizeof(trimmed)) {
			n = sizeof(trimmed) - 1;
		}
		memcpy(trimmed, ptr(dirPart), n);
		trimmed[n] = 0;
		size_t end = strlen(trimmed);
		while (end > 0 && trimmed[end - 1] == '/') {
			end--;
		}
		trimmed[end] = 0;
		for (; i < end; i++) {
			if (trimmed[i] == '/') {
				last = (int)i;
			}
		}
		if (last >= 0) {
			trimmed[last + 1] = 0;
			snprintf(dst, dstcap, "%s", trimmed);
		} else if (strcmp(trimmed, "..") == 0) {
			/* Relative "../" prompt: keep climbing via absolute parent. */
			if (file_contract_home(contracted, sizeof(contracted),
				ranged(parent, strlen(parent)))) {
				snprintf(dst, dstcap, "%s", contracted);
			} else if (strcmp(parent, "/") == 0) {
				snprintf(dst, dstcap, "/");
			} else {
				snprintf(dst, dstcap, "%s/", parent);
			}
		}
	}
	return true;
}

// file_apply_filename_selection joins dirPart with selected, or navigates up for "../".
// Directory selections keep a trailing '/' so the next prompt lists that folder.
bool file_apply_filename_selection(const char[..] dirPart, const char[..] selected,
	char *out, size_t cap) {
	if (out == NULL || cap < 2 || len(selected) == 0) {
		return false;
	}
	char[..] ov = ranged(out, cap);
	if (len(selected) >= 3 && selected[0] == '.' && selected[1] == '.' && selected[2] == '/') {
		return file_prompt_parent_dir(out, cap, dirPart);
	}
	if (len(dirPart) == 0) {
		size_t n = len(selected);
		if (n >= cap) {
			n = cap - 1;
		}
		memcpy(out, ptr(selected), n);
		ov[n] = 0;
	} else {
		size_t dlen = len(dirPart);
		size_t slen = len(selected);
		if (dlen + slen >= cap) {
			return false;
		}
		memcpy(out, ptr(dirPart), dlen);
		memcpy(out + dlen, ptr(selected), slen);
		ov[dlen + slen] = 0;
	}
	{
		size_t slen = len(selected);
		if (slen > 0 && selected[slen - 1] == '/') {
			size_t n = strlen(out);
			if (n + 1 < cap && (n == 0 || ov[n - 1] != '/')) {
				ov[n] = '/';
				ov[n + 1] = 0;
			}
		}
	}
	return true;
}

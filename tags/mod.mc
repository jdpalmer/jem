// tags — Universal Ctags tags.json load, goto, and build (jem.go/tools/tags).
import "buffer";
import "window";
import "display";
import "minibuffer";
import "term";
import "file";
import "markring";
import "job";
import "mode";
import "fs";
import "os";
import "path";
import "str";
import "syntax";

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <ctype.h>
#include <unistd.h>
#include "temp.h"

enum {
	TagPathMax = 4096,
	TagNameMax = 256,
	TagKindMax = 64,
	TagPatMax = 512,
	TagPickerMax = 255
};

struct TagEntry {
	char[..] name;
	char[..] path;
	char[..] kind;
	char[..] signature;
	char[..] scope;
	char[..] typeref;
	char[..] pattern;
	int line;
};

struct TagDb {
	char path[TagPathMax];
	char anchor[TagPathMax];
	int64_t mtime;
	TagEntry[..] entries; /* owned heap */
	TagEntry *[..] by_name; /* owned heap; pointers into entries */
};

struct TagList {
	TagEntry *[..] items; /* owned heap of TagEntry*; entries owned */
};

/* Same layout as job loc metadata: line, column, then path bytes. */
struct TagLocHead {
	int line;
	int column;
	size_t pattern_offset;
};

static TagDb g_tags;
static char g_tags_name[TagNameMax] = {0};
static char g_tags_error[256] = {0};

static void tag_entry_free(TagEntry *e) {
	if (e == NULL) {
		return;
	}
	str_free(&e.name);
	str_free(&e.path);
	str_free(&e.kind);
	str_free(&e.signature);
	str_free(&e.scope);
	str_free(&e.typeref);
	str_free(&e.pattern);
	e.name = str_empty();
	e.path = str_empty();
	e.kind = str_empty();
	e.signature = str_empty();
	e.scope = str_empty();
	e.typeref = str_empty();
	e.pattern = str_empty();
	e.line = 0;
}

static void tag_db_free(TagDb *db) {
	if (db == NULL) {
		return;
	}
	for (auto *e : db.entries) {
		tag_entry_free(e);
	}
	release(&db.entries);
	release(&db.by_name);
	memset(db, 0, sizeof(*db));
}

void tags_invalidate() {
	tag_db_free(&g_tags);
}

static bool tag_db_append(TagEntry *e) {
	if (e == NULL) {
		return false;
	}
	if (!push(&g_tags.entries, *e)) {
		return false;
	}
	/* Ownership moved into g_tags.entries; clear without freeing. */
	e.name = str_empty();
	e.path = str_empty();
	e.kind = str_empty();
	e.signature = str_empty();
	e.scope = str_empty();
	e.typeref = str_empty();
	e.pattern = str_empty();
	e.line = 0;
	return true;
}

void tags_list_free(TagList *list) {
	if (list == NULL) {
		return;
	}
	for (auto e : list.items) {
		if (e != NULL) {
			tag_entry_free(e);
			free(e);
		}
	}
	release(&list.items);
}

static bool tag_list_append(TagList *list, TagEntry *entry) {
	if (list == NULL || entry == NULL) {
		return false;
	}
	TagEntry *copy = alloc(TagEntry);
	if (copy == NULL) {
		return false;
	}
	*copy = (TagEntry){0};
	{
		auto (ok, s) = str_dup(entry.name);
		if (!ok) {
			tag_entry_free(copy);
			free(copy);
			return false;
		}
		copy.name = s;
	}
	{
		auto (ok, s) = str_dup(entry.path);
		if (!ok) {
			tag_entry_free(copy);
			free(copy);
			return false;
		}
		copy.path = s;
	}
	{
		auto (ok, s) = str_dup(entry.kind);
		if (!ok) {
			tag_entry_free(copy);
			free(copy);
			return false;
		}
		copy.kind = s;
	}
	{
		auto (ok, s) = str_dup(entry.signature);
		if (!ok) {
			tag_entry_free(copy);
			free(copy);
			return false;
		}
		copy.signature = s;
	}
	{
		auto (ok, s) = str_dup(entry.scope);
		if (!ok) {
			tag_entry_free(copy);
			free(copy);
			return false;
		}
		copy.scope = s;
	}
	{
		auto (ok, s) = str_dup(entry.typeref);
		if (!ok) {
			tag_entry_free(copy);
			free(copy);
			return false;
		}
		copy.typeref = s;
	}
	{
		auto (ok, s) = str_dup(entry.pattern);
		if (!ok) {
			tag_entry_free(copy);
			free(copy);
			return false;
		}
		copy.pattern = s;
	}
	copy.line = entry.line;
	if (!push(&list.items, copy)) {
		tag_entry_free(copy);
		free(copy);
		return false;
	}
	return true;
}

static int tag_entry_name_cmp(TagEntry *a, TagEntry *b) {
	if (a == NULL || str_is_empty(a.name)) {
		return b == NULL || str_is_empty(b.name) ? 0 : -1;
	}
	if (b == NULL || str_is_empty(b.name)) {
		return 1;
	}
	int cmp = str_cmp(a.name, b.name);
	if (cmp != 0) {
		return cmp;
	}
	if (!str_is_empty(a.path) && !str_is_empty(b.path)) {
		cmp = str_cmp(a.path, b.path);
		if (cmp != 0) {
			return cmp;
		}
	}
	if (a.line < b.line) {
		return -1;
	}
	if (a.line > b.line) {
		return 1;
	}
	return 0;
}

static bool tag_db_rebuild_index() {
	size_t width = 1;
	size_t i = 0;
	size_t n = len(g_tags.entries);
	release(&g_tags.by_name);
	if (n == 0) {
		return true;
	}
	g_tags.by_name = alloc(TagEntry *, n);
	if (ptr(g_tags.by_name) == NULL) {
		return false;
	}
	for (; i < n; i++) {
		g_tags.by_name[i] = &g_tags.entries[i];
	}
	pool p {
		TagEntry *[..] tmp = p.alloc(TagEntry *, n);
		if (ptr(tmp) == NULL) {
			release(&g_tags.by_name);
			return false;
		}
		while (width < n) {
			size_t left = 0;
			for (left = 0; left < n; left += width * 2) {
				size_t mid = left + width;
				size_t right = left + width * 2;
				size_t a = left;
				size_t b = 0;
				size_t out = left;
				if (mid > n) {
					mid = n;
				}
				if (right > n) {
					right = n;
				}
				b = mid;
				while (a < mid && b < right) {
					if (tag_entry_name_cmp(g_tags.by_name[a], g_tags.by_name[b]) <= 0) {
						tmp[out++] = g_tags.by_name[a++];
					} else {
						tmp[out++] = g_tags.by_name[b++];
					}
				}
				while (a < mid) {
					tmp[out++] = g_tags.by_name[a++];
				}
				while (b < right) {
					tmp[out++] = g_tags.by_name[b++];
				}
			}
			for (i = 0; i < n; i++) {
				g_tags.by_name[i] = tmp[i];
			}
			if (width > n / 2) {
				break;
			}
			width *= 2;
		}
	}
	return true;
}

static size_t tag_index_lower_bound(const char[..] name) {
	size_t lo = 0;
	size_t hi = len(g_tags.by_name);
	while (lo < hi) {
		size_t mid = lo + (hi - lo) / 2;
		const char[..] at = g_tags.by_name[mid].name;
		if (!str_is_empty(at) && str_cmp(at, name) < 0) {
			lo = mid + 1;
		} else {
			hi = mid;
		}
	}
	return lo;
}

[[provides]] void *tags_loc_alloc(const char[..] path, int line, int column,
	const char[..] pattern) {
	if (str_is_empty(path)) {
		return NULL;
	}
	size_t plen = len(path);
	size_t patlen = len(pattern);
	TagLocHead *h = malloc(sizeof(TagLocHead) + plen + 1 + patlen + 1);
	if (h == NULL) {
		return NULL;
	}
	h.line = line;
	h.column = column;
	h.pattern_offset = plen + 1;
	memcpy((char *)(h + 1), ptr(path), plen);
	{
		char[..] z = ranged((char *)(h + 1), plen + 1);
		z[plen] = 0;
	}
	if (patlen > 0) {
		memcpy((char *)(h + 1) + h.pattern_offset, ptr(pattern), patlen);
	}
	{
		char[..] tail = ranged((char *)(h + 1) + h.pattern_offset, patlen + 1);
		tail[patlen] = 0;
	}
	return h;
}

static char *tag_loc_path(TagLocHead *h) {
	if (h == NULL) {
		return NULL;
	}
	return (char *)(h + 1);
}

static char *tag_loc_pattern(TagLocHead *h) {
	if (h == NULL) {
		return NULL;
	}
	return (char *)(h + 1) + h.pattern_offset;
}

static int tag_parse_line(char *line, char *tags_dir, TagEntry *out) {
	TagJsonFields f = {0};
	char resolved[TagPathMax] = {0};
	TagEntry e = {0};
	if (line == NULL || out == NULL) {
		return TagJsonMalformed;
	}
	int parsed = tag_json_parse_line(line, &f);
	if (parsed != (int)TagJsonEntry) {
		return parsed;
	}
	e.name = f.name;
	f.name = str_empty();
	e.kind = f.kind;
	f.kind = str_empty();
	e.signature = f.signature;
	f.signature = str_empty();
	e.scope = f.scope;
	f.scope = str_empty();
	e.typeref = f.typeref;
	f.typeref = str_empty();
	e.pattern = f.pattern;
	f.pattern = str_empty();
	e.line = f.line;
	if (path_is_abs(f.path)) {
		e.path = f.path;
		f.path = str_empty();
	} else {
		auto (jv, jok) = path_join(ranged(resolved, 0, sizeof(resolved)),
			ranged(tags_dir, strlen(tags_dir)), f.path);
		if (!jok || len(jv) == 0) {
			tag_entry_free(&e);
			tag_json_fields_free(&f);
			return TagJsonSkip;
		}
		auto (ok, s) = str_dup(jv);
		if (!ok) {
			tag_entry_free(&e);
			tag_json_fields_free(&f);
			return TagJsonSkip;
		}
		e.path = s;
	}
	tag_json_fields_free(&f);
	if (str_is_empty(e.name) || str_is_empty(e.path)) {
		tag_entry_free(&e);
		return TagJsonSkip;
	}
	*out = e;
	return TagJsonEntry;
}

(char *, size_t, bool) tags_read_file(const char[..] path) {
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

static bool tag_db_load(const char[..] path) {
	TagDb previous = g_tags;
	char tags_dir[TagPathMax] = {0};
	int source_line = 0;
	g_tags_error[0] = 0;
	memset(&g_tags, 0, sizeof(g_tags));
	auto (text, text_len, ok) = tags_read_file(path);
	if (!ok) {
		g_tags = previous;
		return false;
	}
	{
		const char[..] d = path_dir(path);
		if (len(d) == 0 || ptr(d) == NULL) {
			/* keep empty tags_dir */
		} else {
			int n = snprintf(tags_dir, sizeof(tags_dir), "%s", d);
			if (n < 0 || (size_t)n >= sizeof(tags_dir)) {
				tags_dir[0] = 0;
			}
		}
	}
	size_t i = 0;
	const char[..] tv = ranged(text != NULL ? text : "", text_len);
	while (i < text_len) {
		size_t start = i;
		size_t nline = 0;
		char[..] line = {0};
		TagEntry e = {0};
		int parsed = TagJsonSkip;
		source_line++;
		while (i < text_len && tv[i] != '\n') {
			i++;
		}
		nline = i - start;
		if (i < text_len && tv[i] == '\n') {
			i++;
		}
		if (nline == 0) {
			continue;
		}
		if (tv[start + nline - 1] == '\r') {
			nline--;
		}
		if (nline == 0) {
			continue;
		}
		auto (lok, ls) = str_dup(tv[start .. start + nline]);
		if (!lok) {
			free(text);
			tags_invalidate();
			g_tags = previous;
			return false;
		}
		line = ls;
		if (!str_reserve(&line, len(line) + 1)) {
			str_free(&line);
			free(text);
			tags_invalidate();
			g_tags = previous;
			return false;
		}
		{
			char[..] z = ranged(ptr(line), len(line) + 1, cap(line));
			z[len(line)] = 0;
		}
		parsed = tag_parse_line(ptr(line), tags_dir, &e);
		if (parsed == (int)TagJsonMalformed) {
			str_free(&line);
			free(text);
			tags_invalidate();
			g_tags = previous;
			snprintf(g_tags_error, sizeof(g_tags_error),
				"[invalid tags.json line %d]", source_line);
			return false;
		}
		if (parsed == (int)TagJsonEntry) {
			if (!tag_db_append(&e)) {
				tag_entry_free(&e);
				str_free(&line);
				free(text);
				tags_invalidate();
				g_tags = previous;
				snprintf(g_tags_error, sizeof(g_tags_error),
					"[cannot index tags.json]");
				return false;
			}
		}
		str_free(&line);
	}
	free(text);
	auto (info, sok) = fs_stat(path);
	if (!sok) {
		tags_invalidate();
		g_tags = previous;
		return false;
	}
	if (!tag_db_rebuild_index()) {
		tags_invalidate();
		g_tags = previous;
		snprintf(g_tags_error, sizeof(g_tags_error), "[cannot index tags.json]");
		return false;
	}
	snprintf(g_tags.path, sizeof(g_tags.path), "%s", path);
	g_tags.mtime = info.mtime_ns;
	tag_db_free(&previous);
	return true;
}

static void tag_file_name(char *dst, size_t cap) {
	auto (ok, env) = os_env("JEM_TAGS_FILE");
	if (ok && len(env) > 0 && ptr(env) != NULL) {
		int n = snprintf(dst, cap, "%s", env);
		if (n >= 0 && (size_t)n < cap) {
			return;
		}
	}
	snprintf(dst, cap, "tags.json");
}

static bool tag_start_dir(char *dst, size_t cap) {
	Buffer *cur = buffer_current();
	char tmp[TagPathMax] = {0};
	if (cur != NULL && len(cur.file_name) > 0) {
		char[..] d = path_dir(cur.file_name);
		if (len(d) > 0 && ptr(d) != NULL) {
			int n = snprintf(dst, cap, "%s", d);
			if (n >= 0 && (size_t)n < cap) {
				return true;
			}
		}
	}
	{
		auto (cw, ok) = os_cwd(ranged(tmp, 0, sizeof(tmp)));
		if (!ok || len(cw) == 0) {
			return false;
		}
		int n = snprintf(dst, cap, "%s", cw);
		if (n < 0 || (size_t)n >= cap) {
			return false;
		}
	}
	return true;
}

static bool tag_resolve_path(char *out, size_t cap) {
	char tag_name[TagNameMax] = {0};
	char dir[TagPathMax] = {0};
	char cand[TagPathMax] = {0};
	char prev[TagPathMax] = {0};
	tag_file_name(tag_name, sizeof(tag_name));
	char[..] tag_namev = ranged(tag_name, strlen(tag_name));
	if (path_is_abs(tag_namev)) {
		auto (inf, ok) = fs_stat(tag_namev);
		(void)inf;
		if (!ok) {
			return false;
		}
		if (strlen(tag_name) >= cap) {
			return false;
		}
		snprintf(out, cap, "%s", tag_name);
		return true;
	}
	if (!tag_start_dir(dir, sizeof(dir))) {
		return false;
	}
	for (;;) {
		char[..] dirv = ranged(dir, strlen(dir));
		auto (jv, jok) = path_join(ranged(cand, 0, sizeof(cand)),
			dirv, tag_namev);
		if (jok && len(jv) > 0) {
			auto (inf, ok) = fs_stat(jv);
			(void)inf;
			if (ok) {
				int n = snprintf(out, cap, "%s", jv);
				if (n < 0 || (size_t)n >= cap) {
					return false;
				}
				return true;
			}
		}
		snprintf(prev, sizeof(prev), "%s", dir);
		{
			char[..] parent = path_dir(dirv);
			int n = 0;
			if (len(parent) == 0 || ptr(parent) == NULL) {
				return false;
			}
			n = snprintf(dir, sizeof(dir), "%s", parent);
			if (n < 0 || (size_t)n >= sizeof(dir)) {
				return false;
			}
		}
		if (strcmp(dir, prev) == 0) {
			return false;
		}
	}
}

static void tag_search_anchor(char *dst, size_t cap) {
	char tag_name[TagNameMax] = {0};
	char dir[TagPathMax] = {0};
	tag_file_name(tag_name, sizeof(tag_name));
	if (path_is_abs(ranged(tag_name, strlen(tag_name)))) {
		snprintf(dst, cap, "abs:%s", tag_name);
		return;
	}
	if (!tag_start_dir(dir, sizeof(dir))) {
		snprintf(dst, cap, "walk:");
		return;
	}
	snprintf(dst, cap, "walk:%s", dir);
}

bool tags_ensure_loaded(bool quiet) {
	char path[TagPathMax] = {0};
	char anchor[TagPathMax] = {0};
	tag_search_anchor(anchor, sizeof(anchor));
	if (!tag_resolve_path(path, sizeof(path))) {
		if (!quiet) {
			display_set_echo("[no tags.json found; run M-x build-tags]");
		}
		return false;
	}
	if (g_tags.path[0] != 0 && strcmp(g_tags.path, path) == 0 && strcmp(g_tags.anchor, anchor) == 0) {
		auto (info, sok) = fs_stat(ranged(path, strlen(path)));
		if (sok && info.mtime_ns == g_tags.mtime) {
			return true;
		}
	}
	if (!tag_db_load(ranged(path, strlen(path)))) {
		if (!quiet) {
			if (g_tags_error[0] != 0) {
				display_set_echo(g_tags_error);
			} else {
				display_set_echo("[cannot load tags.json]");
			}
		}
		return false;
	}
	snprintf(g_tags.anchor, sizeof(g_tags.anchor), "%s", anchor);
	return true;
}

void tags_preload() {
	(void)tags_ensure_loaded(true);
}

static bool tag_is_symbol_char(char c) {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '_';
}

bool tags_symbol_at_point(char *dst, size_t cap) {
	Window *win = window_current();
	size_t end = 0;
	if (dst == NULL || cap < 2 || win == NULL || win.buffer == NULL) {
		return false;
	}
	Line *line = win.buffer.line(win.cursor.line);
	if (line == NULL) {
		return false;
	}
	char[..] v = line.view();
	size_t llen = len(v);
	if (llen == 0 || ptr(v) == NULL) {
		return false;
	}
	size_t offset = win.cursor.offset;
	if (offset > llen) {
		offset = llen;
	}
	size_t start = offset;
	if (start < llen && tag_is_symbol_char(v[start])) {
		end = start;
	} else if (start > 0 && tag_is_symbol_char(v[start - 1])) {
		start--;
		end = start;
	} else {
		return false;
	}
	while (start > 0 && tag_is_symbol_char(v[start - 1])) {
		start--;
	}
	while (end < llen && tag_is_symbol_char(v[end])) {
		end++;
	}
	if (end <= start || (end - start) >= cap) {
		return false;
	}
	memcpy(dst, ptr(v) + start, (end - start));
	{
		char[..] dv = ranged(dst, cap);
		dv[end - start] = 0;
	}
	return true;
}

bool tags_prefix_at_point(char *dst, size_t cap) {
	Window *win = window_current();
	if (dst == NULL || cap < 2 || win == NULL || win.buffer == NULL) {
		return false;
	}
	{
		char[..] dv = ranged(dst, cap);
		dv[0] = 0;
	}
	Line *line = win.buffer.line(win.cursor.line);
	if (line == NULL) {
		return false;
	}
	char[..] view = line.view();
	size_t end = win.cursor.offset;
	if (ptr(view) == NULL || end == 0) {
		return false;
	}
	if (end > len(view)) {
		end = len(view);
	}
	size_t start = end;
	while (start > 0 && tag_is_symbol_char(view[start - 1])) {
		start--;
	}
	if (start == end || (end - start) >= cap) {
		return false;
	}
	memcpy(dst, ptr(view) + start, (end - start));
	{
		char[..] dv = ranged(dst, cap);
		dv[end - start] = 0;
	}
	return true;
}

bool tags_ready() {
	return g_tags.path[0] != 0;
}

bool tags_paths_equal(const char[..] a, const char[..] b) {
	char aclean[TagPathMax] = {0};
	char bclean[TagPathMax] = {0};
	if (len(a) == 0 || len(b) == 0) {
		return false;
	}
	if (str_eq(a, b)) {
		return true;
	}
	if (file_resolve_path(aclean, sizeof(aclean), a) &&
		file_resolve_path(bclean, sizeof(bclean), b)) {
		return strcmp(aclean, bclean) == 0;
	}
	return false;
}

static void tag_scope_hint(char *dst, size_t cap) {
	Window *win = window_current();
	if (dst == NULL || cap < 2) {
		return;
	}
	{
		char[..] dv = ranged(dst, cap);
		dv[0] = 0;
	}
	if (win == NULL || win.buffer == NULL) {
		return;
	}
	Line *line = win.buffer.line(win.cursor.line);
	if (line == NULL) {
		return;
	}
	char[..] view = line.view();
	size_t offset = win.cursor.offset;
	if (ptr(view) == NULL || len(view) == 0) {
		return;
	}
	if (offset > len(view)) {
		offset = len(view);
	}
	size_t start = offset;
	if (start < len(view) && tag_is_symbol_char(view[start])) {
		/* Cursor is on the member name. */
	} else if (start > 0 && tag_is_symbol_char(view[start - 1])) {
		start--;
	} else {
		return;
	}
	while (start > 0 && tag_is_symbol_char(view[start - 1])) {
		start--;
	}
	if (start == 0 || view[start - 1] != '.') {
		return;
	}
	size_t end = start - 1;
	start = end;
	while (start > 0 && tag_is_symbol_char(view[start - 1])) {
		start--;
	}
	if (start == end || (end - start) >= cap) {
		return;
	}
	memcpy(dst, ptr(view) + start, (end - start));
	{
		char[..] dv = ranged(dst, cap);
		dv[end - start] = 0;
	}
}

static bool tag_ascii_eq(const char[..] a, const char[..] b) {
	size_t i = 0;
	size_t n = len(a);
	if (n != len(b)) {
		return false;
	}
	for (; i < n; i++) {
		char ca = a[i];
		char cb = b[i];
		if (ca >= 'A' && ca <= 'Z') {
			ca = (char)(ca + ('a' - 'A'));
		}
		if (cb >= 'A' && cb <= 'Z') {
			cb = (char)(cb + ('a' - 'A'));
		}
		if (ca != cb) {
			return false;
		}
	}
	return true;
}

static bool tag_scope_matches(const char[..] scope, char *hint) {
	if (str_is_empty(scope) || hint == NULL || *hint == 0) {
		return false;
	}
	const char[..] hv = str_from_cstr(hint);
	if (str_eq(scope, hv)) {
		return true;
	}
	size_t slen = len(scope);
	size_t hlen = len(hv);
	return slen > hlen && scope[slen - hlen - 1] == '.' &&
		str_eq(scope[slen - hlen ..], hv);
}

static int tag_kind_score(const char[..] kind, bool want_member) {
	if (str_is_empty(kind)) {
		return 5;
	}
	if (tag_ascii_eq(kind, "member") ||
		tag_ascii_eq(kind, "field") ||
		tag_ascii_eq(kind, "property")) {
		return want_member ? 45 : 20;
	}
	if (tag_ascii_eq(kind, "method")) {
		return want_member ? 42 : 40;
	}
	if (tag_ascii_eq(kind, "function") ||
		tag_ascii_eq(kind, "func")) {
		return want_member ? 25 : 40;
	}
	if (tag_ascii_eq(kind, "class") ||
		tag_ascii_eq(kind, "struct") ||
		tag_ascii_eq(kind, "type") ||
		tag_ascii_eq(kind, "interface") ||
		tag_ascii_eq(kind, "typedef") ||
		tag_ascii_eq(kind, "enum")) {
		return 30;
	}
	if (tag_ascii_eq(kind, "prototype")) {
		return 15;
	}
	if (tag_ascii_eq(kind, "macro")) {
		return 10;
	}
	return 8;
}

static int tag_score(TagEntry *e, const char[..] cur_path, char *scope_hint) {
	bool want_member = scope_hint != NULL && *scope_hint != 0;
	if (e == NULL) {
		return 0;
	}
	int score = tag_kind_score(e.kind, want_member);
	if (len(cur_path) > 0 && !str_is_empty(e.path) &&
		tags_paths_equal(cur_path, e.path)) {
		score += 100;
	}
	if (want_member && tag_scope_matches(e.scope, scope_hint)) {
		score += 80;
	}
	return score;
}

static int tag_nullable_cmp(const char[..] a, const char[..] b) {
	return str_cmp(a, b);
}

static int tag_rank_cmp(TagEntry *a, TagEntry *b, const char[..] cur_path,
	char *scope_hint) {
	int ascore = tag_score(a, cur_path, scope_hint);
	int bscore = tag_score(b, cur_path, scope_hint);
	char apath[TagPathMax] = {0};
	char bpath[TagPathMax] = {0};
	if (ascore != bscore) {
		return ascore > bscore ? -1 : 1;
	}
	int cmp = tag_nullable_cmp(a.name, b.name);
	if (cmp != 0) {
		return cmp;
	}
	/* Prefer display (project-relative) paths like jem.go tagCompareRanked. */
	tags_display_path(a, apath, sizeof(apath));
	tags_display_path(b, bpath, sizeof(bpath));
	cmp = strcmp(apath, bpath);
	if (cmp != 0) {
		return cmp;
	}
	if (a.line < b.line) {
		return -1;
	}
	if (a.line > b.line) {
		return 1;
	}
	return 0;
}

static void tag_list_sort_ranked(TagList *list) {
	Buffer *cur = buffer_current();
	char[..] cur_path = cur != NULL ? cur.file_name : str_empty();
	char scope_hint[TagNameMax] = {0};
	size_t i = 1;
	tag_scope_hint(scope_hint, sizeof(scope_hint));
	for (; i < len(list.items); i++) {
		TagEntry *key = list.items[i];
		size_t j = i;
		while (j > 0 && tag_rank_cmp(list.items[j - 1], key,
			cur_path, scope_hint) > 0) {
			list.items[j] = list.items[j - 1];
			j--;
		}
		list.items[j] = key;
	}
}

bool tags_collect_matches(const char[..] name, TagList *out) {
	if (str_is_empty(name) || out == NULL || !tags_ready()) {
		return false;
	}
	tags_list_free(out);
	size_t i = tag_index_lower_bound(name);
	while (i < len(g_tags.by_name) && str_eq(g_tags.by_name[i].name, name)) {
		if (!tag_list_append(out, g_tags.by_name[i])) {
			tags_list_free(out);
			return false;
		}
		i++;
	}
	tag_list_sort_ranked(out);
	return true;
}

bool tags_collect_completions(const char[..] prefix, TagList *out) {
	Buffer *cur = buffer_current();
	char[..] cur_path = cur != NULL ? cur.file_name : str_empty();
	char scope_hint[TagNameMax] = {0};
	if (str_is_empty(prefix) || out == NULL || !tags_ready()) {
		return false;
	}
	tags_list_free(out);
	size_t plen = len(prefix);
	tag_scope_hint(scope_hint, sizeof(scope_hint));
	size_t i = tag_index_lower_bound(prefix);
	while (i < len(g_tags.by_name)) {
		TagEntry *best = NULL;
		const char[..] name = g_tags.by_name[i].name;
		size_t j = i;
		if (str_is_empty(name) || len(name) < plen ||
			!str_eq(name[0 .. plen], prefix)) {
			break;
		}
		while (j < len(g_tags.by_name) &&
			str_eq(g_tags.by_name[j].name, name)) {
			TagEntry *candidate = g_tags.by_name[j];
			if (best == NULL || tag_rank_cmp(candidate, best,
				cur_path, scope_hint) < 0) {
				best = candidate;
			}
			j++;
		}
		{
			auto (found, _) = str_find(name, ".");
			if (!str_eq(name, prefix) && !found &&
				!tag_list_append(out, best)) {
				tags_list_free(out);
				return false;
			}
		}
		i = j;
	}
	tag_list_sort_ranked(out);
	return true;
}

static int tag_collect_call_context(Window *win, char *out, size_t cap) {
	char reversed[512] = {0};
	size_t used = 0;
	int lines = 1;
	if (win == NULL || win.buffer == NULL || out == NULL || cap < 2) {
		return 0;
	}
	int line_number = win.cursor.line;
	size_t offset = win.cursor.offset;
	Line *line = win.buffer.line(line_number);
	if (line != NULL && offset > line.byte_len()) {
		offset = line.byte_len();
	}
	while (used + 1 < sizeof(reversed) && used + 1 < cap) {
		if (offset == 0) {
			if (line_number <= 1 || lines >= 8) {
				break;
			}
			reversed[used++] = '\n';
			line_number--;
			line = win.buffer.line(line_number);
			offset = line != NULL ? line.byte_len() : 0;
			lines++;
			continue;
		}
		if (line == NULL) {
			break;
		}
		{
			char[..] view = line.view();
			if (ptr(view) == NULL || offset > len(view)) {
				break;
			}
			reversed[used++] = view[offset - 1];
		}
		offset--;
	}
	{
		size_t i = 0;
		char[..] ov = ranged(out, cap);
		for (; i < used; i++) {
			ov[i] = reversed[used - i - 1];
		}
		ov[used] = 0;
	}
	return (int)used;
}

static bool tag_find_call_hint(Window *win, char *name, size_t name_cap,
	char *scope, size_t scope_cap) {
	char context[513] = {0};
	int length = 0;
	int open = 0;
	int depth = 0;
	bool found = false;
	int i = length;
	if (name == NULL || name_cap < 2 || scope == NULL || scope_cap < 2) {
		return false;
	}
	{
		char[..] nv = ranged(name, name_cap);
		char[..] sv = ranged(scope, scope_cap);
		nv[0] = 0;
		sv[0] = 0;
	}
	length = tag_collect_call_context(win, context, sizeof(context));
	for (; i > 0;) {
		char ch = context[--i];
		if (ch == ')' || ch == ']' || ch == '}') {
			depth++;
		} else if (ch == '(' || ch == '[' || ch == '{') {
			if (depth > 0) {
				depth--;
			} else if (ch == '(') {
				open = i;
				found = true;
				break;
			}
		}
	}
	if (!found) {
		return false;
	}
	int end = open;
	while (end > 0 && isspace(context[end - 1])) {
		end--;
	}
	int start = end;
	while (start > 0 &&
		tag_is_symbol_char(context[start - 1])) {
		start--;
	}
	if (start == end || (size_t)(end - start) >= name_cap) {
		return false;
	}
	memcpy(name, context + start, (end - start));
	{
		char[..] nv = ranged(name, name_cap);
		nv[end - start] = 0;
	}
	if (start > 0 && context[start - 1] == '.') {
		int receiver_end = start - 1;
		int receiver_start = receiver_end;
		while (receiver_start > 0 &&
			tag_is_symbol_char(context[receiver_start - 1])) {
			receiver_start--;
		}
		if (receiver_start < receiver_end &&
			(size_t)(receiver_end - receiver_start) < scope_cap) {
			memcpy(scope, context + receiver_start,
				(receiver_end - receiver_start));
			{
				char[..] sv = ranged(scope, scope_cap);
				sv[receiver_end - receiver_start] = 0;
			}
		}
	}
	return true;
}

void tags_maybe_show_call_hint() {
	Window *win = window_current();
	Buffer *buf = buffer_current();
	char name[TagNameMax] = {0};
	char scope[TagNameMax] = {0};
	TagEntry *best = NULL;
	char[..] cur_path = buf != NULL ? buf.file_name : str_empty();
	int best_score = -1;
	if (!tags_ready() || win == NULL ||
		!tag_find_call_hint(win, name, sizeof(name), scope, sizeof(scope))) {
		return;
	}
	const char[..] nv = ranged(name, strlen(name));
	size_t i = tag_index_lower_bound(nv);
	while (i < len(g_tags.by_name) && str_eq(g_tags.by_name[i].name, nv)) {
		TagEntry *entry = g_tags.by_name[i++];
		int score = 0;
		if (str_is_empty(entry.signature)) {
			continue;
		}
		score = tag_score(entry, cur_path, scope);
		if (best == NULL || score > best_score) {
			best = entry;
			best_score = score;
		}
	}
	if (best != NULL) {
		char message[TagNameMax + 768] = {0};
		snprintf(message, sizeof(message), "%s%s", name, best.signature);
		display_set_echo(message);
	}
}

void tags_format_annotation(TagEntry *entry, char *dst, size_t cap) {
	const char[..] kind_default = "tag";
	const char[..] kind = kind_default;
	if (dst == NULL || cap == 0) {
		return;
	}
	{ char[..] __dv = ranged(dst, cap); __dv[0] = 0; };
	if (entry == NULL) {
		return;
	}
	if (!str_is_empty(entry.kind)) {
		kind = entry.kind;
	}
	const char[..] typeref = entry.typeref;
	if (str_starts_with(typeref, "typename:")) {
		typeref = typeref[9 ..];
	}
	if (!str_is_empty(typeref) && !str_is_empty(entry.scope)) {
		snprintf(dst, cap, "(%s/%s@%s)", kind, typeref, entry.scope);
	} else if (!str_is_empty(typeref)) {
		snprintf(dst, cap, "(%s/%s)", kind, typeref);
	} else if (!str_is_empty(entry.scope)) {
		snprintf(dst, cap, "(%s@%s)", kind, entry.scope);
	} else {
		snprintf(dst, cap, "(%s)", kind);
	}
}

void tags_display_path(TagEntry *entry, char *dst, size_t cap) {
	char root[TagPathMax] = {0};
	if (dst == NULL || cap == 0) {
		return;
	}
	{ char[..] __dv = ranged(dst, cap); __dv[0] = 0; };
	if (entry == NULL || str_is_empty(entry.path)) {
		return;
	}
	char[..] dir = path_dir(ranged(g_tags.path, strlen(g_tags.path)));
	if (ptr(dir) != NULL && len(dir) > 0) {
		int n = snprintf(root, sizeof(root), "%s", dir);
		if (n < 0 || (size_t)n >= sizeof(root)) {
			root[0] = 0;
		}
	}
	size_t rlen = strlen(root);
	{
		const char[..] pv = entry.path;
		if (rlen > 0 && rlen < len(pv) &&
			str_eq(pv[0 .. rlen], ranged(root, rlen)) &&
			pv[rlen] == '/') {
			snprintf(dst, cap, "%s", pv[rlen + 1 ..]);
		} else {
			snprintf(dst, cap, "%s", pv);
		}
	}
}

static bool tag_qualified_duplicate(TagList *list, TagEntry *entry) {
	if (entry == NULL || str_is_empty(entry.name)) {
		return false;
	}
	{
		auto (found, _) = str_find(entry.name, ".");
		if (!found) {
			return false;
		}
	}
	for (auto other : list.items) {
		if (other == NULL || str_is_empty(other.name) ||
			other.line != entry.line ||
			tag_nullable_cmp(other.kind, entry.kind) != 0) {
			continue;
		}
		auto (found, _) = str_find(other.name, ".");
		if (!found) {
			return true;
		}
	}
	return false;
}

bool tags_collect_in_file(const char[..] file_path, TagList *out) {
	size_t i = 0;
	if (len(file_path) == 0 || out == NULL) {
		return false;
	}
	tags_list_free(out);
	for (auto *entry : g_tags.entries) {
		if (!str_is_empty(entry.path) &&
			tags_paths_equal(entry.path, file_path)) {
			if (!tag_list_append(out, entry)) {
				tags_list_free(out);
				return false;
			}
		}
	}
	for (; i < len(out.items);) {
		if (tag_qualified_duplicate(out, out.items[i])) {
			size_t j = i;
			size_t n = len(out.items);
			tag_entry_free(out.items[i]);
			free(out.items[i]);
			for (j = i; j + 1 < n; j++) {
				out.items[j] = out.items[j + 1];
			}
			out.items = ranged(ptr(out.items), n - 1, cap(out.items));
		} else {
			i++;
		}
	}
	for (i = 1; i < len(out.items); i++) {
		TagEntry *key = out.items[i];
		size_t j = i;
		while (j > 0 && out.items[j - 1].line > key.line) {
			out.items[j] = out.items[j - 1];
			j--;
		}
		out.items[j] = key;
	}
	return true;
}

static bool tag_pattern_needle(const char[..] pattern, char *dst, size_t cap,
	bool *anchored) {
	size_t start = 1;
	size_t out = 0;
	if (str_is_empty(pattern) || dst == NULL || cap < 2 || anchored == NULL) {
		return false;
	}
	const char[..] pv = pattern;
	char[..] dv = ranged(dst, cap);
	size_t n = len(pv);
	if (n < 3) {
		return false;
	}
	char delim = pv[0];
	if ((delim != '/' && delim != '?') || pv[n - 1] != delim) {
		return false;
	}
	size_t end = n - 1;
	*anchored = start < end && pv[start] == '^' &&
		pv[end - 1] == '$';
	if (pv[start] == '^') {
		start++;
	}
	if (end > start && pv[end - 1] == '$') {
		end--;
	}
	while (start < end) {
		char ch = pv[start++];
		if (ch == '\\' && start < end) {
			ch = pv[start++];
		}
		if (out + 1 >= cap) {
			return false;
		}
		dv[out++] = ch;
	}
	dv[out] = 0;
	return out > 0;
}

static bool tag_line_matches(Line *line, char *needle, bool anchored) {
	if (line == NULL || needle == NULL) {
		return false;
	}
	char[..] view = line.view();
	size_t nlen = strlen(needle);
	if (anchored) {
		return len(view) == nlen && (nlen == 0 ||
			memcmp(ptr(view), needle, nlen) == 0);
	}
	if (nlen == 0 || len(view) < nlen || ptr(view) == NULL) {
		return false;
	}
	{
		size_t i = 0;
		for (; i + nlen <= len(view); i++) {
			if (memcmp(ptr(view) + i, needle, nlen) == 0) {
				return true;
			}
		}
	}
	return false;
}

int tags_resolve_line(Buffer *buf, TagEntry *entry) {
	char needle[TagPatMax] = {0};
	bool anchored = false;
	int distance = 1;
	if (entry == NULL || entry.line < 1 || buf == NULL ||
		!tag_pattern_needle(entry.pattern, needle, sizeof(needle), &anchored)) {
		return entry != NULL ? entry.line : 0;
	}
	int count = buf.line_count_i();
	if (count < 1) {
		return entry.line;
	}
	int hint = entry.line;
	if (hint > count) {
		hint = count;
	}
	if (tag_line_matches(buf.line(hint), needle, anchored)) {
		return hint;
	}
	for (; distance < count; distance++) {
		int down = hint + distance;
		int up = hint - distance;
		if (down <= count && tag_line_matches(buf.line(down), needle, anchored)) {
			return down;
		}
		if (up >= 1 && tag_line_matches(buf.line(up), needle, anchored)) {
			return up;
		}
	}
	return entry.line;
}

bool tags_visit_entry(TagEntry *e) {
	Window *w = window_current();
	if (e == NULL || str_is_empty(e.path) || e.line < 1) {
		return false;
	}
	if (w != NULL) {
		markring_push(w.cursor);
	}
	auto (buf, ok) = file_visit(e.path);
	if (!ok || buf == NULL) {
		return false;
	}
	int line = tags_resolve_line(buf, e);
	return file_visit_location(e.path, line, 1);
}

static Buffer *tags_ensure_list_buf() {
	Buffer *buf = buffer_find_by_name("*tags*");
	if (buf != NULL) {
		return buf;
	}
	buf = buffer_new();
	if (buf == NULL) {
		return NULL;
	}
	buf.set_name("*tags*");
	if (!buffer_list_add(buf)) {
		return NULL;
	}
	return buffer_find_by_name("*tags*");
}

static void tags_present_list(Buffer *buf) {
	if (buf == NULL) {
		return;
	}
	{
		Window *cur = window_current();
		if (cur != NULL) {
			markring_push(cur.cursor);
		}
	}
	buffer_set_current(buf);
	window_switch_buffer(buf);
	Window *w = window_current();
	if (w == NULL) {
		return;
	}
	Location loc = { .line = 1, .offset = 0 };
	w.set_cursor(loc);
	w.should_redraw = true;
	w.should_update_mode_line = true;
}

static bool tags_show_match_list(TagList *matches, char *title) {
	char header[256] = {0};
	if (matches == NULL || len(matches.items) == 0) {
		return false;
	}
	Buffer *buf = tags_ensure_list_buf();
	if (buf == NULL) {
		return false;
	}
	bool was_ro = buf.is_readonly;
	buf.is_readonly = false;
	buf.clear();
	buf.set_file_name(str_empty());
	buf.lang_mode = LModeMarkdown;
	buf.is_changed = false;
	if (title != NULL) {
		snprintf(header, sizeof(header), "# %s", title);
		if (buf.append_line(ranged(header, strlen(header))) == NULL) {
			buf.is_readonly = was_ro;
			return false;
		}
	}
	for (auto e : matches.items) {
		char linebuf[TagPathMax + TagNameMax + 64] = {0};
		Line *ln = NULL;
		const char[..] kind = "tag";
		if (e == NULL || str_is_empty(e.path) || str_is_empty(e.name)) {
			continue;
		}
		if (!str_is_empty(e.kind)) {
			kind = e.kind;
		}
		snprintf(linebuf, sizeof(linebuf), "%s  %s:%d  (%s)", e.name, e.path,
			e.line, kind);
		ln = buf.append_line(ranged(linebuf, strlen(linebuf)));
		if (ln == NULL) {
			buf.is_readonly = true;
			return false;
		}
		ln.metadata = tags_loc_alloc(e.path, e.line, 1, e.pattern);
	}
	buf.is_readonly = true;
	tags_present_list(buf);
	return true;
}

bool tags_goto_name(const char[..] name) {
	TagList matches = {0};
	if (str_is_empty(name)) {
		display_set_echo("[empty tag name]");
		return false;
	}
	if (!tags_ensure_loaded(false)) {
		return false;
	}
	if (!tags_collect_matches(name, &matches)) {
		display_set_echo("[cannot collect tags]");
		return false;
	}
	if (len(matches.items) == 0) {
		char msg[TagNameMax + 32] = {0};
		snprintf(msg, sizeof(msg), "[tag not found: %s]", name);
		display_set_echo(msg);
		tags_list_free(&matches);
		return false;
	}
	if (len(matches.items) == 1) {
		bool ok = tags_visit_entry(matches.items[0]);
		tags_list_free(&matches);
		return ok;
	}
	if (!tags_picker_open(&matches, TagPickerGoto, "Tag: ", true)) {
		tags_list_free(&matches);
		return false;
	}
	return true;
}

bool tags_goto_buffer() {
	Buffer *cur = buffer_current();
	TagList matches = {0};
	if (cur == NULL || len(cur.file_name) == 0) {
		display_set_echo("[no file for buffer]");
		return false;
	}
	if (!tags_ensure_loaded(false)) {
		return false;
	}
	if (!tags_collect_in_file(cur.file_name, &matches)) {
		display_set_echo("[cannot collect buffer tags]");
		return false;
	}
	if (len(matches.items) == 0) {
		display_set_echo("[no tags in buffer]");
		tags_list_free(&matches);
		return false;
	}
	if (!tags_picker_open(&matches, TagPickerGoto, "Symbol: ", false)) {
		tags_list_free(&matches);
		return false;
	}
	return true;
}

bool tags_visit_at_point() {
	Window *w = window_current();
	TagEntry entry = {0};
	if (w == NULL || w.buffer == NULL) {
		return false;
	}
	Buffer *buf = w.buffer;
	if (len(buf.name) == 0 || (!str_eq(buf.name, "*tags*") &&
		!str_eq(buf.name, "*describe*"))) {
		return false;
	}
	Line *line = buf.line(w.cursor.line);
	if (line == NULL || line.metadata == NULL) {
		return false;
	}
	TagLocHead *h = (TagLocHead *)line.metadata;
	char *p = tag_loc_path(h);
	if (p == NULL || *p == 0) {
		return false;
	}
	/* Views into loc metadata; not owned — do not str_free. */
	entry.path = str_from_cstr(p);
	entry.pattern = str_from_cstr(tag_loc_pattern(h));
	entry.line = h.line;
	return tags_visit_entry(&entry);
}

static bool tags_project_root(char *dst, size_t cap) {
	char dir[TagPathMax] = {0};
	char prev[TagPathMax] = {0};
	char git[TagPathMax] = {0};
	if (!tag_start_dir(dir, sizeof(dir))) {
		return false;
	}
	for (;;) {
		char[..] dirv = ranged(dir, strlen(dir));
		auto (jv, jok) = path_join(ranged(git, 0, sizeof(git)),
			dirv, ".git");
		if (jok && len(jv) > 0) {
			auto (inf, ok) = fs_stat(jv);
			(void)inf;
			if (ok) {
				if (strlen(dir) >= cap) {
					return false;
				}
				snprintf(dst, cap, "%s", dir);
				return true;
			}
		}
		snprintf(prev, sizeof(prev), "%s", dir);
		{
			char[..] parent = path_dir(dirv);
			if (len(parent) == 0 || ptr(parent) == NULL) {
				break;
			}
			int n = snprintf(dir, sizeof(dir), "%s", parent);
			if (n < 0 || (size_t)n >= sizeof(dir)) {
				return false;
			}
		}
		if (strcmp(dir, prev) == 0) {
			break;
		}
	}
	return tag_start_dir(dst, cap);
}

static bool tags_make_temp(char *dst, size_t cap, const char[..] target) {
	if (dst == NULL || cap < 2 || len(target) == 0) {
		return false;
	}
	const char[..] parent = path_dir(target);
	if (len(parent) == 0 || ptr(parent) == NULL) {
		return false;
	}
	int n = snprintf(dst, cap, "%s/.jem-tags-XXXXXX", parent);
	if (n < 0 || (size_t)n >= cap) {
		return false;
	}
	int fd = mkstemp(dst);
	if (fd < 0) {
		{ char[..] __dv = ranged(dst, cap); __dv[0] = 0; };
		return false;
	}
	close(fd);
	return true;
}

static bool tags_is_universal_ctags(const char[..] path_name) {
	Cmd cmd = {0};
	const char[..] args[1] = {0};
	char text[512] = {0};
	size_t used = 0;
	if (len(path_name) == 0) {
		return false;
	}
	args[0] = "--version";
	cmd.init(path_name, args, 1);
	cmd.stdio(StdErr, IoNull);
	auto (output, ok) = cmd.pipe(StdOut);
	if (!ok) {
		cmd.close();
		return false;
	}
	if (!cmd.start()) {
		output.close();
		cmd.close();
		return false;
	}
	auto (exit_code, waited) = cmd.wait();
	while (used + 1 < sizeof(text)) {
		auto (n, rok) = output.read(ranged(text + used, 0,
			sizeof(text) - used - 1));
		if (!rok || n == 0) {
			break;
		}
		used += n;
	}
	text[used] = 0;
	output.close();
	cmd.close();
	return waited && exit_code == 0 &&
		strstr(text, "Universal Ctags") != NULL;
}

bool tags_build() {
	char root[TagPathMax] = {0};
	char ctags[TagPathMax] = {0};
	char tag_name[TagNameMax] = {0};
	char exclude_name[TagNameMax] = {0};
	char target[TagPathMax] = {0};
	char temp[TagPathMax] = {0};
	tag_file_name(tag_name, sizeof(tag_name));
	{
		auto (found, ok) = os_look_path(ranged(ctags, 0, sizeof(ctags)), "ctags");
		(void)found;
		if (!ok) {
			display_set_echo("[universal-ctags not found in PATH]");
			return false;
		}
	}
	if (!tags_is_universal_ctags(ranged(ctags, strlen(ctags)))) {
		display_set_echo("[jem requires Universal Ctags]");
		return false;
	}
	if (!tags_project_root(root, sizeof(root))) {
		display_set_echo("[cannot determine project root]");
		return false;
	}
	if (job_running()) {
		display_set_echo("[busy: background job running]");
		return false;
	}
	char[..] tag_namev = ranged(tag_name, strlen(tag_name));
	if (path_is_abs(tag_namev)) {
		if (strlen(tag_name) >= sizeof(target)) {
			display_set_echo("[tags output path is too long]");
			return false;
		}
		snprintf(target, sizeof(target), "%s", tag_name);
	} else {
		auto (tv, ok) = path_join(ranged(target, 0, sizeof(target)),
			ranged(root, strlen(root)), tag_namev);
		if (!ok || len(tv) == 0) {
			display_set_echo("[cannot determine tags output path]");
			return false;
		}
	}
	if (!tags_make_temp(temp, sizeof(temp), ranged(target, strlen(target)))) {
		display_set_echo("[cannot create temporary tags file]");
		return false;
	}
	{
		char[..] base = path_base(ranged(target, strlen(target)));
		if (len(base) == 0 || ptr(base) == NULL) {
			fs_remove(ranged(temp, strlen(temp)));
			display_set_echo("[invalid tags output path]");
			return false;
		}
		int n = snprintf(exclude_name, sizeof(exclude_name), "%s", base);
		if (n < 0 || (size_t)n >= sizeof(exclude_name)) {
			fs_remove(ranged(temp, strlen(temp)));
			display_set_echo("[invalid tags output path]");
			return false;
		}
	}
	return job_start_tags(ctags, root, target, temp, exclude_name);
}

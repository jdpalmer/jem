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
	char *name;
	char *path;
	char *kind;
	char *signature;
	char *scope;
	char *typeref;
	char *pattern;
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

static char *tag_strdup(char *s) {
	size_t n = 0;
	char *p = NULL;
	if (s == NULL) {
		return NULL;
	}
	n = strlen(s);
	p = malloc(n + 1);
	if (p == NULL) {
		return NULL;
	}
	memcpy(p, s, n + 1);
	return p;
}

static char *tag_strdup_n(char *s, size_t n) {
	char *p = NULL;
	if (s == NULL) {
		return NULL;
	}
	p = malloc(n + 1);
	if (p == NULL) {
		return NULL;
	}
	memcpy(p, s, n);
	p[n] = 0;
	return p;
}

static void tag_entry_free(TagEntry *e) {
	if (e == NULL) {
		return;
	}
	free(e.name);
	free(e.path);
	free(e.kind);
	free(e.signature);
	free(e.scope);
	free(e.typeref);
	free(e.pattern);
	e.name = NULL;
	e.path = NULL;
	e.kind = NULL;
	e.signature = NULL;
	e.scope = NULL;
	e.typeref = NULL;
	e.pattern = NULL;
	e.line = 0;
}

static void tag_db_free(TagDb *db) {
	if (db == NULL) {
		return;
	}
	for (auto *e : db.entries) {
		tag_entry_free(e);
	}
	free(ptr(db.entries));
	free(ptr(db.by_name));
	memset(db, 0, sizeof(*db));
}

void tags_invalidate() {
	tag_db_free(&g_tags);
}

static bool tag_db_reserve(size_t need) {
	TagEntry *grown = {0};
	size_t ncap = 0;
	if (need <= cap(g_tags.entries)) {
		return true;
	}
	ncap = cap(g_tags.entries) != 0 ? cap(g_tags.entries) : 256;
	while (ncap < need) {
		ncap = ncap * 2;
	}
	grown = realloc(ptr(g_tags.entries), ncap * sizeof(TagEntry));
	if (grown == NULL) {
		return false;
	}
	g_tags.entries = ranged(grown, len(g_tags.entries), ncap);
	return true;
}

static bool tag_db_append(TagEntry *e) {
	size_t n = 0;
	if (e == NULL) {
		return false;
	}
	n = len(g_tags.entries);
	if (!tag_db_reserve(n + 1)) {
		return false;
	}
	g_tags.entries = ranged(ptr(g_tags.entries), n + 1, cap(g_tags.entries));
	g_tags.entries[n] = *e;
	e.name = NULL;
	e.path = NULL;
	e.kind = NULL;
	e.signature = NULL;
	e.scope = NULL;
	e.typeref = NULL;
	e.pattern = NULL;
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
	free(ptr(list.items));
	list.items = ranged((TagEntry **)0, 0);
}

static bool tag_list_reserve(TagList *list, size_t need) {
	TagEntry **grown = {0};
	size_t ncap = 0;
	if (list == NULL) {
		return false;
	}
	if (need <= cap(list.items)) {
		return true;
	}
	ncap = cap(list.items) != 0 ? cap(list.items) : 32;
	while (ncap < need) {
		ncap = ncap * 2;
	}
	grown = realloc(ptr(list.items), ncap * sizeof(TagEntry *));
	if (grown == NULL) {
		return false;
	}
	list.items = ranged(grown, len(list.items), ncap);
	return true;
}

static bool tag_list_append(TagList *list, TagEntry *entry) {
	TagEntry *copy = {0};
	size_t n = 0;
	if (list == NULL || entry == NULL) {
		return false;
	}
	n = len(list.items);
	if (!tag_list_reserve(list, n + 1)) {
		return false;
	}
	copy = calloc(1, sizeof(TagEntry));
	if (copy == NULL) {
		return false;
	}
	copy.name = tag_strdup(entry.name);
	copy.path = tag_strdup(entry.path);
	copy.kind = tag_strdup(entry.kind);
	copy.signature = tag_strdup(entry.signature);
	copy.scope = tag_strdup(entry.scope);
	copy.typeref = tag_strdup(entry.typeref);
	copy.pattern = tag_strdup(entry.pattern);
	copy.line = entry.line;
	if ((entry.name != NULL && copy.name == NULL) ||
		(entry.path != NULL && copy.path == NULL) ||
		(entry.kind != NULL && copy.kind == NULL) ||
		(entry.signature != NULL && copy.signature == NULL) ||
		(entry.scope != NULL && copy.scope == NULL) ||
		(entry.typeref != NULL && copy.typeref == NULL) ||
		(entry.pattern != NULL && copy.pattern == NULL)) {
		tag_entry_free(copy);
		free(copy);
		return false;
	}
	list.items = ranged(ptr(list.items), n + 1, cap(list.items));
	list.items[n] = copy;
	return true;
}

static int tag_entry_name_cmp(TagEntry *a, TagEntry *b) {
	int cmp = 0;
	if (a == NULL || a.name == NULL) {
		return b == NULL || b.name == NULL ? 0 : -1;
	}
	if (b == NULL || b.name == NULL) {
		return 1;
	}
	cmp = strcmp(a.name, b.name);
	if (cmp != 0) {
		return cmp;
	}
	if (a.path != NULL && b.path != NULL) {
		cmp = strcmp(a.path, b.path);
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
	TagEntry **tmp = NULL;
	TagEntry **indexed = NULL;
	size_t width = 1;
	size_t i = 0;
	size_t n = len(g_tags.entries);
	free(ptr(g_tags.by_name));
	g_tags.by_name = ranged((TagEntry **)0, 0);
	if (n == 0) {
		return true;
	}
	indexed = malloc(n * sizeof(TagEntry *));
	tmp = malloc(n * sizeof(TagEntry *));
	if (indexed == NULL || tmp == NULL) {
		free(indexed);
		free(tmp);
		return false;
	}
	g_tags.by_name = ranged(indexed, n, n);
	for (i = 0; i < n; i++) {
		g_tags.by_name[i] = &g_tags.entries[i];
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
	free(tmp);
	return true;
}

static size_t tag_index_lower_bound(char *name) {
	size_t lo = 0;
	size_t hi = len(g_tags.by_name);
	while (lo < hi) {
		size_t mid = lo + (hi - lo) / 2;
		char *at = g_tags.by_name[mid].name;
		if (at != NULL && strcmp(at, name) < 0) {
			lo = mid + 1;
		} else {
			hi = mid;
		}
	}
	return lo;
}

void *tags_loc_alloc(char *path, int line, int column, char *pattern) {
	size_t plen = 0;
	size_t patlen = 0;
	TagLocHead *h = NULL;
	if (path == NULL) {
		return NULL;
	}
	plen = strlen(path);
	if (pattern != NULL) {
		patlen = strlen(pattern);
	}
	h = malloc(sizeof(TagLocHead) + plen + 1 + patlen + 1);
	if (h == NULL) {
		return NULL;
	}
	h.line = line;
	h.column = column;
	h.pattern_offset = plen + 1;
	memcpy((char *)(h + 1), path, plen + 1);
	if (pattern != NULL) {
		memcpy((char *)(h + 1) + h.pattern_offset, pattern, patlen + 1);
	} else {
		*((char *)(h + 1) + h.pattern_offset) = 0;
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
	int parsed = TagJsonMalformed;
	if (line == NULL || out == NULL) {
		return TagJsonMalformed;
	}
	parsed = tag_json_parse_line(line, &f);
	if (parsed != (int)TagJsonEntry) {
		return parsed;
	}
	e.name = f.name;
	f.name = NULL;
	e.kind = f.kind;
	f.kind = NULL;
	e.signature = f.signature;
	f.signature = NULL;
	e.scope = f.scope;
	f.scope = NULL;
	e.typeref = f.typeref;
	f.typeref = NULL;
	e.pattern = f.pattern;
	f.pattern = NULL;
	e.line = f.line;
	if (path_is_abs(str_from_cstr(f.path))) {
		e.path = f.path;
		f.path = NULL;
	} else {
		auto (jv, jok) = path_join(ranged(resolved, 0, sizeof(resolved)),
			str_from_cstr(tags_dir), str_from_cstr(f.path));
		if (!jok || len(jv) == 0) {
			tag_entry_free(&e);
			tag_json_fields_free(&f);
			return TagJsonSkip;
		}
		e.path = tag_strdup_n(ptr(jv), len(jv));
	}
	tag_json_fields_free(&f);
	if (e.name == NULL || e.path == NULL) {
		tag_entry_free(&e);
		return TagJsonSkip;
	}
	*out = e;
	return TagJsonEntry;
}

(char *, size_t, bool) tags_read_file(char *path) {
	char[..] pv = str_from_cstr(path);
	char *data = NULL;
	size_t cap = 0;
	size_t len = 0;
	char chunk[4096] = {0};
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
		if (len + n + 1 > cap) {
			size_t ncap = cap != 0 ? cap * 2 : 8192;
			char *p = NULL;
			while (ncap < len + n + 1) {
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
		len = len + n;
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

static bool tag_db_load(char *path) {
	TagDb previous = g_tags;
	char tags_dir[TagPathMax] = {0};
	size_t i = 0;
	int source_line = 0;
	g_tags_error[0] = 0;
	memset(&g_tags, 0, sizeof(g_tags));
	auto (text, text_len, ok) = tags_read_file(path);
	if (!ok) {
		g_tags = previous;
		return false;
	}
	{
		char[..] pv = str_from_cstr(path);
		char[..] d = path_dir(pv);
		int n = 0;
		if (len(d) == 0 || ptr(d) == NULL) {
			/* keep empty tags_dir */
		} else {
			n = snprintf(tags_dir, sizeof(tags_dir), "%s", d);
			if (n < 0 || (size_t)n >= sizeof(tags_dir)) {
				tags_dir[0] = 0;
			}
		}
	}
	i = 0;
	while (i < text_len) {
		size_t start = i;
		size_t nline = 0;
		char *line = NULL;
		TagEntry e = {0};
		int parsed = TagJsonSkip;
		source_line++;
		while (i < text_len && text[i] != '\n') {
			i++;
		}
		nline = i - start;
		if (i < text_len && text[i] == '\n') {
			i++;
		}
		if (nline == 0) {
			continue;
		}
		if (text[start + nline - 1] == '\r') {
			nline--;
		}
		if (nline == 0) {
			continue;
		}
		line = tag_strdup_n(text + start, nline);
		if (line == NULL) {
			free(text);
			tags_invalidate();
			g_tags = previous;
			return false;
		}
		parsed = tag_parse_line(line, tags_dir, &e);
		if (parsed == (int)TagJsonMalformed) {
			free(line);
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
				free(line);
				free(text);
				tags_invalidate();
				g_tags = previous;
				snprintf(g_tags_error, sizeof(g_tags_error),
					"[cannot index tags.json]");
				return false;
			}
		}
		free(line);
	}
	free(text);
	auto (info, sok) = fs_stat(str_from_cstr(path));
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
		int n = 0;
		if (len(d) > 0 && ptr(d) != NULL) {
			n = snprintf(dst, cap, "%s", d);
			if (n >= 0 && (size_t)n < cap) {
				return true;
			}
		}
	}
	{
		auto (cw, ok) = os_cwd(ranged(tmp, 0, sizeof(tmp)));
		int n = 0;
		if (!ok || len(cw) == 0) {
			return false;
		}
		n = snprintf(dst, cap, "%s", cw);
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
	if (path_is_abs(str_from_cstr(tag_name))) {
		auto (inf, ok) = fs_stat(str_from_cstr(tag_name));
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
		auto (jv, jok) = path_join(ranged(cand, 0, sizeof(cand)),
			str_from_cstr(dir), str_from_cstr(tag_name));
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
			char[..] parent = path_dir(str_from_cstr(dir));
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
	if (path_is_abs(str_from_cstr(tag_name))) {
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
		auto (info, sok) = fs_stat(str_from_cstr(path));
		if (sok && info.mtime_ns == g_tags.mtime) {
			return true;
		}
	}
	if (!tag_db_load(path)) {
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
	Line *line = NULL;
	char[..] v = {0};
	size_t offset = 0;
	size_t start = 0;
	size_t end = 0;
	size_t llen = 0;
	char *vp = NULL;
	if (dst == NULL || cap < 2 || win == NULL || win.buffer == NULL) {
		return false;
	}
	line = win.buffer.line(win.cursor.line);
	if (line == NULL) {
		return false;
	}
	v = line.view();
	llen = len(v);
	if (llen == 0 || ptr(v) == NULL) {
		return false;
	}
	vp = ptr(v);
	offset = win.cursor.offset;
	if (offset > llen) {
		offset = llen;
	}
	start = offset;
	if (start < llen && tag_is_symbol_char(vp[start])) {
		end = start;
	} else if (start > 0 && tag_is_symbol_char(vp[start - 1])) {
		start--;
		end = start;
	} else {
		return false;
	}
	while (start > 0 && tag_is_symbol_char(vp[start - 1])) {
		start--;
	}
	while (end < llen && tag_is_symbol_char(vp[end])) {
		end++;
	}
	if (end <= start || (end - start) >= cap) {
		return false;
	}
	memcpy(dst, vp + start, (end - start));
	dst[end - start] = 0;
	return true;
}

bool tags_prefix_at_point(char *dst, size_t cap) {
	Window *win = window_current();
	Line *line = NULL;
	char[..] view = {0};
	char *p = NULL;
	size_t end = 0;
	size_t start = 0;
	if (dst == NULL || cap < 2 || win == NULL || win.buffer == NULL) {
		return false;
	}
	dst[0] = 0;
	line = win.buffer.line(win.cursor.line);
	if (line == NULL) {
		return false;
	}
	view = line.view();
	p = ptr(view);
	end = win.cursor.offset;
	if (p == NULL || end == 0) {
		return false;
	}
	if (end > len(view)) {
		end = len(view);
	}
	start = end;
	while (start > 0 && tag_is_symbol_char(p[start - 1])) {
		start--;
	}
	if (start == end || (end - start) >= cap) {
		return false;
	}
	memcpy(dst, p + start, (end - start));
	dst[end - start] = 0;
	return true;
}

bool tags_ready() {
	return g_tags.path[0] != 0;
}

bool tags_paths_equal(const char[..] a, const char[..] b) {
	char aclean[TagPathMax] = {0};
	char bclean[TagPathMax] = {0};
	char abuf[TagPathMax] = {0};
	char bbuf[TagPathMax] = {0};
	if (len(a) == 0 || len(b) == 0) {
		return false;
	}
	if (str_eq(a, b)) {
		return true;
	}
	if (snprintf(abuf, sizeof(abuf), "%s", a) < 0 ||
		snprintf(bbuf, sizeof(bbuf), "%s", b) < 0) {
		return false;
	}
	if (file_resolve_path(aclean, sizeof(aclean), abuf) &&
		file_resolve_path(bclean, sizeof(bclean), bbuf)) {
		return strcmp(aclean, bclean) == 0;
	}
	return false;
}

static void tag_scope_hint(char *dst, size_t cap) {
	Window *win = window_current();
	Line *line = NULL;
	char[..] view = {0};
	char *p = NULL;
	size_t offset = 0;
	size_t start = 0;
	size_t end = 0;
	if (dst == NULL || cap < 2) {
		return;
	}
	dst[0] = 0;
	if (win == NULL || win.buffer == NULL) {
		return;
	}
	line = win.buffer.line(win.cursor.line);
	if (line == NULL) {
		return;
	}
	view = line.view();
	p = ptr(view);
	offset = win.cursor.offset;
	if (p == NULL || len(view) == 0) {
		return;
	}
	if (offset > len(view)) {
		offset = len(view);
	}
	start = offset;
	if (start < len(view) && tag_is_symbol_char(p[start])) {
		/* Cursor is on the member name. */
	} else if (start > 0 && tag_is_symbol_char(p[start - 1])) {
		start--;
	} else {
		return;
	}
	while (start > 0 && tag_is_symbol_char(p[start - 1])) {
		start--;
	}
	if (start == 0 || p[start - 1] != '.') {
		return;
	}
	end = start - 1;
	start = end;
	while (start > 0 && tag_is_symbol_char(p[start - 1])) {
		start--;
	}
	if (start == end || (end - start) >= cap) {
		return;
	}
	memcpy(dst, p + start, (end - start));
	dst[end - start] = 0;
}

static bool tag_ascii_eq(const char *a, const char[..] b) {
	size_t i = 0;
	size_t n = 0;
	if (a == NULL) {
		return false;
	}
	n = len(b);
	if (strlen(a) != n) {
		return false;
	}
	for (i = 0; i < n; i++) {
		char ca = a[i];
		char cb = ptr(b)[i];
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

static bool tag_scope_matches(char *scope, char *hint) {
	size_t slen = 0;
	size_t hlen = 0;
	if (scope == NULL || hint == NULL || scope[0] == 0 || hint[0] == 0) {
		return false;
	}
	if (strcmp(scope, hint) == 0) {
		return true;
	}
	slen = strlen(scope);
	hlen = strlen(hint);
	return slen > hlen && scope[slen - hlen - 1] == '.' &&
		strcmp(scope + slen - hlen, hint) == 0;
}

static int tag_kind_score(char *kind, bool want_member) {
	if (kind == NULL || kind[0] == 0) {
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
	int score = 0;
	bool want_member = scope_hint != NULL && scope_hint[0] != 0;
	if (e == NULL) {
		return 0;
	}
	score = tag_kind_score(e.kind, want_member);
	if (len(cur_path) > 0 && e.path != NULL &&
		tags_paths_equal(cur_path, str_from_cstr(e.path))) {
		score += 100;
	}
	if (want_member && tag_scope_matches(e.scope, scope_hint)) {
		score += 80;
	}
	return score;
}

static int tag_nullable_cmp(char *a, char *b) {
	if (a == NULL) {
		return b == NULL ? 0 : -1;
	}
	if (b == NULL) {
		return 1;
	}
	return strcmp(a, b);
}

static int tag_rank_cmp(TagEntry *a, TagEntry *b, const char[..] cur_path,
	char *scope_hint) {
	int ascore = tag_score(a, cur_path, scope_hint);
	int bscore = tag_score(b, cur_path, scope_hint);
	int cmp = 0;
	char apath[TagPathMax] = {0};
	char bpath[TagPathMax] = {0};
	if (ascore != bscore) {
		return ascore > bscore ? -1 : 1;
	}
	cmp = tag_nullable_cmp(a.name, b.name);
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
	size_t i = 0;
	tag_scope_hint(scope_hint, sizeof(scope_hint));
	for (i = 1; i < len(list.items); i++) {
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

bool tags_collect_matches(char *name, TagList *out) {
	size_t i = 0;
	if (name == NULL || name[0] == 0 || out == NULL || !tags_ready()) {
		return false;
	}
	tags_list_free(out);
	i = tag_index_lower_bound(name);
	while (i < len(g_tags.by_name) && strcmp(g_tags.by_name[i].name, name) == 0) {
		if (!tag_list_append(out, g_tags.by_name[i])) {
			tags_list_free(out);
			return false;
		}
		i++;
	}
	tag_list_sort_ranked(out);
	return true;
}

bool tags_collect_completions(char *prefix, TagList *out) {
	Buffer *cur = buffer_current();
	char[..] cur_path = cur != NULL ? cur.file_name : str_empty();
	char scope_hint[TagNameMax] = {0};
	size_t plen = 0;
	size_t i = 0;
	if (prefix == NULL || prefix[0] == 0 || out == NULL || !tags_ready()) {
		return false;
	}
	tags_list_free(out);
	plen = strlen(prefix);
	tag_scope_hint(scope_hint, sizeof(scope_hint));
	i = tag_index_lower_bound(prefix);
	while (i < len(g_tags.by_name)) {
		TagEntry *best = NULL;
		char *name = g_tags.by_name[i].name;
		size_t j = i;
		if (name == NULL || strncmp(name, prefix, plen) != 0) {
			break;
		}
		while (j < len(g_tags.by_name) && strcmp(g_tags.by_name[j].name, name) == 0) {
			TagEntry *candidate = g_tags.by_name[j];
			if (best == NULL || tag_rank_cmp(candidate, best,
				cur_path, scope_hint) < 0) {
				best = candidate;
			}
			j++;
		}
		if (strcmp(name, prefix) != 0 && strchr(name, '.') == NULL &&
			!tag_list_append(out, best)) {
			tags_list_free(out);
			return false;
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
	int line_number = 0;
	size_t offset = 0;
	Line *line = NULL;
	if (win == NULL || win.buffer == NULL || out == NULL || cap < 2) {
		return 0;
	}
	line_number = win.cursor.line;
	offset = win.cursor.offset;
	line = win.buffer.line(line_number);
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
			reversed[used++] = ptr(view)[offset - 1];
		}
		offset--;
	}
	{
		size_t i = 0;
		for (i = 0; i < used; i++) {
			out[i] = reversed[used - i - 1];
		}
	}
	out[used] = 0;
	return (int)used;
}

static bool tag_find_call_hint(Window *win, char *name, size_t name_cap,
	char *scope, size_t scope_cap) {
	char context[513] = {0};
	int length = 0;
	int open = 0;
	int depth = 0;
	bool found = false;
	int i = 0;
	int end = 0;
	int start = 0;
	if (name == NULL || name_cap < 2 || scope == NULL || scope_cap < 2) {
		return false;
	}
	name[0] = 0;
	scope[0] = 0;
	length = tag_collect_call_context(win, context, sizeof(context));
	for (i = length; i > 0;) {
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
	end = open;
	while (end > 0 && isspace(context[end - 1])) {
		end--;
	}
	start = end;
	while (start > 0 &&
		tag_is_symbol_char(context[start - 1])) {
		start--;
	}
	if (start == end || (size_t)(end - start) >= name_cap) {
		return false;
	}
	memcpy(name, context + start, (end - start));
	name[end - start] = 0;
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
			scope[receiver_end - receiver_start] = 0;
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
	size_t i = 0;
	int best_score = -1;
	if (!tags_ready() || win == NULL ||
		!tag_find_call_hint(win, name, sizeof(name), scope, sizeof(scope))) {
		return;
	}
	i = tag_index_lower_bound(name);
	while (i < len(g_tags.by_name) && strcmp(g_tags.by_name[i].name, name) == 0) {
		TagEntry *entry = g_tags.by_name[i++];
		int score = 0;
		if (entry.signature == NULL || entry.signature[0] == 0) {
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
	char kind_default[4] = {0};
	char *kind = kind_default;
	char *typeref = NULL;
	if (dst == NULL || cap == 0) {
		return;
	}
	dst[0] = 0;
	snprintf(kind_default, sizeof(kind_default), "tag");
	if (entry == NULL) {
		return;
	}
	if (entry.kind != NULL && entry.kind[0] != 0) {
		kind = entry.kind;
	}
	typeref = entry.typeref;
	if (typeref != NULL && strncmp(typeref, "typename:", 9) == 0) {
		typeref = typeref + 9;
	}
	if (typeref != NULL && typeref[0] != 0 &&
		entry.scope != NULL && entry.scope[0] != 0) {
		snprintf(dst, cap, "(%s/%s@%s)", kind, typeref, entry.scope);
	} else if (typeref != NULL && typeref[0] != 0) {
		snprintf(dst, cap, "(%s/%s)", kind, typeref);
	} else if (entry.scope != NULL && entry.scope[0] != 0) {
		snprintf(dst, cap, "(%s@%s)", kind, entry.scope);
	} else {
		snprintf(dst, cap, "(%s)", kind);
	}
}

void tags_display_path(TagEntry *entry, char *dst, size_t cap) {
	char root[TagPathMax] = {0};
	char[..] dir = {0};
	size_t rlen = 0;
	if (dst == NULL || cap == 0) {
		return;
	}
	dst[0] = 0;
	if (entry == NULL || entry.path == NULL) {
		return;
	}
	dir = path_dir(str_from_cstr(g_tags.path));
	if (ptr(dir) != NULL && len(dir) > 0) {
		int n = snprintf(root, sizeof(root), "%s", dir);
		if (n < 0 || (size_t)n >= sizeof(root)) {
			root[0] = 0;
		}
	}
	rlen = strlen(root);
	if (rlen > 0 && strncmp(entry.path, root, rlen) == 0 &&
		entry.path[rlen] == '/') {
		snprintf(dst, cap, "%s", entry.path + rlen + 1);
	} else {
		snprintf(dst, cap, "%s", entry.path);
	}
}

static bool tag_qualified_duplicate(TagList *list, TagEntry *entry) {
	if (entry == NULL || entry.name == NULL || strchr(entry.name, '.') == NULL) {
		return false;
	}
	for (auto other : list.items) {
		if (other != NULL && other.name != NULL &&
			strchr(other.name, '.') == NULL && other.line == entry.line &&
			tag_nullable_cmp(other.kind, entry.kind) == 0) {
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
		if (entry.path != NULL &&
			tags_paths_equal(str_from_cstr(entry.path), file_path)) {
			if (!tag_list_append(out, entry)) {
				tags_list_free(out);
				return false;
			}
		}
	}
	for (i = 0; i < len(out.items);) {
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

static bool tag_pattern_needle(char *pattern, char *dst, size_t cap,
	bool *anchored) {
	size_t n = 0;
	size_t start = 1;
	size_t end = 0;
	size_t out = 0;
	char delim = 0;
	if (pattern == NULL || dst == NULL || cap < 2 || anchored == NULL) {
		return false;
	}
	n = strlen(pattern);
	if (n < 3) {
		return false;
	}
	delim = pattern[0];
	if ((delim != '/' && delim != '?') || pattern[n - 1] != delim) {
		return false;
	}
	end = n - 1;
	*anchored = start < end && pattern[start] == '^' &&
		pattern[end - 1] == '$';
	if (pattern[start] == '^') {
		start++;
	}
	if (end > start && pattern[end - 1] == '$') {
		end--;
	}
	while (start < end) {
		char ch = pattern[start++];
		if (ch == '\\' && start < end) {
			ch = pattern[start++];
		}
		if (out + 1 >= cap) {
			return false;
		}
		dst[out++] = ch;
	}
	dst[out] = 0;
	return out > 0;
}

static bool tag_line_matches(Line *line, char *needle, bool anchored) {
	char[..] view = {0};
	size_t nlen = 0;
	if (line == NULL || needle == NULL) {
		return false;
	}
	view = line.view();
	nlen = strlen(needle);
	if (anchored) {
		return len(view) == nlen && (nlen == 0 ||
			memcmp(ptr(view), needle, nlen) == 0);
	}
	if (nlen == 0 || len(view) < nlen || ptr(view) == NULL) {
		return false;
	}
	{
		size_t i = 0;
		for (i = 0; i + nlen <= len(view); i++) {
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
	int hint = 0;
	int count = 0;
	int distance = 0;
	if (entry == NULL || entry.line < 1 || buf == NULL ||
		!tag_pattern_needle(entry.pattern, needle, sizeof(needle), &anchored)) {
		return entry != NULL ? entry.line : 0;
	}
	count = buf.line_count_i();
	if (count < 1) {
		return entry.line;
	}
	hint = entry.line;
	if (hint > count) {
		hint = count;
	}
	if (tag_line_matches(buf.line(hint), needle, anchored)) {
		return hint;
	}
	for (distance = 1; distance < count; distance++) {
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
	int line = 0;
	if (e == NULL || e.path == NULL || e.line < 1) {
		return false;
	}
	if (w != NULL) {
		markring_push(w.cursor);
	}
	auto (buf, ok) = file_visit(e.path);
	if (!ok || buf == NULL) {
		return false;
	}
	line = tags_resolve_line(buf, e);
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
		buf.free();
		return NULL;
	}
	return buf;
}

static void tags_present_list(Buffer *buf) {
	Window *w = NULL;
	Location loc = {0};
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
	w = window_current();
	if (w == NULL) {
		return;
	}
	loc.line = 1;
	loc.offset = 0;
	w.set_cursor(loc);
	w.should_redraw = true;
	w.should_update_mode_line = true;
}

static bool tags_show_match_list(TagList *matches, char *title) {
	Buffer *buf = NULL;
	bool was_ro = false;
	char header[256] = {0};
	if (matches == NULL || len(matches.items) == 0) {
		return false;
	}
	buf = tags_ensure_list_buf();
	if (buf == NULL) {
		return false;
	}
	was_ro = buf.is_readonly;
	buf.is_readonly = false;
	buf.clear();
	buf.set_file_name(str_empty());
	buf.lang_mode = LModeMarkdown;
	buf.is_changed = false;
	if (title != NULL) {
		snprintf(header, sizeof(header), "# %s", title);
		if (buf.append_line(str_from_cstr(header)) == NULL) {
			buf.is_readonly = was_ro;
			return false;
		}
	}
	for (auto e : matches.items) {
		char linebuf[TagPathMax + TagNameMax + 64] = {0};
		Line *ln = NULL;
		char kind_default[4] = {0};
		char *kind = kind_default;
		snprintf(kind_default, sizeof(kind_default), "tag");
		if (e == NULL || e.path == NULL || e.name == NULL) {
			continue;
		}
		if (e.kind != NULL && e.kind[0] != 0) {
			kind = e.kind;
		}
		snprintf(linebuf, sizeof(linebuf), "%s  %s:%d  (%s)", e.name, e.path, e.line, kind);
		ln = buf.append_line(str_from_cstr(linebuf));
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

bool tags_goto_name(char *name) {
	TagList matches = {0};
	if (name == NULL || name[0] == 0) {
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
	Buffer *buf = NULL;
	Line *line = NULL;
	TagLocHead *h = NULL;
	char *p = NULL;
	TagEntry entry = {0};
	if (w == NULL || w.buffer == NULL) {
		return false;
	}
	buf = w.buffer;
	if (len(buf.name) == 0 || (!str_eq(buf.name, "*tags*") &&
		!str_eq(buf.name, "*describe*"))) {
		return false;
	}
	line = buf.line(w.cursor.line);
	if (line == NULL || line.metadata == NULL) {
		return false;
	}
	h = (TagLocHead *)line.metadata;
	p = tag_loc_path(h);
	if (p == NULL || p[0] == 0) {
		return false;
	}
	entry.path = p;
	entry.pattern = tag_loc_pattern(h);
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
		auto (jv, jok) = path_join(ranged(git, 0, sizeof(git)),
			str_from_cstr(dir), ".git");
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
			char[..] parent = path_dir(str_from_cstr(dir));
			int n = 0;
			if (len(parent) == 0 || ptr(parent) == NULL) {
				break;
			}
			n = snprintf(dir, sizeof(dir), "%s", parent);
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

static bool tags_make_temp(char *dst, size_t cap, char *target) {
	char[..] parent = str_empty();
	int n = 0;
	int fd = -1;
	if (dst == NULL || cap < 2 || target == NULL || target[0] == 0) {
		return false;
	}
	parent = path_dir(str_from_cstr(target));
	if (len(parent) == 0 || ptr(parent) == NULL) {
		return false;
	}
	n = snprintf(dst, cap, "%s/.jem-tags-XXXXXX", parent);
	if (n < 0 || (size_t)n >= cap) {
		return false;
	}
	fd = mkstemp(dst);
	if (fd < 0) {
		dst[0] = 0;
		return false;
	}
	close(fd);
	return true;
}

static bool tags_is_universal_ctags(char *path_name) {
	Cmd cmd = {0};
	const char[..] args[1] = {0};
	char text[512] = {0};
	size_t used = 0;
	if (path_name == NULL || path_name[0] == 0) {
		return false;
	}
	args[0] = "--version";
	cmd.init(str_from_cstr(path_name), args, 1);
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
	if (!tags_is_universal_ctags(ctags)) {
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
	if (path_is_abs(str_from_cstr(tag_name))) {
		if (strlen(tag_name) >= sizeof(target)) {
			display_set_echo("[tags output path is too long]");
			return false;
		}
		snprintf(target, sizeof(target), "%s", tag_name);
	} else {
		auto (tv, ok) = path_join(ranged(target, 0, sizeof(target)),
			str_from_cstr(root), str_from_cstr(tag_name));
		if (!ok || len(tv) == 0) {
			display_set_echo("[cannot determine tags output path]");
			return false;
		}
	}
	if (!tags_make_temp(temp, sizeof(temp), target)) {
		display_set_echo("[cannot create temporary tags file]");
		return false;
	}
	{
		char[..] base = path_base(str_from_cstr(target));
		int n = 0;
		if (len(base) == 0 || ptr(base) == NULL) {
			fs_remove(str_from_cstr(temp));
			display_set_echo("[invalid tags output path]");
			return false;
		}
		n = snprintf(exclude_name, sizeof(exclude_name), "%s", base);
		if (n < 0 || (size_t)n >= sizeof(exclude_name)) {
			fs_remove(str_from_cstr(temp));
			display_set_echo("[invalid tags output path]");
			return false;
		}
	}
	return job_start_tags(ctags, root, target, temp, exclude_name);
}

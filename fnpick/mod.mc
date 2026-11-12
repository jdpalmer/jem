// fnpick — fuzzy find/read/write-file picker (jem.go FilenamePrompt).
// Own package: display cannot import file (import cycle).
import "display";
import "file";
import "fs";
import "minibuffer";
import "path";
import "str";
import "term";
import "window";

#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

enum {
	FnMaxEntries = 512,
	FnMaxMatches = 256,
	FnNameCap = 256,
	FnPathCap = 4096
};

struct FnEntry {
	char name[FnNameCap];
	int64_t size; /* -1 for directories / n/a */
	bool is_dir;
};

static FnEntry g_fn_entries[FnMaxEntries];
static int g_fn_count;
static int g_fn_matches[FnMaxMatches];
static int g_fn_match_count;
static int g_fn_sel;
static bool g_fn_active;
static bool g_fn_programmatic;
static char g_fn_match_root[FnPathCap];
static char g_fn_dir_part[FnPathCap];
static char g_fn_last_query[FnPathCap];
static char g_fn_prompt[128];
static char g_fn_accepted[FnPathCap];

static int fn_page_size() {
	Window *mw = window_match();
	if (mw != NULL && mw.height > 1) {
		return mw.height;
	}
	return 10;
}

static void fn_clamp_sel() {
	if (g_fn_match_count == 0) {
		g_fn_sel = 0;
		return;
	}
	if (g_fn_sel < 0) {
		g_fn_sel = 0;
	}
	if (g_fn_sel >= g_fn_match_count) {
		g_fn_sel = g_fn_match_count - 1;
	}
}

static void fn_prompt_split(const char[..] path, char *dir, size_t dircap, char *base,
	size_t basecap) {
	size_t i = 0;
	size_t last = 0;
	bool have_sep = false;
	size_t n = len(path);

	if (dir != NULL && dircap > 0) {
		char[..] dv = ranged(dir, dircap);
		dv[0] = 0;
	}
	if (base != NULL && basecap > 0) {
		char[..] bv = ranged(base, basecap);
		bv[0] = 0;
	}
	if (n == 0) {
		return;
	}
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
		{
			char[..] dv = ranged(dir, dircap);
			dv[dlen] = 0;
		}
	}
	if (base != NULL && basecap > 0) {
		snprintf(base, basecap, "%s", path[last + 1 ..]);
	}
}

static bool fn_suffix_eq(const char *name, const char *suf) {
	if (name == NULL || suf == NULL) {
		return false;
	}
	size_t n = strlen(name);
	size_t s = strlen(suf);
	if (s == 0 || n < s) {
		return false;
	}
	return strcmp(name + (n - s), suf) == 0;
}

static bool fn_skip_name(const char *name, bool is_dir) {
	if (name == NULL || *name == 0) {
		return true;
	}
	if (*name == '.') {
		return true;
	}
	if (is_dir) {
		if (strcmp(name, ".git") == 0 || strcmp(name, "node_modules") == 0 ||
			strcmp(name, "__pycache__") == 0) {
			return true;
		}
	} else if (fn_suffix_eq(name, ".o") || fn_suffix_eq(name, ".exe") ||
		fn_suffix_eq(name, ".pyc")) {
		return true;
	}
	return false;
}

static void fn_sort_entries() {
	int start = 0;
	if (g_fn_count > 0 && strcmp(g_fn_entries[0].name, "../") == 0) {
		start = 1;
	}
	for (int i = start + 1; i < g_fn_count; i++) {
		FnEntry key = g_fn_entries[i];
		int j = i;
		while (j > start && strcmp(g_fn_entries[j - 1].name, key.name) > 0) {
			g_fn_entries[j] = g_fn_entries[j - 1];
			j--;
		}
		g_fn_entries[j] = key;
	}
}

static bool fn_at_fs_root(const char *abs_dir) {
	char parent[FnPathCap] = {0};
	if (abs_dir == NULL || *abs_dir == 0) {
		return true;
	}
	{
		const char[..] pd = path_dir(ranged(abs_dir, strlen(abs_dir)));
		size_t n = len(pd);
		if (n == 0) {
			return true;
		}
		if (n >= sizeof(parent)) {
			n = sizeof(parent) - 1;
		}
		memcpy(parent, ptr(pd), n);
		parent[n] = 0;
	}
	return strcmp(parent, abs_dir) == 0;
}

static void fn_collect(const char *open_dir) {
	char abs_dir[FnPathCap] = {0};

	g_fn_count = 0;
	memset(g_fn_entries, 0, sizeof(g_fn_entries));
	if (open_dir == NULL || *open_dir == 0) {
		open_dir = ".";
	}
	if (!file_resolve_path(abs_dir, sizeof(abs_dir), ranged(open_dir, strlen(open_dir)))) {
		snprintf(abs_dir, sizeof(abs_dir), "%s", open_dir);
	}
	if (!fn_at_fs_root(abs_dir) && g_fn_count < FnMaxEntries) {
		FnEntry *e = &g_fn_entries[g_fn_count];
		snprintf(e.name, sizeof(e.name), "../");
		e.size = -1;
		e.is_dir = true;
		g_fn_count++;
	}
	{
		auto (opened, dok) = dir_open(ranged(open_dir, strlen(open_dir)));
		Dir d = opened;
		if (!dok) {
			return;
		}
		for (;;) {
			auto (namev, nok) = d.next();
			char name[FnNameCap] = {0};
			size_t nl = 0;
			bool is_dir = false;
			int64_t size = -1;
			if (!nok) {
				d.close();
				return;
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
			if (strcmp(name, ".") == 0 || strcmp(name, "..") == 0) {
				continue;
			}
			pool p {
				auto (jv, jok) = path_join(&p,
					ranged(open_dir, strlen(open_dir)), ranged(name, strlen(name)));
				if (!jok) {
					continue;
				}
				auto (info, sok) = fs_stat(jv);
				if (!sok) {
					continue;
				}
				if (info.kind == KindDir) {
					is_dir = true;
					size = -1;
				} else if (info.kind == KindFile) {
					is_dir = false;
					size = info.size;
				} else {
					continue;
				}
			}
			if (fn_skip_name(name, is_dir)) {
				continue;
			}
			if (g_fn_count >= FnMaxEntries) {
				break;
			}
			{
				FnEntry *e = &g_fn_entries[g_fn_count];
				if (is_dir) {
					snprintf(e.name, sizeof(e.name), "%s/", name);
				} else {
					snprintf(e.name, sizeof(e.name), "%s", name);
				}
				e.size = size;
				e.is_dir = is_dir;
				g_fn_count++;
			}
		}
		d.close();
	}
	fn_sort_entries();
}

static void fn_refresh_list(const char *dir) {
	char open_dir[FnPathCap] = {0};
	if (dir == NULL) {
		dir = "";
	}
	if (!file_open_dir_from_prompt(open_dir, sizeof(open_dir), ranged(dir, strlen(dir)))) {
		snprintf(open_dir, sizeof(open_dir), ".");
	}
	if (strcmp(open_dir, g_fn_match_root) == 0) {
		return;
	}
	fn_collect(open_dir);
	snprintf(g_fn_match_root, sizeof(g_fn_match_root), "%s", open_dir);
}

static void fn_rebuild_matches(const char *pattern) {
	struct {
		int idx;
		int score;
	} scored[FnMaxEntries] = {0};
	int nscored = 0;
	g_fn_match_count = 0;
	if (pattern == NULL || *pattern == 0) {
		int n = g_fn_count;
		if (n > FnMaxMatches) {
			n = FnMaxMatches;
		}
		for (int i = 0; i < n; i++) {
			g_fn_matches[i] = i;
			g_fn_match_count++;
		}
		return;
	}
	for (int i = 0; i < g_fn_count; i++) {
		auto (ok, sc) = fuzzy_score(g_fn_entries[i].name, pattern);
		if (!ok) {
			continue;
		}
		if (nscored < FnMaxEntries) {
			scored[nscored].idx = i;
			scored[nscored].score = sc;
			nscored++;
		}
	}
	for (int i = 1; i < nscored; i++) {
		int idx = scored[i].idx;
		int sc = scored[i].score;
		int j = i;
		while (j > 0 && (scored[j - 1].score < sc ||
			(scored[j - 1].score == sc &&
				strcmp(g_fn_entries[scored[j - 1].idx].name, g_fn_entries[idx].name) > 0))) {
			scored[j] = scored[j - 1];
			j--;
		}
		scored[j].idx = idx;
		scored[j].score = sc;
	}
	for (int i = 0; i < nscored && i < FnMaxMatches; i++) {
		g_fn_matches[i] = scored[i].idx;
		g_fn_match_count++;
	}
}

static void fn_format_size(char *dst, size_t cap, int64_t n) {
	if (dst == NULL || cap == 0) {
		return;
	}
	snprintf(dst, cap, "");
	if (n < 0) {
		return;
	}
	if (n < 1024) {
		snprintf(dst, cap, "%dB", (int)n);
		return;
	}
	if (n < 1024 * 1024) {
		double v = (double)n / 1024.0;
		snprintf(dst, cap, "%.1fk", v);
		return;
	}
	if (n < (int64_t)1024 * 1024 * 1024) {
		double v = (double)n / (1024.0 * 1024.0);
		snprintf(dst, cap, "%.1fM", v);
		return;
	}
	{
		double v = (double)n / (1024.0 * 1024.0 * 1024.0);
		snprintf(dst, cap, "%.1fG", v);
	}
}

static void fn_refresh_match_window() {
	size_t out_len = 0;
	int name_w = 0;
	int size_w = 0;
	char prompt[160] = {0};
	int shown = g_fn_sel + 1;
	if (g_fn_match_count == 0) {
		shown = 0;
	}
	{
		char base_prompt[128] = {0};
		snprintf(base_prompt, sizeof(base_prompt), "%s", g_fn_prompt);
		size_t plen = strlen(base_prompt);
		if (plen >= 2 && base_prompt[plen - 2] == ':' && base_prompt[plen - 1] == ' ') {
			snprintf(prompt, sizeof(prompt), "%.*s [%d/%d]: ", (int)(plen - 2), base_prompt,
				shown, g_fn_match_count);
		} else {
			snprintf(prompt, sizeof(prompt), "%s", base_prompt);
		}
		minibuffer_set_prompt(ranged(prompt, strlen(prompt)));
	}
	if (g_fn_match_count == 0) {
		window_set_match_buffer_text(NULL, 0, 0);
		return;
	}
	for (int i = 0; i < g_fn_match_count; i++) {
		FnEntry *e = &g_fn_entries[g_fn_matches[i]];
		char sz[32] = {0};
		int nw = (int)strlen(e.name);
		fn_format_size(sz, sizeof(sz), e.size);
		int sw = (int)strlen(sz);
		if (nw > name_w) {
			name_w = nw;
		}
		if (sw > size_w) {
			size_w = sw;
		}
	}
	{
		size_t cap = (size_t)g_fn_match_count * 320 + 1;
		pool p {
			char[..] out = p.alloc(char, cap);
			if (ptr(out) == NULL) {
				return;
			}
			out[0] = 0;
			for (int i = 0; i < g_fn_match_count; i++) {
				FnEntry *e = &g_fn_entries[g_fn_matches[i]];
				char line[320] = {0};
				char sz[32] = {0};
				const char *arrow = (i == g_fn_sel) ? "→ " : "  ";
				int n = 0;
				fn_format_size(sz, sizeof(sz), e.size);
				if (sz[0] != 0) {
					n = snprintf(line, sizeof(line), "%s%-*s  %*s", arrow, name_w, e.name,
						size_w, sz);
				} else {
					n = snprintf(line, sizeof(line), "%s%s", arrow, e.name);
				}
				if (n < 0) {
					n = 0;
				}
				if (i > 0 && out_len + 1 < cap) {
					out[out_len++] = '\n';
					out[out_len] = 0;
				}
				if (out_len + (size_t)n + 1 > cap) {
					break;
				}
				memcpy(ptr(out) + out_len, line, n);
				out_len += n;
				out[out_len] = 0;
			}
			window_set_match_buffer_text(ptr(out), out_len, g_fn_sel);
		}
	}
}

static void fn_sync_matches() {
	char query[FnPathCap] = {0};
	char dir_part[FnPathCap] = {0};
	char base_part[FnPathCap] = {0};
	char *raw = minibuffer_text();

	if (raw != NULL) {
		snprintf(query, sizeof(query), "%s", raw);
	}
	bool query_changed = !g_fn_programmatic && strcmp(query, g_fn_last_query) != 0;
	snprintf(g_fn_last_query, sizeof(g_fn_last_query), "%s", query);

	if (strcmp(query, "~") == 0) {
		g_fn_dir_part[0] = 0;
		g_fn_match_count = 0;
		if (query_changed) {
			g_fn_sel = 0;
		}
		return;
	}
	fn_prompt_split(ranged(query, strlen(query)), dir_part, sizeof(dir_part), base_part,
		sizeof(base_part));
	snprintf(g_fn_dir_part, sizeof(g_fn_dir_part), "%s", dir_part);
	fn_refresh_list(dir_part);
	fn_rebuild_matches(base_part);
	if (query_changed) {
		g_fn_sel = 0;
	} else if (g_fn_sel >= g_fn_match_count) {
		g_fn_sel = 0;
	}
}

static void fn_set_prompt_text(const char *text) {
	g_fn_programmatic = true;
	if (text == NULL) {
		minibuffer_set_text(str_empty());
	} else {
		minibuffer_set_text(ranged(text, strlen(text)));
	}
	g_fn_programmatic = false;
}

static const char *fn_selected_name() {
	if (g_fn_match_count == 0 || g_fn_sel < 0 || g_fn_sel >= g_fn_match_count) {
		return NULL;
	}
	return g_fn_entries[g_fn_matches[g_fn_sel]].name;
}

static void fn_apply_selection_to(char *out, size_t cap) {
	const char *sel = fn_selected_name();
	char *raw = minibuffer_text();
	if (sel == NULL) {
		if (raw != NULL) {
			snprintf(out, cap, "%s", raw);
		} else {
			snprintf(out, cap, "");
		}
		return;
	}
	if (!file_apply_filename_selection(ranged(g_fn_dir_part, strlen(g_fn_dir_part)),
		ranged(sel, strlen(sel)), out, cap)) {
		snprintf(out, cap, "%s", raw != NULL ? raw : "");
	}
}

static FuzzyKeyResult fn_accept_enter() {
	char full[FnPathCap] = {0};
	const char *sel = fn_selected_name();

	fn_apply_selection_to(full, sizeof(full));
	if (sel != NULL && (strcmp(sel, "../") == 0 || fn_suffix_eq(sel, "/"))) {
		fn_set_prompt_text(full);
		fn_sync_matches();
		fn_refresh_match_window();
		return FuzzyContinue;
	}
	if (sel != NULL) {
		fn_set_prompt_text(full);
	}
	{
		char *raw = minibuffer_text();
		if (raw != NULL) {
			snprintf(g_fn_accepted, sizeof(g_fn_accepted), "%s", raw);
			minibuffer_history_add(raw);
		} else {
			g_fn_accepted[0] = 0;
		}
	}
	return FuzzyAccept;
}

void fuzzy_filename_open(const char[..] prompt, const char[..] initial) {
	memset(g_fn_entries, 0, sizeof(g_fn_entries));
	g_fn_count = 0;
	g_fn_match_count = 0;
	g_fn_sel = 0;
	g_fn_active = true;
	g_fn_programmatic = false;
	g_fn_match_root[0] = 0;
	g_fn_dir_part[0] = 0;
	g_fn_last_query[0] = 0;
	g_fn_accepted[0] = 0;
	g_fn_prompt[0] = 0;
	if (len(prompt) > 0 && ptr(prompt) != NULL) {
		size_t n = len(prompt);
		if (n >= sizeof(g_fn_prompt)) {
			n = sizeof(g_fn_prompt) - 1;
		}
		memcpy(g_fn_prompt, ptr(prompt), n);
		g_fn_prompt[n] = 0;
	} else {
		snprintf(g_fn_prompt, sizeof(g_fn_prompt), "Find file: ");
	}
	minibuffer_begin(ranged(g_fn_prompt, strlen(g_fn_prompt)), 2);
	if (len(initial) > 0 && ptr(initial) != NULL) {
		minibuffer_set_text(initial);
	}
	fn_sync_matches();
	fn_refresh_match_window();
}

void fuzzy_filename_close() {
	g_fn_active = false;
	window_discard_match_buffer();
	minibuffer_cancel();
}

bool fuzzy_filename_active() {
	return g_fn_active;
}

char *fuzzy_filename_text() {
	if (g_fn_accepted[0] != 0) {
		return g_fn_accepted;
	}
	return minibuffer_text();
}

FuzzyKeyResult fuzzy_filename_handle_key(uint32_t key) {
	bool changed = false;
	if (!g_fn_active) {
		return FuzzyAbort;
	}
	if (key == term_key_value(TermKeyEnter) || key == (uint32_t)'\r' || key == (uint32_t)'\n' ||
		key == (term_key_value(TermCTL) | (uint32_t)'M') ||
		key == (term_key_value(TermCTL) | (uint32_t)'J')) {
		return fn_accept_enter();
	}
	if (key == (term_key_value(TermCTL) | (uint32_t)'G') || key == 0x07 || key == 0x1B) {
		return FuzzyAbort;
	}
	if (key == term_key_value(TermKeyTab) || key == (uint32_t)'\t') {
		if (g_fn_match_count > 0 && g_fn_sel < g_fn_match_count) {
			char full[FnPathCap] = {0};
			fn_apply_selection_to(full, sizeof(full));
			fn_set_prompt_text(full);
			fn_sync_matches();
		} else {
			char tmp[FnPathCap] = {0};
			char *raw = minibuffer_text();
			if (raw != NULL) {
				snprintf(tmp, sizeof(tmp), "%s", raw);
			}
			if (file_complete_path(tmp, sizeof(tmp))) {
				fn_set_prompt_text(tmp);
				snprintf(g_fn_last_query, sizeof(g_fn_last_query), "%s", tmp);
				fn_sync_matches();
			} else {
				term_beep();
			}
		}
		fn_refresh_match_window();
		return FuzzyContinue;
	}
	if (key == term_key_value(TermKeyUp) || key == (term_key_value(TermCTL) | (uint32_t)'P') ||
		key == term_key_value(TermKeyDown) || key == (term_key_value(TermCTL) | (uint32_t)'N')) {
		if (g_fn_match_count == 0) {
			term_beep();
		} else {
			int delta = 1;
			if (key == term_key_value(TermKeyUp) || key == (term_key_value(TermCTL) | (uint32_t)'P')) {
				delta = -1;
			}
			g_fn_sel = (g_fn_sel + g_fn_match_count + delta) % g_fn_match_count;
		}
		fn_refresh_match_window();
		return FuzzyContinue;
	}
	if (key == (term_key_value(TermSHIFT) | term_key_value(TermKeyUp)) ||
		key == term_key_value(TermKeyPageUp) ||
		key == (term_key_value(TermSHIFT) | term_key_value(TermKeyDown)) ||
		key == term_key_value(TermKeyPageDown)) {
		if (g_fn_match_count == 0) {
			term_beep();
		} else {
			int delta = fn_page_size();
			int prev = g_fn_sel;
			if (key == (term_key_value(TermSHIFT) | term_key_value(TermKeyUp)) ||
				key == term_key_value(TermKeyPageUp)) {
				delta = -delta;
			}
			g_fn_sel += delta;
			fn_clamp_sel();
			if (g_fn_sel == prev) {
				term_beep();
			}
		}
		fn_refresh_match_window();
		return FuzzyContinue;
	}
	if (key == (term_key_value(TermCTL) | (uint32_t)'H') || key == 127) {
		if (minibuffer_backspace()) {
			changed = true;
		} else {
			term_beep();
		}
	} else if (key >= 32 && (key & term_key_value(TermKeyMask)) == 0 && key < 0x10000000) {
		if (minibuffer_insert(key)) {
			changed = true;
		} else {
			term_beep();
		}
	} else {
		term_beep();
		return FuzzyContinue;
	}
	if (changed) {
		fn_sync_matches();
		fn_refresh_match_window();
	}
	return FuzzyContinue;
}

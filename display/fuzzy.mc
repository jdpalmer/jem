// display/fuzzy.mc — fuzzy score + C-x b buffer picker (jem.go display minibuf_fuzzy / prompt_fuzzy).
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

enum {
	FuzzyMaxMatches = 256,
	FuzzyMaxBuffers = 64,
	FuzzyNameCap = 256
};

enum FuzzyKeyResult {
	FuzzyContinue = 0,
	FuzzyAccept = 1,
	FuzzyAbort = 2
};

struct FuzzyBufEntry {
	char name[FuzzyNameCap];
	int lines;
	bool modified;
	Buffer *buf;
};

static FuzzyBufEntry g_fb_entries[FuzzyMaxBuffers];
static int g_fb_count;
static int g_fb_matches[FuzzyMaxMatches];
static int g_fb_match_count;
static int g_fb_sel;
static bool g_fb_active;

static char fuzzy_lower(char c) {
	if (c >= 'A' && c <= 'Z') {
		return (char)(c + ('a' - 'A'));
	}
	return c;
}

static bool fuzzy_is_word_start(const char *name, int nlen, int i) {
	char c = 0;
	char p = 0;
	if (i <= 0) {
		return true;
	}
	if (i >= nlen) {
		return false;
	}
	c = name[i];
	if (c >= 'A' && c <= 'Z') {
		return true;
	}
	p = name[i - 1];
	return !((p >= 'a' && p <= 'z') || (p >= 'A' && p <= 'Z') || (p >= '0' && p <= '9'));
}

/* Returns (matched, score). Empty query matches everything with score 1. */
(bool, int) fuzzy_score(const char *name, const char *query) {
	int nlen = 0;
	int qlen = 0;
	int ni = 0;
	int prev = -1;
	int first = -1;
	int score = 0;
	int qi = 0;
	if (name == NULL) {
		return (false, 0);
	}
	nlen = (int)strlen(name);
	if (query == NULL || query[0] == 0) {
		return (true, 1);
	}
	qlen = (int)strlen(query);
	for (qi = 0; qi < qlen; qi++) {
		char qc = fuzzy_lower(query[qi]);
		int found = -1;
		while (ni < nlen) {
			int at = ni;
			bool consec = false;
			ni++;
			if (fuzzy_lower(name[at]) != qc) {
				continue;
			}
			consec = prev >= 0 && at == prev + 1;
			if (!consec && !fuzzy_is_word_start(name, nlen, at)) {
				continue;
			}
			found = at;
			if (fuzzy_is_word_start(name, nlen, at)) {
				score += 80;
			}
			if (consec) {
				score += 50;
			} else if (prev >= 0) {
				score -= (at - prev - 1) * 5;
			}
			break;
		}
		if (found < 0) {
			return (false, 0);
		}
		if (first < 0) {
			first = found;
		}
		prev = found;
	}
	score += qlen * 100;
	if (first == 0) {
		score += 20;
	}
	return (true, score);
}

static int cmp_access_desc(void *a, void *b) {
	Buffer *ba = *(Buffer **)a;
	Buffer *bb = *(Buffer **)b;
	int64_t ta = 0;
	int64_t tb = 0;
	if (ba == NULL || bb == NULL) {
		return 0;
	}
	ta = ba.access_time;
	tb = bb.access_time;
	if (ta != tb) {
		return ta > tb ? -1 : 1;
	}
	return str_cmp(ba.name, bb.name);
}

static void sort_buffers_by_access(Buffer **list, int n) {
	int i = 0;
	int j = 0;
	for (i = 1; i < n; i++) {
		Buffer *key = list[i];
		j = i;
		while (j > 0 && cmp_access_desc(&list[j - 1], &key) > 0) {
			list[j] = list[j - 1];
			j--;
		}
		list[j] = key;
	}
}

static void fuzzy_rebuild_matches(const char *query) {
	struct {
		int idx;
		int score;
	} scored[FuzzyMaxBuffers] = {0};
	int nscored = 0;
	int i = 0;
	int j = 0;
	g_fb_match_count = 0;
	for (i = 0; i < g_fb_count; i++) {
		auto (ok, sc) = fuzzy_score(g_fb_entries[i].name, query);
		if (!ok) {
			continue;
		}
		if (nscored < FuzzyMaxBuffers) {
			scored[nscored].idx = i;
			scored[nscored].score = sc;
			nscored++;
		}
	}
	/* insertion sort by score desc, then idx asc */
	for (i = 1; i < nscored; i++) {
		int idx = scored[i].idx;
		int sc = scored[i].score;
		j = i;
		while (j > 0 && (scored[j - 1].score < sc ||
			(scored[j - 1].score == sc && scored[j - 1].idx > idx))) {
			scored[j] = scored[j - 1];
			j--;
		}
		scored[j].idx = idx;
		scored[j].score = sc;
	}
	for (i = 0; i < nscored && i < FuzzyMaxMatches; i++) {
		g_fb_matches[i] = scored[i].idx;
		g_fb_match_count++;
	}
}

static void fuzzy_refresh_match_window() {
	char *out = NULL;
	size_t cap = 0;
	size_t len = 0;
	int i = 0;
	int count = g_fb_match_count;
	if (count > FuzzyMaxMatches) {
		count = FuzzyMaxMatches;
	}
	if (count == 0) {
		window_set_match_buffer_text(NULL, 0, 0);
		return;
	}
	cap = (size_t)count * 320 + 1;
	out = malloc(cap);
	if (out == NULL) {
		return;
	}
	out[0] = 0;
	for (i = 0; i < count; i++) {
		FuzzyBufEntry *e = &g_fb_entries[g_fb_matches[i]];
		char line[320] = {0};
		int n = 0;
		const char *arrow = (i == g_fb_sel) ? "→ " : "  ";
		n = snprintf(line, sizeof(line), "%s%s  %d", arrow, e.name, e.lines);
		if (n < 0) {
			n = 0;
		}
		if (i > 0) {
			if (len + 1 < cap) {
				out[len++] = '\n';
				out[len] = 0;
			}
		}
		if (len + (size_t)n + 1 > cap) {
			break;
		}
		memcpy(out + len, line, n);
		len += n;
		out[len] = 0;
	}
	window_set_match_buffer_text(out, len, g_fb_sel);
	free(out);
}

static void fuzzy_clamp_sel() {
	if (g_fb_match_count == 0) {
		g_fb_sel = 0;
		return;
	}
	if (g_fb_sel < 0) {
		g_fb_sel = 0;
	}
	if (g_fb_sel >= g_fb_match_count) {
		g_fb_sel = g_fb_match_count - 1;
	}
}

void fuzzy_buffer_open() {
	Buffer *list[FuzzyMaxBuffers] = {0};
	int n = 0;
	size_t i = 0;
	int initial_sel = 1;
	memset(g_fb_entries, 0, sizeof(g_fb_entries));
	g_fb_count = 0;
	g_fb_match_count = 0;
	g_fb_sel = 0;
	g_fb_active = true;
	for (auto b : buffer_all) {
		if (n >= FuzzyMaxBuffers) {
			break;
		}
		if (b == NULL || len(b.name) == 0) {
			continue;
		}
		if (str_eq(b.name, "*match*")) {
			continue;
		}
		list[n++] = b;
	}
	sort_buffers_by_access(list, n);
	for (i = 0; i < (size_t)n; i++) {
		Buffer *b = list[i];
		FuzzyBufEntry *e = &g_fb_entries[g_fb_count];
		size_t namelen = len(b.name);
		if (namelen >= (size_t)FuzzyNameCap) {
			namelen = (size_t)FuzzyNameCap - 1;
		}
		memcpy(e.name, ptr(b.name), namelen);
		e.name[namelen] = 0;
		e.lines = b.line_count_i();
		e.modified = b.is_changed;
		e.buf = b;
		g_fb_count++;
	}
	fuzzy_rebuild_matches("");
	if (g_fb_match_count == 0) {
		initial_sel = 0;
	} else if (initial_sel >= g_fb_match_count) {
		initial_sel = g_fb_match_count - 1;
	}
	g_fb_sel = initial_sel;
	minibuffer_begin("Switch buffer: ", 0);
	fuzzy_refresh_match_window();
}

void fuzzy_buffer_close() {
	g_fb_active = false;
	window_discard_match_buffer();
	minibuffer_cancel();
}

bool fuzzy_buffer_active() {
	return g_fb_active;
}

Buffer *fuzzy_buffer_selected() {
	if (!g_fb_active || g_fb_match_count == 0) {
		return NULL;
	}
	if (g_fb_sel < 0 || g_fb_sel >= g_fb_match_count) {
		return NULL;
	}
	return g_fb_entries[g_fb_matches[g_fb_sel]].buf;
}

static int match_page_size() {
	Window *mw = window_match();
	if (mw != NULL && mw.height > 1) {
		return mw.height;
	}
	return 10;
}

FuzzyKeyResult fuzzy_buffer_handle_key(uint32_t key) {
	bool changed = false;
	if (!g_fb_active) {
		return FuzzyAbort;
	}
	if (key == term_key_value(TermKeyEnter) || key == (uint32_t)'\r' || key == (uint32_t)'\n' ||
		key == (term_key_value(TermCTL) | (uint32_t)'M') || key == (term_key_value(TermCTL) | (uint32_t)'J')) {
		return FuzzyAccept;
	}
	if (key == (term_key_value(TermCTL) | (uint32_t)'G') || key == 0x07 || key == 0x1B) {
		return FuzzyAbort;
	}
	if (key == term_key_value(TermKeyUp) || key == (term_key_value(TermCTL) | (uint32_t)'P') ||
		key == term_key_value(TermKeyDown) || key == (term_key_value(TermCTL) | (uint32_t)'N')) {
		if (g_fb_match_count == 0) {
			term_beep();
		} else {
			int delta = 1;
			if (key == term_key_value(TermKeyUp) || key == (term_key_value(TermCTL) | (uint32_t)'P')) {
				delta = -1;
			}
			g_fb_sel = (g_fb_sel + g_fb_match_count + delta) % g_fb_match_count;
		}
		fuzzy_refresh_match_window();
		return FuzzyContinue;
	}
	if (key == (term_key_value(TermSHIFT) | term_key_value(TermKeyUp)) || key == term_key_value(TermKeyPageUp) ||
		key == (term_key_value(TermSHIFT) | term_key_value(TermKeyDown)) || key == term_key_value(TermKeyPageDown)) {
		if (g_fb_match_count == 0) {
			term_beep();
		} else {
			int delta = match_page_size();
			int prev = g_fb_sel;
			if (key == (term_key_value(TermSHIFT) | term_key_value(TermKeyUp)) || key == term_key_value(TermKeyPageUp)) {
				delta = -delta;
			}
			g_fb_sel += delta;
			fuzzy_clamp_sel();
			if (g_fb_sel == prev) {
				term_beep();
			}
		}
		fuzzy_refresh_match_window();
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
		fuzzy_rebuild_matches(minibuffer_text());
		g_fb_sel = 0;
		fuzzy_refresh_match_window();
	}
	return FuzzyContinue;
}

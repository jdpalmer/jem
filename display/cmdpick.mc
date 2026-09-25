// display/cmdpick.mc — fuzzy command palette (jem.go CmdCommandPalette).
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

enum {
	CmdPickMax = 128,
	CmdPickNameCap = 64,
	CmdPickBindCap = 32
};

struct CmdPickEntry {
	char name[CmdPickNameCap];
	char bind[CmdPickBindCap];
};

static CmdPickEntry g_cp[CmdPickMax];
static int g_cp_count;
static int g_cp_matches[CmdPickMax];
static int g_cp_match_count;
static int g_cp_sel;
static int g_cp_name_width;
static bool g_cp_active;
static char g_cp_last[CmdPickNameCap];

static int cmdpick_page_size() {
	Window *mw = window_match();
	if (mw != NULL && mw.height > 1) {
		return mw.height;
	}
	return 10;
}

static void cmdpick_clamp_sel() {
	if (g_cp_match_count == 0) {
		g_cp_sel = 0;
		return;
	}
	if (g_cp_sel < 0) {
		g_cp_sel = 0;
	}
	if (g_cp_sel >= g_cp_match_count) {
		g_cp_sel = g_cp_match_count - 1;
	}
}

static void cmdpick_rebuild(const char *query) {
	struct {
		int idx;
		int score;
	} scored[CmdPickMax] = {0};
	int nscored = 0;
	int i = 0;
	int j = 0;
	g_cp_match_count = 0;
	for (i = 0; i < g_cp_count; i++) {
		auto (ok, sc) = fuzzy_score(g_cp[i].name, query);
		if (!ok) {
			continue;
		}
		scored[nscored].idx = i;
		scored[nscored].score = sc;
		nscored++;
	}
	for (i = 1; i < nscored; i++) {
		int idx = scored[i].idx;
		int sc = scored[i].score;
		j = i;
		while (j > 0 && (scored[j - 1].score < sc ||
			(scored[j - 1].score == sc && strcmp(g_cp[scored[j - 1].idx].name, g_cp[idx].name) > 0))) {
			scored[j] = scored[j - 1];
			j--;
		}
		scored[j].idx = idx;
		scored[j].score = sc;
	}
	for (i = 0; i < nscored; i++) {
		g_cp_matches[i] = scored[i].idx;
		g_cp_match_count++;
	}
}

static void cmdpick_refresh() {
	char *out = NULL;
	size_t cap = 0;
	size_t len = 0;
	int i = 0;
	char prompt[64] = {0};
	int shown = g_cp_sel + 1;
	if (g_cp_match_count == 0) {
		shown = 0;
	}
	snprintf(prompt, sizeof(prompt), "→ [%d/%d] ", shown, g_cp_match_count);
	minibuffer_set_prompt(prompt);
	if (g_cp_match_count == 0) {
		window_set_match_buffer_text(NULL, 0, 0);
		return;
	}
	cap = (size_t)g_cp_match_count * 160 + 1;
	out = malloc(cap);
	if (out == NULL) {
		return;
	}
	out[0] = 0;
	for (i = 0; i < g_cp_match_count; i++) {
		CmdPickEntry *e = &g_cp[g_cp_matches[i]];
		char line[160] = {0};
		const char *arrow = (i == g_cp_sel) ? "→ " : "  ";
		int n = 0;
		n = snprintf(line, sizeof(line), "%s%-*s  %s", arrow, g_cp_name_width, e.name, e.bind);
		if (n < 0) {
			n = 0;
		}
		if (i > 0 && len + 1 < cap) {
			out[len++] = '\n';
			out[len] = 0;
		}
		if (len + (size_t)n + 1 > cap) {
			break;
		}
		memcpy(out + len, line, n);
		len += n;
		out[len] = 0;
	}
	window_set_match_buffer_text(out, len, g_cp_sel);
	free(out);
}

static bool name_eq_fold(const char *a, const char *b) {
	if (a == NULL || b == NULL) {
		return false;
	}
	while (*a != 0 && *b != 0) {
		char ca = *a;
		char cb = *b;
		if (ca >= 'A' && ca <= 'Z') {
			ca = (char)(ca - 'A' + 'a');
		}
		if (cb >= 'A' && cb <= 'Z') {
			cb = (char)(cb - 'A' + 'a');
		}
		if (ca != cb) {
			return false;
		}
		a++;
		b++;
	}
	return *a == 0 && *b == 0;
}

static int cmdpick_find_name(const char *name) {
	int i = 0;
	if (name == NULL || name[0] == 0) {
		return 0;
	}
	for (i = 0; i < g_cp_count; i++) {
		if (name_eq_fold(g_cp[i].name, name)) {
			return i;
		}
	}
	return 0;
}

void fuzzy_command_open(const char **names, const char **binds, int count,
	const char *prompt, const char *initial_name) {
	int i = 0;
	int j = 0;
	int initial = 0;
	memset(g_cp, 0, sizeof(g_cp));
	g_cp_count = 0;
	g_cp_match_count = 0;
	g_cp_sel = 0;
	g_cp_name_width = 0;
	g_cp_active = true;
	if (count > CmdPickMax) {
		count = CmdPickMax;
	}
	if (count < 0) {
		count = 0;
	}
	for (i = 0; i < count; i++) {
		size_t nlen = 0;
		size_t blen = 0;
		if (names == NULL || names[i] == NULL || names[i][0] == 0) {
			continue;
		}
		nlen = strlen(names[i]);
		if (nlen >= (size_t)CmdPickNameCap) {
			nlen = (size_t)CmdPickNameCap - 1;
		}
		memcpy(g_cp[g_cp_count].name, names[i], nlen);
		g_cp[g_cp_count].name[nlen] = 0;
		if (binds != NULL && binds[i] != NULL) {
			blen = strlen(binds[i]);
			if (blen >= (size_t)CmdPickBindCap) {
				blen = (size_t)CmdPickBindCap - 1;
			}
			memcpy(g_cp[g_cp_count].bind, binds[i], blen);
			g_cp[g_cp_count].bind[blen] = 0;
		}
		if ((int)nlen > g_cp_name_width) {
			g_cp_name_width = (int)nlen;
		}
		g_cp_count++;
	}
	for (i = 1; i < g_cp_count; i++) {
		CmdPickEntry key = g_cp[i];
		j = i;
		while (j > 0 && strcmp(g_cp[j - 1].name, key.name) > 0) {
			g_cp[j] = g_cp[j - 1];
			j--;
		}
		g_cp[j] = key;
	}
	cmdpick_rebuild("");
	initial = cmdpick_find_name(initial_name != NULL && initial_name[0] != 0 ? initial_name : g_cp_last);
	if (g_cp_match_count == 0) {
		g_cp_sel = 0;
	} else {
		g_cp_sel = 0;
		for (i = 0; i < g_cp_match_count; i++) {
			if (g_cp_matches[i] == initial) {
				g_cp_sel = i;
				break;
			}
		}
	}
	if (prompt != NULL && prompt[0] != 0) {
		minibuffer_begin(str_from_cstr(prompt), 0);
	} else {
		minibuffer_begin("→ ", 0);
	}
	cmdpick_refresh();
}

void fuzzy_command_close() {
	g_cp_active = false;
	window_discard_match_buffer();
	minibuffer_cancel();
}

bool fuzzy_command_active() {
	return g_cp_active;
}

char *fuzzy_command_selected() {
	if (!g_cp_active || g_cp_match_count == 0) {
		return NULL;
	}
	if (g_cp_sel < 0 || g_cp_sel >= g_cp_match_count) {
		return NULL;
	}
	return g_cp[g_cp_matches[g_cp_sel]].name;
}

void fuzzy_command_remember(const char *name) {
	size_t n = 0;
	g_cp_last[0] = 0;
	if (name == NULL) {
		return;
	}
	n = strlen(name);
	if (n >= (size_t)CmdPickNameCap) {
		n = (size_t)CmdPickNameCap - 1;
	}
	memcpy(g_cp_last, name, n);
	g_cp_last[n] = 0;
}

FuzzyKeyResult fuzzy_command_handle_key(uint32_t key) {
	bool changed = false;
	if (!g_cp_active) {
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
		if (g_cp_match_count == 0) {
			term_beep();
		} else {
			int delta = 1;
			if (key == term_key_value(TermKeyUp) || key == (term_key_value(TermCTL) | (uint32_t)'P')) {
				delta = -1;
			}
			g_cp_sel = (g_cp_sel + g_cp_match_count + delta) % g_cp_match_count;
		}
		cmdpick_refresh();
		return FuzzyContinue;
	}
	if (key == (term_key_value(TermSHIFT) | term_key_value(TermKeyUp)) || key == term_key_value(TermKeyPageUp) ||
		key == (term_key_value(TermSHIFT) | term_key_value(TermKeyDown)) || key == term_key_value(TermKeyPageDown)) {
		if (g_cp_match_count == 0) {
			term_beep();
		} else {
			int delta = cmdpick_page_size();
			int prev = g_cp_sel;
			if (key == (term_key_value(TermSHIFT) | term_key_value(TermKeyUp)) || key == term_key_value(TermKeyPageUp)) {
				delta = -delta;
			}
			g_cp_sel += delta;
			cmdpick_clamp_sel();
			if (g_cp_sel == prev) {
				term_beep();
			}
		}
		cmdpick_refresh();
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
		cmdpick_rebuild(minibuffer_text());
		g_cp_sel = 0;
		cmdpick_refresh();
	}
	return FuzzyContinue;
}

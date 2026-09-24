// Interactive fuzzy picker for tag navigation and completion.
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

enum TagPickerPurpose {
	TagPickerGoto = 1,
	TagPickerComplete = 2
};

struct TagPickScore {
	size_t index;
	int score;
};

static TagList g_tp_entries;
static TagPickScore g_tp_matches[TagPickerMax];
static int g_tp_match_count;
static int g_tp_selected;
static int g_tp_purpose;
static bool g_tp_with_path;
static bool g_tp_active;

static int tag_picker_page_size() {
	Window *mw = window_match();
	if (mw != NULL && mw.height > 1) {
		return mw.height;
	}
	return 10;
}

static void tag_picker_clamp() {
	if (g_tp_match_count == 0) {
		g_tp_selected = 0;
		return;
	}
	if (g_tp_selected < 0) {
		g_tp_selected = 0;
	}
	if (g_tp_selected >= g_tp_match_count) {
		g_tp_selected = g_tp_match_count - 1;
	}
}

static bool tag_picker_score_before(TagPickScore a, TagPickScore b) {
	if (a.score != b.score) {
		return a.score > b.score;
	}
	return a.index < b.index;
}

static void tag_picker_rebuild(const char *query) {
	size_t i = 0;
	g_tp_match_count = 0;
	for (i = 0; i < g_tp_entries.count; i++) {
		TagEntry *entry = g_tp_entries.items[i];
		TagPickScore candidate = {0};
		int pos = 0;
		auto (matched, score) = fuzzy_score(entry != NULL ? entry.name : NULL,
			query);
		if (!matched) {
			continue;
		}
		candidate.index = i;
		candidate.score = score;
		pos = g_tp_match_count;
		if (pos >= TagPickerMax) {
			pos = TagPickerMax - 1;
			if (!tag_picker_score_before(candidate, g_tp_matches[pos])) {
				continue;
			}
		} else {
			g_tp_match_count++;
		}
		while (pos > 0 &&
			tag_picker_score_before(candidate, g_tp_matches[pos - 1])) {
			if (pos < TagPickerMax) {
				g_tp_matches[pos] = g_tp_matches[pos - 1];
			}
			pos--;
		}
		g_tp_matches[pos] = candidate;
	}
	tag_picker_clamp();
}

static void tag_picker_refresh() {
	char *out = NULL;
	size_t cap = 0;
	size_t used = 0;
	int i = 0;
	if (g_tp_match_count == 0) {
		window_set_match_buffer_text(NULL, 0, 0);
		return;
	}
	cap = (size_t)g_tp_match_count * (TagPathMax + TagNameMax + 768);
	out = malloc(cap);
	if (out == NULL) {
		return;
	}
	out[0] = 0;
	for (i = 0; i < g_tp_match_count; i++) {
		TagEntry *entry = g_tp_entries.items[g_tp_matches[i].index];
		char annotation[TagKindMax + TagNameMax * 2] = {0};
		char shown_path[TagPathMax] = {0};
		char line[TagPathMax + TagNameMax * 3 + 1024] = {0};
		char arrow[5] = {0};
		char empty_signature[1] = {0};
		char *signature = empty_signature;
		int n = 0;
		snprintf(arrow, sizeof(arrow), "  ");
		if (i == g_tp_selected) {
			snprintf(arrow, sizeof(arrow), "→ ");
		}
		if (entry == NULL || entry.name == NULL) {
			continue;
		}
		tags_format_annotation(entry, annotation, sizeof(annotation));
		tags_display_path(entry, shown_path, sizeof(shown_path));
		if (entry.signature != NULL) {
			signature = entry.signature;
		}
		if (g_tp_with_path) {
			n = snprintf(line, sizeof(line), "%s%s  %s  %s:%d %s",
				arrow, entry.name, annotation, shown_path, entry.line,
				signature);
		} else {
			n = snprintf(line, sizeof(line), "%s%s  %s  :%d %s",
				arrow, entry.name, annotation, entry.line, signature);
		}
		if (n < 0) {
			n = 0;
		}
		if ((size_t)n >= sizeof(line)) {
			n = (int)sizeof(line) - 1;
		}
		if (i > 0 && used + 1 < cap) {
			out[used++] = '\n';
		}
		if (used + (size_t)n + 1 > cap) {
			break;
		}
		memcpy(out + used, line, n);
		used += n;
		out[used] = 0;
	}
	window_set_match_buffer_text(out, used, g_tp_selected);
	free(out);
}

bool tags_picker_open(TagList *entries, int purpose, const char[..] prompt,
	bool with_path) {
	if (entries == NULL || entries.count == 0) {
		return false;
	}
	if (g_tp_active) {
		tags_list_free(&g_tp_entries);
		window_discard_match_buffer();
		minibuffer_cancel();
	}
	g_tp_entries = *entries;
	entries.items = NULL;
	entries.count = 0;
	entries.capacity = 0;
	g_tp_purpose = purpose;
	g_tp_with_path = with_path;
	g_tp_selected = 0;
	g_tp_active = true;
	{
		char empty[1] = {0};
		tag_picker_rebuild(empty);
	}
	minibuffer_begin(prompt, 0);
	tag_picker_refresh();
	return true;
}

bool tags_picker_active() {
	return g_tp_active;
}

int tags_picker_purpose() {
	return g_tp_purpose;
}

TagEntry *tags_picker_selected() {
	if (!g_tp_active || g_tp_match_count == 0 ||
		g_tp_selected < 0 || g_tp_selected >= g_tp_match_count) {
		return NULL;
	}
	return g_tp_entries.items[g_tp_matches[g_tp_selected].index];
}

void tags_picker_close() {
	tags_list_free(&g_tp_entries);
	g_tp_match_count = 0;
	g_tp_selected = 0;
	g_tp_purpose = 0;
	g_tp_active = false;
	window_discard_match_buffer();
	minibuffer_cancel();
}

FuzzyKeyResult tags_picker_handle_key(uint32_t key) {
	bool changed = false;
	if (!g_tp_active) {
		return FuzzyAbort;
	}
	if (key == term_key_value(TermKeyEnter) || key == (uint32_t)'\r' ||
		key == (uint32_t)'\n' || key == (term_key_value(TermCTL) | (uint32_t)'M') ||
		key == (term_key_value(TermCTL) | (uint32_t)'J')) {
		return FuzzyAccept;
	}
	if (key == (term_key_value(TermCTL) | (uint32_t)'G') || key == 0x07 || key == 0x1B) {
		return FuzzyAbort;
	}
	if (key == term_key_value(TermKeyUp) || key == (term_key_value(TermCTL) | (uint32_t)'P') ||
		key == term_key_value(TermKeyDown) || key == (term_key_value(TermCTL) | (uint32_t)'N')) {
		if (g_tp_match_count == 0) {
			term_beep();
		} else {
			int delta = 1;
			if (key == term_key_value(TermKeyUp) || key == (term_key_value(TermCTL) | (uint32_t)'P')) {
				delta = -1;
			}
			g_tp_selected = (g_tp_selected + g_tp_match_count + delta) %
				g_tp_match_count;
		}
		tag_picker_refresh();
		return FuzzyContinue;
	}
	if (key == (term_key_value(TermSHIFT) | term_key_value(TermKeyUp)) || key == term_key_value(TermKeyPageUp) ||
		key == (term_key_value(TermSHIFT) | term_key_value(TermKeyDown)) || key == term_key_value(TermKeyPageDown)) {
		if (g_tp_match_count == 0) {
			term_beep();
		} else {
			int delta = tag_picker_page_size();
			int previous = g_tp_selected;
			if (key == (term_key_value(TermSHIFT) | term_key_value(TermKeyUp)) || key == term_key_value(TermKeyPageUp)) {
				delta = -delta;
			}
			g_tp_selected += delta;
			tag_picker_clamp();
			if (g_tp_selected == previous) {
				term_beep();
			}
		}
		tag_picker_refresh();
		return FuzzyContinue;
	}
	if (key == (term_key_value(TermCTL) | (uint32_t)'H') || key == 127) {
		if (minibuffer_backspace()) {
			changed = true;
		} else {
			term_beep();
		}
	} else if (key >= 32 && (key & term_key_value(TermKeyMask)) == 0 &&
		key < 0x10000000) {
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
		tag_picker_rebuild(minibuffer_text());
		tag_picker_refresh();
	}
	return FuzzyContinue;
}

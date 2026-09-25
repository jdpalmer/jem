// minibuffer — prompt editing state for Ask* flows.
import "str";

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

enum MinibufLimits {
	MinibufCap = 1024,
	MinibufHistorySlots = 16
};

struct MinibufferState {
	bool active;
	char prompt[128];
	char text[MinibufCap];
	size_t len;
	size_t cursor;
	int kind; /* 0=string 1=yesno 2=filename */
	int history_pos; /* -1 = editing current; 0 = newest history entry */
	bool have_saved_edit;
	char saved_edit[MinibufCap];
};

static MinibufferState g_mb;
static char g_history[MinibufHistorySlots][MinibufCap];
static int g_history_count;

// minibuffer_history_add appends text to the global prompt history.
void minibuffer_history_add(const char *text) {
	int i = 0;
	if (text == NULL || text[0] == 0) {
		return;
	}
	if (g_history_count > 0 &&
		strcmp(g_history[g_history_count - 1], text) == 0) {
		return;
	}
	if (g_history_count >= (int)MinibufHistorySlots) {
		for (i = 1; i < (int)MinibufHistorySlots; i++) {
			memcpy(g_history[i - 1], g_history[i], MinibufCap);
		}
		g_history_count = (int)MinibufHistorySlots - 1;
	}
	snprintf(g_history[g_history_count], MinibufCap, "%s", text);
	g_history_count++;
}

int minibuffer_history_count() {
	return g_history_count;
}

const char *minibuffer_history_at(int newest_index) {
	/* newest_index 0 = most recent */
	int idx = 0;
	if (newest_index < 0 || newest_index >= g_history_count) {
		return NULL;
	}
	idx = g_history_count - 1 - newest_index;
	return g_history[idx];
}

bool minibuffer_active() {
	return g_mb.active;
}

void minibuffer_set_prompt(const char[..] prompt) {
	if (!g_mb.active) {
		return;
	}
	g_mb.prompt[0] = 0;
	if (len(prompt) == 0 || ptr(prompt) == NULL) {
		return;
	}
	(void)snprintf(g_mb.prompt, sizeof(g_mb.prompt), "%s", prompt);
}

void minibuffer_begin(const char[..] prompt, int kind) {
	memset(&g_mb, 0, sizeof(g_mb));
	g_mb.active = true;
	g_mb.kind = kind;
	g_mb.history_pos = -1;
	if (len(prompt) != 0 && ptr(prompt) != NULL) {
		(void)snprintf(g_mb.prompt, sizeof(g_mb.prompt), "%s", prompt);
	}
}

void minibuffer_cancel() {
	g_mb.active = false;
	g_mb.len = 0;
	g_mb.cursor = 0;
	g_mb.text[0] = 0;
	g_mb.history_pos = -1;
	g_mb.have_saved_edit = false;
}

void minibuffer_set_text(const char[..] s) {
	size_t n = 0;
	if (!g_mb.active) {
		return;
	}
	if (len(s) == 0 || ptr(s) == NULL) {
		g_mb.len = 0;
		g_mb.cursor = 0;
		g_mb.text[0] = 0;
		return;
	}
	n = len(s);
	if (n >= (size_t)MinibufCap) {
		n = (size_t)MinibufCap - 1;
	}
	memcpy(g_mb.text, ptr(s), n);
	g_mb.text[n] = 0;
	g_mb.len = n;
	g_mb.cursor = n;
}

char *minibuffer_text() {
	return g_mb.text;
}

int minibuffer_kind() {
	return g_mb.kind;
}

// minibuffer_step_history: dir < 0 older, dir > 0 newer (jem.go StepHistory).
bool minibuffer_step_history(int dir) {
	if (!g_mb.active || g_history_count == 0) {
		return false;
	}
	if (dir < 0) {
		if (g_mb.history_pos + 1 >= g_history_count) {
			return false;
		}
		if (g_mb.history_pos < 0) {
			memcpy(g_mb.saved_edit, g_mb.text, MinibufCap);
			g_mb.have_saved_edit = true;
		}
		g_mb.history_pos++;
		{
			const char *entry = minibuffer_history_at(g_mb.history_pos);
			if (entry == NULL) {
				return false;
			}
			minibuffer_set_text(str_from_cstr(entry));
		}
		return true;
	}
	if (g_mb.history_pos < 0) {
		return false;
	}
	if (g_mb.history_pos == 0) {
		if (g_mb.have_saved_edit) {
			minibuffer_set_text(str_from_cstr(g_mb.saved_edit));
		} else {
			g_mb.len = 0;
			g_mb.cursor = 0;
			g_mb.text[0] = 0;
		}
		g_mb.history_pos = -1;
		g_mb.have_saved_edit = false;
		return true;
	}
	g_mb.history_pos--;
	{
		const char *entry = minibuffer_history_at(g_mb.history_pos);
		if (entry == NULL) {
			return false;
		}
		minibuffer_set_text(str_from_cstr(entry));
	}
	return true;
}

bool minibuffer_insert(uint32_t ch) {
	char utf[8] = {0};
	int n = 0;
	if (!g_mb.active) {
		return false;
	}
	if (ch < 32 || ch == 127) {
		return false;
	}
	if (ch < 128) {
		utf[0] = (char)ch;
		n = 1;
	} else if (ch < 0x800) {
		utf[0] = (char)(0xC0 | (ch >> 6));
		utf[1] = (char)(0x80 | (ch & 0x3F));
		n = 2;
	} else if (ch < 0x10000) {
		utf[0] = (char)(0xE0 | (ch >> 12));
		utf[1] = (char)(0x80 | ((ch >> 6) & 0x3F));
		utf[2] = (char)(0x80 | (ch & 0x3F));
		n = 3;
	} else {
		utf[0] = (char)(0xF0 | (ch >> 18));
		utf[1] = (char)(0x80 | ((ch >> 12) & 0x3F));
		utf[2] = (char)(0x80 | ((ch >> 6) & 0x3F));
		utf[3] = (char)(0x80 | (ch & 0x3F));
		n = 4;
	}
	if (g_mb.len + (size_t)n >= (size_t)MinibufCap) {
		return false;
	}
	memmove(g_mb.text + g_mb.cursor + (size_t)n, g_mb.text + g_mb.cursor, g_mb.len - g_mb.cursor);
	memcpy(g_mb.text + g_mb.cursor, utf, n);
	g_mb.cursor = g_mb.cursor + (size_t)n;
	g_mb.len = g_mb.len + (size_t)n;
	g_mb.text[g_mb.len] = 0;
	return true;
}

bool minibuffer_insert_paste(const char[..] text) {
	size_t insert_len = 0;
	size_t i = 0;
	if (!g_mb.active || len(text) == 0) {
		return false;
	}
	insert_len = len(text);
	if (g_mb.len + insert_len >= (size_t)MinibufCap) {
		insert_len = (size_t)MinibufCap - g_mb.len - 1;
	}
	if (insert_len == 0) {
		return false;
	}
	memmove(g_mb.text + g_mb.cursor + insert_len,
		g_mb.text + g_mb.cursor, g_mb.len - g_mb.cursor);
	for (i = 0; i < insert_len; i++) {
		char ch = ptr(text)[i];
		if (ch == '\r') {
			ch = '\n';
		}
		g_mb.text[g_mb.cursor + i] = ch;
	}
	g_mb.cursor += insert_len;
	g_mb.len += insert_len;
	g_mb.text[g_mb.len] = 0;
	return true;
}

bool minibuffer_backspace() {
	if (!g_mb.active || g_mb.cursor == 0) {
		return false;
	}
	/* delete one UTF-8 trail */
	size_t i = g_mb.cursor - 1;
	while (i != 0 && (g_mb.text[i] & 0xC0) == 0x80) {
		i--;
	}
	memmove(g_mb.text + i, g_mb.text + g_mb.cursor, g_mb.len - g_mb.cursor);
	g_mb.len = g_mb.len - (g_mb.cursor - i);
	g_mb.cursor = i;
	g_mb.text[g_mb.len] = 0;
	return true;
}

void minibuffer_format(char *out, size_t cap) {
	if (out == NULL || cap == 0) {
		return;
	}
	snprintf(out, cap, "%s%s", g_mb.prompt, g_mb.text);
}

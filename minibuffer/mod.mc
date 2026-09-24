// minibuffer — prompt editing state for Ask* flows.
#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

enum MinibufLimits {
	MinibufCap = 1024
};

struct MinibufferState {
	bool active;
	char prompt[128];
	char text[MinibufCap];
	size_t len;
	size_t cursor;
	int kind; /* 0=string 1=yesno 2=filename */
};

static MinibufferState g_mb;

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
	if (len(prompt) != 0 && ptr(prompt) != NULL) {
		(void)snprintf(g_mb.prompt, sizeof(g_mb.prompt), "%s", prompt);
	}
}

void minibuffer_cancel() {
	g_mb.active = false;
	g_mb.len = 0;
	g_mb.cursor = 0;
	g_mb.text[0] = 0;
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

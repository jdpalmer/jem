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

bool minibuffer_active(void) {
	return g_mb.active;
}

void minibuffer_set_prompt(char[..] prompt) {
	size_t n = 0;
	if (!g_mb.active) {
		return;
	}
	g_mb.prompt[0] = 0;
	if (len(prompt) == 0 || ptr(prompt) == NULL) {
		return;
	}
	n = len(prompt);
	if (n >= sizeof(g_mb.prompt)) {
		n = sizeof(g_mb.prompt) - 1;
	}
	memcpy(g_mb.prompt, ptr(prompt), n);
	g_mb.prompt[n] = 0;
}

void minibuffer_begin(char[..] prompt, int kind) {
	size_t n = 0;
	memset(&g_mb, 0, sizeof(g_mb));
	g_mb.active = true;
	g_mb.kind = kind;
	if (len(prompt) != 0 && ptr(prompt) != NULL) {
		n = len(prompt);
		if (n >= sizeof(g_mb.prompt)) {
			n = sizeof(g_mb.prompt) - 1;
		}
		memcpy(g_mb.prompt, ptr(prompt), n);
		g_mb.prompt[n] = 0;
	}
}

void minibuffer_cancel(void) {
	g_mb.active = false;
	g_mb.len = 0;
	g_mb.cursor = 0;
	g_mb.text[0] = 0;
}

void minibuffer_set_text(char *s) {
	size_t n = 0;
	if (!g_mb.active) {
		return;
	}
	if (s == NULL) {
		g_mb.len = 0;
		g_mb.cursor = 0;
		g_mb.text[0] = 0;
		return;
	}
	n = strlen(s);
	if (n >= (size_t)MinibufCap) {
		n = (size_t)MinibufCap - 1;
	}
	memcpy(g_mb.text, s, n);
	g_mb.text[n] = 0;
	g_mb.len = n;
	g_mb.cursor = n;
}

char *minibuffer_text(void) {
	return g_mb.text;
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
	memcpy(g_mb.text + g_mb.cursor, utf, (size_t)n);
	g_mb.cursor = g_mb.cursor + (size_t)n;
	g_mb.len = g_mb.len + (size_t)n;
	g_mb.text[g_mb.len] = 0;
	return true;
}

bool minibuffer_backspace(void) {
	if (!g_mb.active || g_mb.cursor == 0) {
		return false;
	}
	/* delete one UTF-8 trail */
	size_t i = g_mb.cursor - 1;
	while (i != 0 && ((unsigned char)g_mb.text[i] & 0xC0) == 0x80) {
		i--;
	}
	memmove(g_mb.text + i, g_mb.text + g_mb.cursor, g_mb.len - g_mb.cursor);
	g_mb.len = g_mb.len - (g_mb.cursor - i);
	g_mb.cursor = i;
	g_mb.text[g_mb.len] = 0;
	return true;
}

/* Forward-declared Screen/DispCell painting via opaque row write helper. */
struct DispCellFwd {
	uint32_t ch;
	uint16_t style_bits;
};

/* minibuffer_paint_row writes prompt+text into a simple char row buffer. */
void minibuffer_format(char *out, size_t cap) {
	if (out == NULL || cap == 0) {
		return;
	}
	snprintf(out, cap, "%s%s", g_mb.prompt, g_mb.text);
}

/* Used by display: paint into last row via callbacks stored as extern from display.
   We provide a simple API that display calls with its Screen — but Screen is in
   display. To avoid cycles, display paints using minibuffer_format. */
void minibuffer_paint_raw(void *screen_cells, int cols, int row, int cell_stride) {
	char line[1024] = {0};
	int i = 0;
	size_t n = 0;
	unsigned char *base = (unsigned char *)screen_cells;
	minibuffer_format(line, sizeof(line));
	n = strlen(line);
	for (i = 0; i < cols; i++) {
		/* DispCell is {uint32_t ch; TextStyle style} — TextStyle is {uint16_t bits} */
		uint32_t *chp = (uint32_t *)(base + (size_t)(row * cols + i) * (size_t)cell_stride);
		uint16_t *bits = (uint16_t *)((char *)chp + sizeof(uint32_t));
		*chp = (size_t)i < n ? (unsigned char)line[i] : ' ';
		*bits = 0; /* default; display may restyle */
		(void)bits;
	}
}

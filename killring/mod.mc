// killring — kill/yank ring + OSC 52 clipboard hint.
import "term";

#include <stddef.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>

enum KillRingLimits {
	KillRingMax = 32
};

struct KillEntry {
	char *data;
	size_t len;
};

static KillEntry g_ring[KillRingMax];
static int g_count;
static int g_yank_index;

static void kill_entry_set(KillEntry *e, char *data, size_t n) {
	free(e.data);
	e.data = NULL;
	e.len = 0;
	if (n == 0 || data == NULL) {
		e.data = malloc(1);
		if (e.data != NULL) {
			e.data[0] = 0;
		}
		return;
	}
	e.data = malloc(n + 1);
	if (e.data == NULL) {
		return;
	}
	memcpy(e.data, data, n);
	e.data[n] = 0;
	e.len = n;
}

// killring_push stores text at the front of the ring and copies to OSC 52.
void killring_push(char *data, size_t n) {
	int i = 0;
	if (g_count == KillRingMax) {
		free(g_ring[KillRingMax - 1].data);
		g_count--;
	}
	for (i = g_count; i > 0; i--) {
		g_ring[i] = g_ring[i - 1];
	}
	g_ring[0].data = NULL;
	g_ring[0].len = 0;
	kill_entry_set(&g_ring[0], data, n);
	g_count++;
	g_yank_index = 0;
	if (n != 0 && data != NULL) {
		term_osc52_copy(data, n);
	}
}

// killring_append concatenates onto the most recent kill.
void killring_append(char *data, size_t n) {
	KillEntry *e = {0};
	char *p = {0};
	if (g_count == 0) {
		killring_push(data, n);
		return;
	}
	e = &g_ring[0];
	p = realloc(e.data, e.len + n + 1);
	if (p == NULL) {
		return;
	}
	e.data = p;
	if (n != 0 && data != NULL) {
		memcpy(e.data + e.len, data, n);
	}
	e.len = e.len + n;
	e.data[e.len] = 0;
	term_osc52_copy(e.data, e.len);
}

// killring_yank returns (ptr, len) of current yank entry (still owned by ring).
(char *, size_t) killring_yank(void) {
	if (g_count == 0) {
		return (NULL, 0);
	}
	g_yank_index = 0;
	return (g_ring[0].data, g_ring[0].len);
}

// killring_yank_pop rotates to the next older kill.
(char *, size_t) killring_yank_pop(void) {
	if (g_count == 0) {
		return (NULL, 0);
	}
	g_yank_index = (g_yank_index + 1) % g_count;
	return (g_ring[g_yank_index].data, g_ring[g_yank_index].len);
}

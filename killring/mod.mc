// killring — editor kill/yank policy over the system clipboard package.
import "clipboard";
import "str";

#include <stddef.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>

enum KillRingLimits {
	KillRingMax = 32
};

struct KillEntry {
	char[..] data; /* owned heap */
};

static KillEntry g_ring[KillRingMax];
static int g_count;
static int g_yank_index;
static int g_kill_sequence;

static void killring_store_new(const char[..] data) {
	int i = 0;
	if (g_count == (int)KillRingMax) {
		str_free(&g_ring[(int)KillRingMax - 1].data);
		g_count--;
	}
	for (i = g_count; i > 0; i--) {
		g_ring[i] = g_ring[i - 1];
	}
	g_ring[0].data = str_empty();
	(void)str_set(&g_ring[0].data, data);
	g_count++;
	g_yank_index = 0;
}

// killring_push stores text at the front and writes the system clipboard.
void killring_push(const char[..] data) {
	killring_store_new(data);
	if (len(data) != 0) {
		(void)clipboard_write(data);
	}
}

// killring_append concatenates onto the most recent kill.
void killring_append(const char[..] data) {
	if (g_count == 0) {
		killring_push(data);
		return;
	}
	if (!str_append(&g_ring[0].data, data)) {
		return;
	}
	(void)clipboard_write(g_ring[0].data);
}

// killring_kill starts or extends a consecutive kill sequence.
void killring_kill(const char[..] data) {
	if (len(data) == 0) {
		return;
	}
	if (g_kill_sequence != 0 && g_count > 0) {
		killring_append(data);
	} else {
		killring_push(data);
	}
	g_kill_sequence = 2;
}

bool killring_read_clipboard() {
	auto (data, n, ok) = clipboard_read();
	if (!ok || data == NULL || n == 0) {
		free(data);
		return false;
	}
	killring_store_new(ranged(data, n));
	free(data);
	return true;
}

bool killring_in_sequence() {
	return g_kill_sequence != 0;
}

void killring_tick() {
	if (g_kill_sequence > 0) {
		g_kill_sequence--;
	}
}

// killring_yank returns a borrow of the current yank entry (owned by ring).
char[..] killring_yank() {
	if (g_count == 0) {
		return str_empty();
	}
	g_yank_index = 0;
	return g_ring[0].data;
}

// killring_yank_pop rotates to the next older kill.
char[..] killring_yank_pop() {
	if (g_count == 0) {
		return str_empty();
	}
	g_yank_index = (g_yank_index + 1) % g_count;
	return g_ring[g_yank_index].data;
}

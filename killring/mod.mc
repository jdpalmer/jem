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
/* Ring ahead of the system clipboard; flushed when the kill sequence ends. */
static bool g_clipboard_dirty;

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

static void killring_flush_clipboard() {
	if (!g_clipboard_dirty || g_count == 0) {
		return;
	}
	if (len(g_ring[0].data) == 0) {
		g_clipboard_dirty = false;
		return;
	}
	/* Keep dirty on failure so yank still prefers the ring over a stale pasteboard. */
	if (clipboard_write(g_ring[0].data)) {
		g_clipboard_dirty = false;
	}
}

// killring_push stores text at the front; clipboard sync is deferred.
void killring_push(const char[..] data) {
	killring_store_new(data);
	g_clipboard_dirty = true;
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
	g_clipboard_dirty = true;
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

/* Pull system clipboard into the ring when it differs from the front entry.
 * Does not touch the ring when the clipboard is unavailable or empty. */
bool killring_read_clipboard() {
	auto (data, n, ok) = clipboard_read();
	if (!ok || data == NULL || n == 0) {
		free(data);
		return false;
	}
	if (g_count > 0 && str_eq(g_ring[0].data, ranged(data, n))) {
		free(data);
		return true;
	}
	killring_store_new(ranged(data, n));
	g_clipboard_dirty = false;
	free(data);
	return true;
}

bool killring_in_sequence() {
	return g_kill_sequence != 0;
}

bool killring_clipboard_dirty() {
	return g_clipboard_dirty;
}

void killring_tick() {
	if (g_kill_sequence > 0) {
		g_kill_sequence--;
		/* ponytail: one pbcopy after the whole kill chain; external paste of
		 * the last kill waits until the next non-kill command (seq 1→0). */
		if (g_kill_sequence == 0) {
			killring_flush_clipboard();
		}
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

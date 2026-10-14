// markring — per-buffer mark stack for set-mark / pop-mark.
import "buffer";

#include <stddef.h>
#include <stdbool.h>

void markring_push(Location loc) {
	Buffer *buf = buffer_current();
	int i = 0;
	if (buf == NULL || loc.line == 0) {
		return;
	}
	if (buf.mark_ring_count == BufferMarkRingMax) {
		buf.mark_ring_count--;
	}
	for (i = buf.mark_ring_count; i > 0; i--) {
		buf.mark_ring[i] = buf.mark_ring[i - 1];
	}
	buf.mark_ring[0] = loc;
	buf.mark_ring_count++;
}

(Location, bool) markring_pop() {
	Location loc = {0};
	Buffer *buf = buffer_current();
	int i = 0;
	if (buf == NULL || buf.mark_ring_count == 0) {
		return (loc, false);
	}
	loc = buf.mark_ring[0];
	for (i = 0; i + 1 < buf.mark_ring_count; i++) {
		buf.mark_ring[i] = buf.mark_ring[i + 1];
	}
	buf.mark_ring_count--;
	return (loc, true);
}

void markring_clear() {
	Buffer *buf = buffer_current();
	if (buf != NULL) {
		buf.mark_ring_count = 0;
	}
}

int markring_count() {
	Buffer *buf = buffer_current();
	if (buf == NULL) {
		return 0;
	}
	return buf.mark_ring_count;
}

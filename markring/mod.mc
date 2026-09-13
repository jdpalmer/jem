// markring — mark stack for set-mark / pop-mark.
import "buffer";

#include <stddef.h>
#include <stdbool.h>

enum MarkRingLimits {
	MarkRingMax = 16
};

struct MarkRing {
	Location locs[MarkRingMax];
	int count;
};

static MarkRing g_marks;

void markring_push(Location loc) {
	int i = 0;
	if (g_marks.count == MarkRingMax) {
		g_marks.count--;
	}
	for (i = g_marks.count; i > 0; i--) {
		g_marks.locs[i] = g_marks.locs[i - 1];
	}
	g_marks.locs[0] = loc;
	g_marks.count++;
}

(Location, bool) markring_pop(void) {
	Location loc = {0};
	int i = 0;
	if (g_marks.count == 0) {
		return (loc, false);
	}
	loc = g_marks.locs[0];
	for (i = 0; i + 1 < g_marks.count; i++) {
		g_marks.locs[i] = g_marks.locs[i + 1];
	}
	g_marks.count--;
	return (loc, true);
}

void markring_clear(void) {
	g_marks.count = 0;
}

int markring_count(void) {
	return g_marks.count;
}

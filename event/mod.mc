// event — inbound editor event bus (ring queue).
//
// Tagged events replace Go interfaces. Producers call event_enqueue; the
// runtime drains via event_try_dequeue on the main loop tick.
#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

enum EventKind {
	EventNone = 0,
	EventKey = 1,
	EventResize = 2,
	EventQuit = 3,
	EventPaste = 4,
	EventMouse = 5,
	EventCommand = 6
};

struct Event {
	EventKind kind;
	uint32_t key_code;
	bool quit_force;
	int mouse_col;
	int mouse_row;
	char *paste_data;
	size_t paste_len;
	char command_name[64];
	bool command_f;
	int command_n;
};

static Event g_queue[256];
static size_t g_head;
static size_t g_tail;
static size_t g_count;

static void event_clear(Event *e) {
	if (e == NULL) {
		return;
	}
	if (e.paste_data != NULL) {
		free(e.paste_data);
		e.paste_data = NULL;
	}
	memset(e, 0, sizeof(*e));
}

// event_enqueue copies e onto the ring (owns paste_data if set).
bool event_enqueue(Event e) {
	Event *slot = {0};
	if (g_count >= (size_t)256) {
		if (e.paste_data != NULL) {
			free(e.paste_data);
		}
		return false;
	}
	slot = &g_queue[g_tail];
	event_clear(slot);
	*slot = e;
	g_tail = (g_tail + 1) % 256;
	g_count++;
	return true;
}

/* If the ring tail is already this key, keep a single pending event (mouse drag). */
bool event_enqueue_key_coalesced(uint32_t code) {
	if (g_count > 0) {
		size_t prev = (g_tail + 255) % 256;
		Event *slot = &g_queue[prev];
		if (slot.kind == EventKey && slot.key_code == code) {
			return true;
		}
	}
	return event_enqueue(event_key(code));
}

// event_try_dequeue pops the next event into *out. Returns false if empty.
bool event_try_dequeue(Event *out) {
	Event *slot = {0};
	if (g_count == 0 || out == NULL) {
		return false;
	}
	slot = &g_queue[g_head];
	*out = *slot;
	memset(slot, 0, sizeof(*slot));
	g_head = (g_head + 1) % 256;
	g_count--;
	return true;
}

// event_queue_len returns pending count.
size_t event_queue_len() {
	return g_count;
}

// event_key builds a KeyEvent.
Event event_key(uint32_t code) {
	Event e = {EventNone};
	e.kind = EventKey;
	e.key_code = code;
	return e;
}

// event_resize builds a ResizeEvent.
Event event_resize() {
	Event e = {EventNone};
	e.kind = EventResize;
	return e;
}

// event_quit builds a QuitEvent.
Event event_quit(bool force) {
	Event e = {EventNone};
	e.kind = EventQuit;
	e.quit_force = force;
	return e;
}

// event_mouse builds a MouseEvent.
Event event_mouse(int col, int row) {
	Event e = {EventNone};
	e.kind = EventMouse;
	e.mouse_col = col;
	e.mouse_row = row;
	return e;
}

// event_paste takes ownership of data (malloc'd) of length n.
Event event_paste(char *data, size_t n) {
	Event e = {EventNone};
	e.kind = EventPaste;
	e.paste_data = data;
	e.paste_len = n;
	return e;
}

// event_free releases owned paste payload on a dequeued event after handling.
void event_free(Event *e) {
	if (e == NULL) {
		return;
	}
	if (e.paste_data != NULL) {
		free(e.paste_data);
		e.paste_data = NULL;
		e.paste_len = 0;
	}
}

import "search";
import "buffer";
import "window";

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

int main() {
	Buffer *buf = NULL;
	Buffer *undone = NULL;
	Window *win = NULL;
	Location cursor = {0};
	char original[] = "a a";
	char needle[] = "a";
	char replacement[] = "aa";

	buffer_list_init();
	buf = buffer_current();
	check(buf != NULL);
	check(buf.load_bytes(original, strlen(original), EModeLF));
	win = window_create();
	check(win != NULL);
	window_select(win);

	search_begin_replace();
	check(search_set_needle(needle));
	check(search_set_replacement(replacement));
	check(search_update());
	check(search_replace_one());
	check(!search_replace_one());
	auto (saved, saved_len, saved_ok) = buf.save_bytes();
	check(saved_ok);
	check(saved_len == 5);
	check(memcmp(saved, "aa aa", 5) == 0);
	free(saved);

	check(buffer_undo.undo_once(&cursor, &undone));
	check(buffer_undo.undo_once(&cursor, &undone));
	auto (restored, restored_len, restored_ok) = buf.save_bytes();
	check(restored_ok);
	check(restored_len == strlen(original));
	check(memcmp(restored, original, restored_len) == 0);
	free(restored);
	search_end();
	return 0;
}

// runtime/search_cmds.mc — non-incremental search + toggle-search-scope.
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

// cmd_search_forward prompts then searches forward (jem.go CmdSearchForward).
bool cmd_search_forward(bool f, int n) {
	const char *prior = search_last_pattern();
	(void)f;
	(void)n;
	push_listener(ListenerAskString);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = AskSearchForward;
		}
	}
	if (search_scope_is_all()) {
		minibuffer_begin("Search (all buffers): ", 0);
	} else {
		minibuffer_begin("Search: ", 0);
	}
	if (prior != NULL && prior[0] != 0) {
		minibuffer_set_text(str_from_cstr(prior));
	}
	return true;
}

// cmd_search_backward prompts then searches backward.
bool cmd_search_backward(bool f, int n) {
	const char *prior = search_last_pattern();
	(void)f;
	(void)n;
	push_listener(ListenerAskString);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = AskSearchBackward;
		}
	}
	if (search_scope_is_all()) {
		minibuffer_begin("Reverse search (all buffers): ", 0);
	} else {
		minibuffer_begin("Reverse search: ", 0);
	}
	if (prior != NULL && prior[0] != 0) {
		minibuffer_set_text(str_from_cstr(prior));
	}
	return true;
}

bool cmd_run_prompted_search(bool forward, const char *pattern) {
	if (pattern != NULL && pattern[0] != 0) {
		search_set_last_pattern(pattern);
	} else if (search_last_pattern()[0] == 0) {
		return true;
	}
	if (!search_plain_find(forward)) {
		display_set_echo("[not found]");
		return true;
	}
	display_set_echo("");
	return true;
}

// cmd_toggle_search_scope switches between current buffer and all buffers.
bool cmd_toggle_search_scope(bool f, int n) {
	(void)f;
	(void)n;
	if (search_toggle_scope()) {
		display_set_echo("[search scope: all buffers]");
	} else {
		display_set_echo("[search scope: current buffer]");
	}
	{
		size_t i = 0;
		for (i = 0; i < window_active.count; i++) {
			Window *win = window_active.windows[i];
			if (win != NULL) {
				win.should_update_mode_line = true;
			}
		}
	}
	return true;
}

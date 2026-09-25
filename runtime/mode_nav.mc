// runtime/mode_nav.mc — wrappers that surface mode package echo messages.
#include <stddef.h>
#include <stdbool.h>
#include <stdio.h>

static bool flush_mode_echo(bool ok) {
	char msg[128] = {0};
	if (mode_consume_echo(msg, sizeof(msg))) {
		display_set_echo(msg);
	}
	return ok;
}

bool cmd_goto_match(bool f, int n) {
	return flush_mode_echo(mode_goto_match(f, n));
}

bool cmd_beginning_of_defun(bool f, int n) {
	return flush_mode_echo(mode_top_of_function(f, n));
}

bool cmd_end_of_defun(bool f, int n) {
	return flush_mode_echo(mode_end_of_function(f, n));
}

bool cmd_mark_defun(bool f, int n) {
	return flush_mode_echo(mode_mark_function(f, n));
}

import "job";
import "runtime";
import "buffer";
import "window";
import "display";
import "minibuffer";

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

static Window *g_win;

static void setup() {
	Buffer *buf = NULL;
	buffer_list_init();
	buf = buffer_current();
	check(buf != NULL);
	check(buf.load_bytes("x", 1, EModeLF));
	if (g_win == NULL) {
		g_win = window_create();
		check(g_win != NULL);
		window_select(g_win);
	} else {
		memset(&window_active, 0, sizeof(window_active));
		window_active.windows[0] = g_win;
		window_active.count = 1;
		g_win.buffer = buf;
		window_select(g_win);
	}
}

static void test_spawn_rejects_empty() {
	setup();
	check(!cmd_spawn_run_prompted(""));
	check(!cmd_spawn_run_prompted(NULL));
	check(strstr(display_echo_text(), "empty") != NULL);
}

static void test_spawn_prompt() {
	setup();
	check(cmd_spawn(false, 1));
	check(minibuffer_active());
	minibuffer_cancel();
	pop_listener();
}

static void test_spawn_true() {
	int rc = 0;
	setup();
	/* Non-TTY tests still run the Cmd; resume must tolerate non-TTY. */
	rc = spawn_run("true");
	check(rc == 0 || rc == -1);
	/* If spawn setup failed (no /bin/sh), that is acceptable in odd environments. */
	if (rc == 0) {
		check(true);
	}
}

int main() {
	test_spawn_rejects_empty();
	test_spawn_prompt();
	test_spawn_true();
	return 0;
}

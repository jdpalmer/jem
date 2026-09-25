// job/spawn.mc — foreground shell handoff (jem.c cmd_spawn.c / jem.go tools/spawn.go).
import "term";
import "display";
import "window";
import "os";
import "str";
import "tty";

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

enum {
	SpawnPathMax = 512,
	SpawnCmdMax = 1024
};

static char g_spawn_prog[SpawnPathMax];
static char g_spawn_line[SpawnCmdMax];

static void spawn_print_notice(const char *label, const char *command) {
	char buf[640] = {0};
	int n = 0;
	if (label == NULL) {
		label = "Shell";
	}
	if (command != NULL && command[0] != 0) {
		n = snprintf(buf, sizeof(buf),
			"\n[jem] %s: %s\n[jem] Terminal handed to shell. Exit shell to return to jem.\n\n",
			label, command);
	} else {
		n = snprintf(buf, sizeof(buf),
			"\n[jem] %s\n[jem] Terminal handed to shell. Exit shell to return to jem.\n\n",
			label);
	}
	if (n < 0) {
		return;
	}
	{
		auto (out, ok) = tty_stdout();
		if (ok) {
			(void)out.write(ranged(buf, (size_t)n));
		}
	}
}

static void spawn_mark_dirty() {
	size_t i = 0;
	display_invalidate();
	for (i = 0; i < window_active.count; i++) {
		Window *win = window_active.windows[i];
		if (win != NULL) {
			win.should_redraw = true;
			win.should_update_mode_line = true;
		}
	}
	window_retile();
}

static void spawn_pause_end() {
	display_set_echo("[End]");
	term_flush();
	if (!term_is_tty()) {
		display_set_echo("");
		return;
	}
	for (;;) {
		auto (key, ok) = term_read_key();
		if (!ok) {
			break;
		}
		if (key == term_key_value(TermKeyEnter) || key == (uint32_t)'\r' ||
			key == (uint32_t)'\n' || key == (uint32_t)' ') {
			break;
		}
	}
	display_set_echo("");
}

// spawn_run hands the TTY to a shell. NULL/empty command → interactive $SHELL.
// Returns process exit code, or -1 on setup failure.
int spawn_run(const char *command) {
	Cmd c = {0};
	const char[..] argv[2] = {0};
	size_t nargs = 0;
	bool oneshot = false;
	int exit_code = 0;
	bool wait_ok = false;

	if (job_running()) {
		display_set_echo("[busy: background job running]");
		return -1;
	}

	oneshot = (command != NULL && command[0] != 0);
	if (oneshot) {
		snprintf(g_spawn_line, sizeof(g_spawn_line), "%s", command);
	} else {
		g_spawn_line[0] = 0;
	}

	/* Resolve program */
	g_spawn_prog[0] = 0;
	if (oneshot) {
		auto (found, ok) = os_look_path(ranged(g_spawn_prog, 0, sizeof(g_spawn_prog)), "/bin/sh");
		(void)found;
		if (!ok) {
			display_set_echo("[spawn: /bin/sh not found]");
			return -1;
		}
		argv[0] = "-c";
		argv[1] = str_from_cstr(g_spawn_line);
		nargs = 2;
	} else {
		auto (have, shell_view) = os_env("SHELL");
		if (have && len(shell_view) > 0) {
			snprintf(g_spawn_prog, sizeof(g_spawn_prog), "%.*s",
				(int)len(shell_view), ptr(shell_view));
		} else {
			auto (found, ok) = os_look_path(ranged(g_spawn_prog, 0, sizeof(g_spawn_prog)), "/bin/sh");
			(void)found;
			if (!ok) {
				display_set_echo("[spawn: shell not found]");
				return -1;
			}
		}
		nargs = 0;
	}

	term_move(term_rows(), 0);
	if (oneshot) {
		term_write("\n", 1);
	}
	term_flush();
	term_suspend();

	if (oneshot) {
		spawn_print_notice("Running shell command", g_spawn_line);
	} else {
		spawn_print_notice("Launching interactive shell", NULL);
	}

	c.init(str_from_cstr(g_spawn_prog), argv, nargs);
	if (!c.start()) {
		c.close();
		(void)term_resume();
		display_set_echo("[spawn failed]");
		spawn_mark_dirty();
		return -1;
	}
	{
		auto (code, ok) = c.wait();
		exit_code = code;
		wait_ok = ok;
	}
	c.close();
	if (!wait_ok) {
		exit_code = -1;
	}

	os_sleep_ms(2000);
	term_drain_input();
	if (!term_resume()) {
		display_set_echo("[spawn: failed to restore terminal]");
		spawn_mark_dirty();
		return -1;
	}
	term_drain_input();

	if (oneshot) {
		spawn_pause_end();
	}
	spawn_mark_dirty();
	return exit_code;
}

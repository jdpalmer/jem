// runtime/describe.mc — describe-command (jem.go CmdDescribeCommand).
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

// cmd_describe_command fuzzy-picks a command and shows its documentation.
bool cmd_describe_command(bool f, int n) {
	const char *names[128] = {0};
	char binds_store[128][32] = {0};
	const char *binds[128] = {0};
	int i = 0;
	int ncmd = g_cmd_count;
	(void)f;
	(void)n;
	if (ncmd <= 0) {
		display_set_echo("[no commands]");
		return false;
	}
	if (ncmd > 128) {
		ncmd = 128;
	}
	for (i = 0; i < ncmd; i++) {
		names[i] = ptr(g_cmds[i].name);
		command_bind_label(g_cmds[i].fn, binds_store[i], sizeof(binds_store[i]));
		binds[i] = binds_store[i];
	}
	push_listener(ListenerDescribeCommand);
	fuzzy_command_open(names, binds, ncmd, "Describe: ", NULL);
	return true;
}

bool cmd_describe_command_on_accept() {
	char *sel = fuzzy_command_selected();
	char picked[64] = {0};
	Command *cmd = NULL;
	char msg[256] = {0};
	char bind[32] = {0};
	if (sel != NULL) {
		snprintf(picked, sizeof(picked), "%s", sel);
	}
	fuzzy_command_close();
	pop_listener();
	cmd = find_cmd_entry(picked);
	if (cmd == NULL) {
		display_set_echo("[unknown command]");
		return true;
	}
	command_bind_label(cmd.fn, bind, sizeof(bind));
	if (len(cmd.doc) > 0 && ptr(cmd.doc) != NULL && ptr(cmd.doc)[0] != 0) {
		if (bind[0] != 0) {
			snprintf(msg, sizeof(msg), "%s (%s): %s", ptr(cmd.name), bind, ptr(cmd.doc));
		} else {
			snprintf(msg, sizeof(msg), "%s: %s", ptr(cmd.name), ptr(cmd.doc));
		}
	} else if (bind[0] != 0) {
		snprintf(msg, sizeof(msg), "Command: %s (%s)", ptr(cmd.name), bind);
	} else {
		snprintf(msg, sizeof(msg), "Command: %s", ptr(cmd.name));
	}
	display_set_echo(msg);
	return true;
}

bool cmd_describe_command_on_abort() {
	fuzzy_command_close();
	pop_listener();
	display_set_echo("Quit");
	return true;
}

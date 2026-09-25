// runtime/spawn_cmds.mc — spawn / spawn-cli commands (C-x ! / M-!).
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

// cmd_spawn prompts then runs a one-line shell command (jem.go CmdSpawn).
bool cmd_spawn(bool f, int n) {
	(void)f;
	(void)n;
	push_listener(ListenerAskString);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = AskSpawn;
		}
	}
	minibuffer_begin("! ", 0);
	return true;
}

bool cmd_spawn_run_prompted(const char *command) {
	int rc = 0;
	if (command == NULL || command[0] == 0) {
		display_set_echo("[empty command]");
		return false;
	}
	rc = spawn_run(command);
	return rc != -1;
}

// cmd_spawn_cli hands the terminal to an interactive shell (jem.go RunSpawnCLI).
bool cmd_spawn_cli(bool f, int n) {
	int rc = 0;
	(void)f;
	(void)n;
	rc = spawn_run(NULL);
	if (rc == 0) {
		display_set_echo("[shell exited]");
	} else if (rc != -1) {
		char msg[64] = {0};
		snprintf(msg, sizeof(msg), "[shell exit %d]", rc);
		display_set_echo(str_from_cstr(msg));
	}
	return rc != -1;
}

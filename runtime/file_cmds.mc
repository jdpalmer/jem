// runtime/file_cmds.mc — delete-this-file and kill-buffer with confirmation.
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

static Buffer *g_delete_buf;
static char g_delete_path[512];

// cmd_kill_buffer asks before killing (jem.go CmdKillBuffer).
bool cmd_kill_buffer(bool f, int n) {
	Buffer *b = NULL;
	(void)f;
	if (n > 0) {
		if (n <= (int)len(buffer_all) && buffer_all[n - 1] != NULL) {
			b = buffer_all[n - 1];
			if (b.is_scratch()) {
				display_set_echo("[cannot kill *scratch*]");
				return false;
			}
			misc_begin_kill_confirm(b, "Kill buffer? (y/n) ");
			return true;
		}
		display_set_echo("[no such buffer]");
		return false;
	}
	b = buffer_current();
	if (b == NULL || b.is_scratch()) {
		display_set_echo("[cannot kill *scratch*]");
		return false;
	}
	misc_begin_kill_confirm(b, "Kill current buffer? (y/n) ");
	return true;
}

void misc_delete_file_yes() {
	Buffer *b = g_delete_buf;
	char path[512] = {0};
	snprintf(path, sizeof(path), "%s", g_delete_path);
	g_delete_buf = NULL;
	g_delete_path[0] = 0;
	if (path[0] == 0) {
		display_set_echo("[no such file]");
		return;
	}
	if (!fs_remove(str_from_cstr(path))) {
		display_set_echo("[delete failed]");
		return;
	}
	if (b != NULL && !b.is_scratch()) {
		(void)editor_kill_buffer(b);
	}
	display_set_echo("[deleted]");
}

void misc_delete_file_no() {
	g_delete_buf = NULL;
	g_delete_path[0] = 0;
	display_set_echo("[aborted]");
}

// cmd_delete_this_file deletes the on-disk file after confirmation, then kills the buffer.
bool cmd_delete_this_file(bool f, int n) {
	Buffer *buf = buffer_current();
	char path[512] = {0};
	(void)f;
	(void)n;
	if (buf == NULL || len(buf.file_name) == 0) {
		display_set_echo("[no file associated with buffer]");
		return false;
	}
	snprintf(path, sizeof(path), "%.*s", (int)len(buf.file_name), ptr(buf.file_name));
	{
		auto (info, ok) = fs_stat(str_from_cstr(path));
		if (!ok || info.kind != KindFile) {
			display_set_echo("[no such file]");
			return false;
		}
	}
	g_delete_buf = buf;
	snprintf(g_delete_path, sizeof(g_delete_path), "%s", path);
	push_listener(ListenerAskYesNo);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = AskYesDeleteFile;
		}
	}
	minibuffer_begin("Delete this file? (y/n) ", 1);
	return true;
}

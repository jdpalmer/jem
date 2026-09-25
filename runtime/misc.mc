// runtime/misc.mc — insert-date, show-position, set-eol-mode, kill-buffer-fuzzy.
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "host_time.h"

static Buffer *g_kill_pending;

static const char *month_name(int mon) {
	static const char *names[12] = {
		"Jan", "Feb", "Mar", "Apr", "May", "Jun",
		"Jul", "Aug", "Sep", "Oct", "Nov", "Dec"
	};
	if (mon < 0 || mon > 11) {
		return "?";
	}
	return names[mon];
}

static void buffer_char_stats(Buffer *buf, Window *win, int *char_at, int *before, int *total) {
	int cline = 1;
	size_t cbo = 0;
	int nch = 0;
	int at = '\n';
	int bef = 0;
	if (buf == NULL || win == NULL) {
		*char_at = '\n';
		*before = 0;
		*total = 0;
		return;
	}
	for (;;) {
		Line *line = buf.line(cline);
		size_t llen = line != NULL ? line.byte_len() : 0;
		if (cline == win.cursor.line && cbo == win.cursor.offset) {
			bef = nch;
			if (line == NULL || cbo == llen) {
				at = '\n';
			} else {
				char[..] v = line.view();
				at = ptr(v)[cbo];
			}
		}
		if (line == NULL || cbo == llen) {
			if (cline >= buf.line_count_i()) {
				break;
			}
			cline++;
			cbo = 0;
		} else {
			cbo++;
		}
		nch++;
	}
	*char_at = at;
	*before = bef;
	*total = nch;
}

// cmd_insert_date inserts the current date at point (jem.go CmdInsertDate).
bool cmd_insert_date(bool f, int n) {
	Window *win = window_current();
	char date[64] = {0};
	int64_t now = 0;
	struct JemTm *lt = NULL;
	Location at = {0};
	(void)f;
	(void)n;
	if (win == NULL || win.buffer == NULL) {
		return false;
	}
	if (win.buffer.is_readonly) {
		display_set_echo("[read-only buffer]");
		return false;
	}
	now = time(NULL);
	lt = localtime(&now);
	if (lt == NULL) {
		return false;
	}
	snprintf(date, sizeof(date), "%s %d, %d", month_name(lt.tm_mon), lt.tm_mday, lt.tm_year + 1900);
	at = win.cursor;
	buffer_undo.begin_command(win.buffer, at);
	if (!win.set_text(at, at, date, strlen(date))) {
		buffer_undo.end_command();
		return false;
	}
	buffer_undo.record_edit(UndoInsert, at.line, at.offset, date, strlen(date));
	buffer_undo.end_command();
	return true;
}

// cmd_show_position displays cursor line/column, character code, and buffer progress.
bool cmd_show_position(bool f, int n) {
	Window *win = window_current();
	Buffer *buf = buffer_current();
	int char_at = 0;
	int before = 0;
	int total = 0;
	int col = 0;
	int ratio = 0;
	int row = 0;
	char msg[128] = {0};
	(void)f;
	(void)n;
	if (win == NULL || buf == NULL) {
		return false;
	}
	buffer_char_stats(buf, win, &char_at, &before, &total);
	col = display_window_cursor_screen_col(win);
	if (total > 0) {
		ratio = (100 * before) / total;
	}
	row = display_cursor_row() + 1;
	snprintf(msg, sizeof(msg), "X=%d Y=%d CH=0x%x .=%d (%d%% of %d)",
		col + 1, row, char_at, before, ratio, total);
	display_set_echo(msg);
	return true;
}

static void eol_choose_redraw(int sel) {
	char msg[64] = {0};
	if (sel == 0) {
		snprintf(msg, sizeof(msg), "EOL mode: [LF] CRLF");
	} else {
		snprintf(msg, sizeof(msg), "EOL mode: LF [CRLF]");
	}
	display_set_echo(msg);
}

static void apply_eol_mode(Buffer *buf, EolMode mode) {
	size_t i = 0;
	const char *label = "LF";
	if (buf == NULL) {
		return;
	}
	if (buf.eol_mode != mode) {
		buf.eol_mode = mode;
		buf.mark_changed();
		for (i = 0; i < window_active.count; i++) {
			Window *win = window_active.windows[i];
			if (win != NULL && win.buffer == buf) {
				win.should_update_mode_line = true;
			}
		}
	}
	if (mode == EModeCRLF) {
		label = "CRLF";
	}
	{
		char msg[64] = {0};
		snprintf(msg, sizeof(msg), "[EOL mode: %s]", label);
		display_set_echo(msg);
	}
}

// cmd_set_eol_mode prompts for LF vs CRLF (jem.go CmdSetEolMode).
bool cmd_set_eol_mode(bool f, int n) {
	Buffer *buf = buffer_current();
	int def = 0;
	(void)f;
	(void)n;
	if (buf == NULL) {
		return false;
	}
	if (buf.is_readonly) {
		display_set_echo("[read-only buffer]");
		return false;
	}
	if (buf.eol_mode == EModeCRLF) {
		def = 1;
	}
	push_listener(ListenerAskChoose);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = def;
		}
	}
	eol_choose_redraw(def);
	return true;
}

// cmd_choose_handle_key drives the EOL choose menu. Returns true if the key was consumed.
bool cmd_choose_handle_key(uint32_t key) {
	Listener *L = top_listener();
	Buffer *buf = buffer_current();
	int sel = 0;
	if (L == NULL || L.kind != ListenerAskChoose) {
		return false;
	}
	sel = L.ask_phase;
	if (key == 27 || key == (term_key_value(TermCTL) | (uint32_t)'G') || key == 0x07) {
		pop_listener();
		display_set_echo("Quit");
		return true;
	}
	if (key == term_key_value(TermKeyEnter) || key == (uint32_t)'\r' || key == (uint32_t)'\n' ||
		key == (term_key_value(TermCTL) | (uint32_t)'M') || key == (term_key_value(TermCTL) | (uint32_t)'J')) {
		pop_listener();
		apply_eol_mode(buf, sel == 0 ? EModeLF : EModeCRLF);
		return true;
	}
	if (key == (uint32_t)'l' || key == (uint32_t)'L') {
		pop_listener();
		apply_eol_mode(buf, EModeLF);
		return true;
	}
	if (key == (uint32_t)'c' || key == (uint32_t)'C') {
		pop_listener();
		apply_eol_mode(buf, EModeCRLF);
		return true;
	}
	if (key == term_key_value(TermKeyLeft) || key == (term_key_value(TermCTL) | (uint32_t)'B') ||
		key == term_key_value(TermKeyUp)) {
		if (sel > 0) {
			sel--;
		}
		L.ask_phase = sel;
		eol_choose_redraw(sel);
		return true;
	}
	if (key == term_key_value(TermKeyRight) || key == (term_key_value(TermCTL) | (uint32_t)'F') ||
		key == term_key_value(TermKeyDown)) {
		if (sel < 1) {
			sel++;
		}
		L.ask_phase = sel;
		eol_choose_redraw(sel);
		return true;
	}
	return true;
}

bool editor_kill_buffer(Buffer *b) {
	Buffer *replacement = NULL;
	if (b == NULL || b.is_scratch()) {
		return false;
	}
	for (auto other : buffer_all) {
		if (other != b) {
			replacement = other;
			break;
		}
	}
	if (replacement == NULL) {
		return false;
	}
	window_replace_buffer(b, replacement);
	buffer_undo.forget_buffer(b);
	buffer_set_current(replacement);
	buffer_release(b);
	return true;
}

static bool kill_buffer_now(Buffer *b) {
	return editor_kill_buffer(b);
}

void misc_kill_pending_yes() {
	Buffer *b = g_kill_pending;
	g_kill_pending = NULL;
	if (b == NULL) {
		display_set_echo("[buffer not found]");
		return;
	}
	if (b.is_scratch()) {
		display_set_echo("[cannot kill *scratch*]");
		return;
	}
	if (!kill_buffer_now(b)) {
		display_set_echo("[cannot kill buffer]");
		return;
	}
	display_set_echo("[buffer killed]");
}

void misc_kill_pending_no() {
	g_kill_pending = NULL;
	display_set_echo("[aborted]");
}

void misc_begin_kill_confirm(Buffer *buf, const char *prompt) {
	g_kill_pending = buf;
	push_listener(ListenerAskYesNo);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = AskYesKillBuffer;
		}
	}
	minibuffer_begin(str_from_cstr(prompt), 1);
}

static void begin_kill_confirm(Buffer *buf) {
	misc_begin_kill_confirm(buf, "Kill buffer? (y/n) ");
}

// cmd_kill_buffer_fuzzy prompts with a fuzzy list and kills after confirmation.
bool cmd_kill_buffer_fuzzy(bool f, int n) {
	(void)f;
	(void)n;
	push_listener(ListenerKillBuffer);
	fuzzy_buffer_open_with_prompt("Kill buffer: ");
	return true;
}

// cmd_kill_buffer_fuzzy_on_accept runs after fuzzy accept.
bool cmd_kill_buffer_fuzzy_on_accept() {
	Buffer *buf = fuzzy_buffer_selected();
	fuzzy_buffer_close();
	pop_listener();
	if (buf == NULL) {
		display_set_echo("[buffer not found]");
		return true;
	}
	if (buf.is_scratch()) {
		display_set_echo("[cannot kill *scratch*]");
		return true;
	}
	begin_kill_confirm(buf);
	return true;
}

bool cmd_kill_buffer_fuzzy_on_abort() {
	fuzzy_buffer_close();
	pop_listener();
	display_set_echo("Quit");
	return true;
}

// runtime — command loop, keybindings, listeners, config, macros.
import "event";
import "buffer";
import "window";
import "term";
import "display";
import "file";
import "killring";
import "markring";
import "minibuffer";
import "search";
import "register";
import "mode";
import "job";
import "tags";
import "os";
import "fs";
import "path";
import "str";

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <unistd.h>

typedef bool (*CommandFn)(bool f, int n);

struct Command {
	const char[..] name;
	CommandFn fn;
};

struct KeyBind {
	uint32_t key;
	CommandFn fn;
};

enum ListenerKind {
	ListenerNone = 0,
	ListenerUniversal = 1,
	ListenerIsearch = 2,
	ListenerReplace = 3,
	ListenerAskString = 4,
	ListenerAskYesNo = 5,
	ListenerCtlx = 6,
	ListenerMx = 7, /* fuzzy command palette (M-x) */
	ListenerUseBuffer = 8,
	ListenerTagPicker = 9
};

enum AskStringKind {
	AskFindFile = 0,
	AskWriteFile = 1,
	AskReadFile = 2,
	AskGotoLine = 3,
	AskGrep = 4,
	AskCompile = 5,
	AskGotoTag = 6
};

struct Listener {
	ListenerKind kind;
	int universal_n;
	bool universal_f;
	char ask_buf[512];
	int ask_phase; /* replace: 0=needle 1=replacement 2=confirm */
};

struct App {
	bool running;
	bool dispatching;
	Listener listeners[8];
	int listener_top;
	/* macro */
	bool macro_recording;
	bool macro_playing;
	Event macro_tape[256];
	int macro_len;
	int macro_pos;
	/* config */
	bool auto_indent;
};

static App g_app;
static KeyBind g_binds[128];
static int g_bind_count;
static Command g_cmds[128];
static int g_cmd_count;
static bool g_escape_pending;

/* Map raw bytes to editor keys (jem.go/display.DecodeKeyChar). */
static uint32_t decode_key_char(uint32_t key) {
	if (key == (uint32_t)'\t') {
		return term_key_value(TermKeyTab);
	}
	if (key == (uint32_t)'\r' || key == (uint32_t)'\n') {
		return term_key_value(TermKeyEnter);
	}
	if (key == 0x1B) {
		return 0x1B;
	}
	if (key == 0) {
		return term_key_value(TermCTL) | (uint32_t)' ';
	}
	if (key >= (uint32_t)0x01 && key <= (uint32_t)0x1F) {
		return term_key_value(TermCTL) | (key + (uint32_t)'@');
	}
	return key;
}

/* Esc-as-Meta: lowercase letters become META|Upper (jem.go ApplyMetaPrefixToKey). */
static uint32_t apply_meta_prefix(uint32_t k) {
	if ((k & term_key_value(TermKeyMask)) == 0 && k >= (uint32_t)'a' && k <= (uint32_t)'z') {
		return term_key_value(TermMETA) | (k - ((uint32_t)'a' - (uint32_t)'A'));
	}
	return k | term_key_value(TermMETA);
}

/* Kitty Alt+letter reports lowercase; bindings use META|'X'. */
static uint32_t normalize_meta_case(uint32_t k) {
	uint32_t base = k & ~term_key_value(TermKeyMask);
	if ((k & term_key_value(TermMETA)) != 0 && base >= (uint32_t)'a' && base <= (uint32_t)'z') {
		return (k & term_key_value(TermKeyMask)) | (base - ((uint32_t)'a' - (uint32_t)'A'));
	}
	return k;
}

/* Compose Esc-as-Meta / normalize; returns false if key is held as meta prefix. */
static bool prepare_editor_key(uint32_t raw, uint32_t *out_key) {
	uint32_t k = decode_key_char(raw);
	if (g_escape_pending) {
		g_escape_pending = false;
		display_set_echo("");
		*out_key = normalize_meta_case(apply_meta_prefix(k));
		return true;
	}
	if (k == 0x1B) {
		/* Esc quits prompts; otherwise start M- chord (jem.go decodeAndDeliver). */
		if (minibuffer_active() || g_app.listener_top > 0) {
			*out_key = 0x1B;
			return true;
		}
		g_escape_pending = true;
		display_set_echo("ESC");
		return false;
	}
	*out_key = normalize_meta_case(k);
	return true;
}

static void bind_key(uint32_t key, CommandFn fn) {
	if (g_bind_count >= 128) {
		return;
	}
	g_binds[g_bind_count].key = key;
	g_binds[g_bind_count].fn = fn;
	g_bind_count++;
}

static void reg_cmd(const char[..] name, CommandFn fn) {
	if (g_cmd_count >= 128) {
		return;
	}
	g_cmds[g_cmd_count].name = name;
	g_cmds[g_cmd_count].fn = fn;
	g_cmd_count++;
}

static CommandFn find_cmd(const char *name) {
	int i = 0;
	if (name == NULL) {
		return NULL;
	}
	for (i = 0; i < g_cmd_count; i++) {
		if (str_eq(g_cmds[i].name, str_from_cstr(name))) {
			return g_cmds[i].fn;
		}
	}
	return NULL;
}

static CommandFn find_bind(uint32_t key) {
	int i = 0;
	for (i = 0; i < g_bind_count; i++) {
		if (g_binds[i].key == key) {
			return g_binds[i].fn;
		}
	}
	return NULL;
}

static void push_listener(ListenerKind kind) {
	Listener *L = {0};
	if (g_app.listener_top >= 8) {
		return;
	}
	L = &g_app.listeners[g_app.listener_top++];
	memset(L, 0, sizeof(*L));
	L.kind = kind;
}

static void pop_listener() {
	if (g_app.listener_top > 0) {
		g_app.listener_top--;
	}
}

static Listener *top_listener() {
	if (g_app.listener_top == 0) {
		return NULL;
	}
	return &g_app.listeners[g_app.listener_top - 1];
}

static bool any_unsaved() {
	for (auto b : buffer_all) {
		if (b != NULL && b.is_changed && !b.is_scratch()) {
			return true;
		}
	}
	return false;
}

/* ---- commands ------------------------------------------------------------ */

bool cmd_forward_char(bool f, int n) {
	Window *w = window_current();
	(void)f;
	if (n < 1) {
		n = 1;
	}
	if (w != NULL) {
		w.move_forward_char(n);
	}
	return true;
}

bool cmd_backward_char(bool f, int n) {
	Window *w = window_current();
	(void)f;
	if (n < 1) {
		n = 1;
	}
	if (w != NULL) {
		w.move_backward_char(n);
	}
	return true;
}

bool cmd_next_line(bool f, int n) {
	Window *w = window_current();
	(void)f;
	if (n < 1) {
		n = 1;
	}
	if (w != NULL) {
		w.move_vertical(n);
	}
	return true;
}

bool cmd_prev_line(bool f, int n) {
	Window *w = window_current();
	(void)f;
	if (n < 1) {
		n = 1;
	}
	if (w != NULL) {
		w.move_vertical(-n);
	}
	return true;
}

bool cmd_forward_word(bool f, int n) {
	Window *w = window_current();
	(void)f;
	if (n < 1) {
		n = 1;
	}
	if (w != NULL) {
		w.move_forward_word(n);
	}
	return true;
}

bool cmd_backward_word(bool f, int n) {
	Window *w = window_current();
	(void)f;
	if (n < 1) {
		n = 1;
	}
	if (w != NULL) {
		w.move_backward_word(n);
	}
	return true;
}

bool cmd_forward_page(bool f, int n) {
	Window *w = window_current();
	int page = 1;
	(void)f;
	if (n < 1) {
		n = 1;
	}
	if (w == NULL) {
		return true;
	}
	page = w.height;
	if (page > 2) {
		page = w.height - 2;
	} else {
		page = 1;
	}
	return cmd_next_line(false, page * n);
}

bool cmd_backward_page(bool f, int n) {
	Window *w = window_current();
	int page = 1;
	(void)f;
	if (n < 1) {
		n = 1;
	}
	if (w == NULL) {
		return true;
	}
	page = w.height;
	if (page > 2) {
		page = w.height - 2;
	} else {
		page = 1;
	}
	return cmd_prev_line(false, page * n);
}

bool cmd_beginning_of_line(bool f, int n) {
	Window *w = window_current();
	(void)f;
	(void)n;
	if (w != NULL) {
		w.move_bol();
	}
	return true;
}

bool cmd_end_of_line(bool f, int n) {
	Window *w = window_current();
	(void)f;
	(void)n;
	if (w != NULL) {
		w.move_eol();
	}
	return true;
}

bool cmd_self_insert(bool f, int n) {
	(void)f;
	(void)n;
	/* actual char comes from dispatch via pending — use last key in static */
	return true;
}

static uint32_t g_pending_insert;

bool cmd_insert_pending(bool f, int n) {
	Window *w = window_current();
	char utf[8] = {0};
	int len = 0;
	uint32_t ch = g_pending_insert;
	Location at = {0};
	(void)f;
	if (w == NULL || ch < 32) {
		return true;
	}
	if (n < 1) {
		n = 1;
	}
	if (ch < 128) {
		utf[0] = (char)ch;
		len = 1;
	} else if (ch < 0x800) {
		utf[0] = (char)(0xC0 | (ch >> 6));
		utf[1] = (char)(0x80 | (ch & 0x3F));
		len = 2;
	} else if (ch < 0x10000) {
		utf[0] = (char)(0xE0 | (ch >> 12));
		utf[1] = (char)(0x80 | ((ch >> 6) & 0x3F));
		utf[2] = (char)(0x80 | (ch & 0x3F));
		len = 3;
	} else {
		utf[0] = (char)(0xF0 | (ch >> 18));
		utf[1] = (char)(0x80 | ((ch >> 12) & 0x3F));
		utf[2] = (char)(0x80 | ((ch >> 6) & 0x3F));
		utf[3] = (char)(0x80 | (ch & 0x3F));
		len = 4;
	}
	at = w.cursor;
	buffer_undo.begin_command(w.buffer, at);
	while (n-- > 0) {
		if (!w.set_text(w.cursor, w.cursor, utf, len)) {
			break;
		}
		buffer_undo.record_edit(UndoInsert, at.line, at.offset, utf, len);
		at = w.cursor;
	}
	buffer_undo.end_command();
	return true;
}

bool cmd_newline(bool f, int n) {
	Window *w = window_current();
	char nl = '\n';
	(void)f;
	if (w == NULL) {
		return true;
	}
	if (n < 1) {
		n = 1;
	}
	buffer_undo.begin_command(w.buffer, w.cursor);
	while (n-- > 0) {
		Location at = w.cursor;
		if (!w.set_text(w.cursor, w.cursor, &nl, 1)) {
			break;
		}
		buffer_undo.record_edit(UndoInsert, at.line, at.offset, &nl, 1);
	}
	buffer_undo.end_command();
	return true;
}

/* Mode-aware newline+indent (jem.go CmdModeNewlineAndIndent). */
bool cmd_newline_and_indent(bool f, int n) {
	Buffer *b = buffer_current();
	(void)f;
	if (b != NULL && len(b.name) > 0) {
		if (str_eq(b.name, "*grep*") || str_eq(b.name, "*compile*")) {
			return job_visit_at_point();
		}
		if (str_eq(b.name, "*tags*") ||
			str_eq(b.name, "*describe*")) {
			return tags_visit_at_point();
		}
	}
	if (!g_app.auto_indent) {
		return cmd_newline(false, n);
	}
	return mode_newline_and_indent(false, n);
}

bool cmd_indent_line(bool f, int n) {
	return mode_indent_line(f, n);
}

bool cmd_close_brace(bool f, int n) {
	return mode_close_brace(f, n);
}

bool cmd_open_line(bool f, int n) {
	Window *w = window_current();
	(void)f;
	if (w == NULL) {
		return true;
	}
	if (n < 0) {
		return false;
	}
	if (n == 0) {
		return true;
	}
	if (!cmd_newline(false, n)) {
		return false;
	}
	return cmd_backward_char(false, n);
}

static char *region_text(Window *w, Location a, Location b, size_t *out_n) {
	Location begin = a;
	Location end = b;
	char *data = {0};
	size_t n = 0;
	bool ok = false;
	Buffer *tmp = {0};
	if (w == NULL || w.buffer == NULL) {
		return NULL;
	}
	if (begin.line > end.line || (begin.line == end.line && begin.offset > end.offset)) {
		Location t = begin;
		begin = end;
		end = t;
	}
	{
		int line = begin.line;
		size_t cap = 0;
		while (line <= end.line) {
			Line *ln = w.buffer.line(line);
			size_t from = line == begin.line ? begin.offset : 0;
			size_t to = 0;
			size_t chunk = 0;
			if (line == end.line) {
				to = end.offset;
			} else if (ln != NULL) {
				to = ln.byte_len();
			}
			chunk = to > from ? to - from : 0;
			if (n + chunk + 2 > cap) {
				size_t ncap = cap != 0 ? cap * 2 : 256;
				char *p = {0};
				while (ncap < n + chunk + 2) {
					ncap *= 2;
				}
				p = realloc(data, ncap);
				if (p == NULL) {
					free(data);
					return NULL;
				}
				data = p;
				cap = ncap;
			}
			if (chunk > 0 && ln != NULL) {
				memcpy(data + n, ptr(ln.data) + from, chunk);
				n += chunk;
			}
			if (line < end.line) {
				data[n++] = '\n';
			}
			line++;
		}
		if (data != NULL) {
			data[n] = 0;
		}
	}
	if (out_n != NULL) {
		*out_n = n;
	}
	(void)tmp;
	(void)ok;
	return data;
}

static bool delete_range_undo(Window *w, Location begin, Location end) {
	char *text = {0};
	size_t tlen = 0;
	Location lo = begin;
	Location hi = end;
	if (w == NULL) {
		return false;
	}
	if (begin.line > end.line || (begin.line == end.line && begin.offset > end.offset)) {
		lo = end;
		hi = begin;
	}
	if (lo.line == hi.line && lo.offset == hi.offset) {
		return false;
	}
	text = region_text(w, lo, hi, &tlen);
	if (!w.set_text(lo, hi, "", 0)) {
		free(text);
		return false;
	}
	if (text != NULL) {
		buffer_undo.record_edit(UndoDelete, w.cursor.line, w.cursor.offset, text, tlen);
		free(text);
	}
	return true;
}

bool cmd_delete_word_forward(bool f, int n) {
	Window *w = window_current();
	(void)f;
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	if (n <= 0) {
		return false;
	}
	buffer_undo.begin_command(w.buffer, w.cursor);
	while (n-- > 0) {
		Location begin = w.cursor;
		Location end = {0};
		w.move_forward_word(1);
		end = w.cursor;
		if (begin.line == end.line && begin.offset == end.offset) {
			break;
		}
		if (!delete_range_undo(w, begin, end)) {
			buffer_undo.end_command();
			return false;
		}
	}
	buffer_undo.end_command();
	return true;
}

bool cmd_delete_word_backward(bool f, int n) {
	Window *w = window_current();
	(void)f;
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	if (n <= 0) {
		return false;
	}
	buffer_undo.begin_command(w.buffer, w.cursor);
	while (n-- > 0) {
		Location end = w.cursor;
		Location begin = {0};
		w.move_backward_word(1);
		begin = w.cursor;
		if (begin.line == end.line && begin.offset == end.offset) {
			break;
		}
		if (!delete_range_undo(w, begin, end)) {
			buffer_undo.end_command();
			return false;
		}
	}
	buffer_undo.end_command();
	return true;
}

bool cmd_goto_bof(bool f, int n) {
	Window *w = window_current();
	Location loc = {0};
	(void)f;
	(void)n;
	if (w == NULL) {
		return true;
	}
	loc.line = 1;
	loc.offset = 0;
	w.set_cursor(loc);
	return true;
}

bool cmd_goto_eof(bool f, int n) {
	Window *w = window_current();
	Line *line = {0};
	Location loc = {0};
	(void)f;
	(void)n;
	if (w == NULL || w.buffer == NULL) {
		return true;
	}
	loc.line = w.buffer.line_count_i();
	if (loc.line < 1) {
		loc.line = 1;
	}
	line = w.buffer.line(loc.line);
	if (line != NULL) {
		loc.offset = line.byte_len();
	}
	w.set_cursor(loc);
	return true;
}

bool cmd_back_to_indentation(bool f, int n) {
	Window *w = window_current();
	Line *line = {0};
	Location loc = {0};
	(void)f;
	(void)n;
	if (w == NULL || w.buffer == NULL) {
		return true;
	}
	loc.line = w.cursor.line;
	line = w.buffer.line(loc.line);
	if (line != NULL) {
		loc.offset = line.first_nonblank();
	}
	w.set_cursor(loc);
	return true;
}

bool cmd_mark_whole_buffer(bool f, int n) {
	Window *w = window_current();
	Location mark = {0};
	Location cur = {0};
	(void)f;
	(void)n;
	if (w == NULL || w.buffer == NULL) {
		return true;
	}
	mark.line = w.buffer.eof_line();
	mark.offset = 0;
	w.mark = mark;
	cur.line = 1;
	cur.offset = 0;
	w.set_cursor(cur);
	w.should_redraw = true;
	display_set_echo("Mark set");
	return true;
}

bool cmd_refresh(bool f, int n) {
	(void)f;
	(void)n;
	display_invalidate();
	return true;
}

bool cmd_trim_whitespace(bool f, int n) {
	Window *w = window_current();
	Line *line = {0};
	char[..] v = {0};
	size_t pos = 0;
	size_t start = 0;
	size_t end = 0;
	Location begin = {0};
	Location finish = {0};
	(void)f;
	(void)n;
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	line = w.buffer.line(w.cursor.line);
	if (line == NULL) {
		return false;
	}
	v = line.view();
	{
		char *vp = ptr(v);
		pos = w.cursor.offset;
		start = pos;
		while (start > 0) {
			char c = vp[start - 1];
			if (c != ' ' && c != '\t') {
				break;
			}
			start--;
		}
		end = pos;
		while (end < len(v)) {
			char c = vp[end];
			if (c != ' ' && c != '\t') {
				break;
			}
			end++;
		}
		if (start == end) {
			return true;
		}
	}
	begin.line = w.cursor.line;
	begin.offset = start;
	finish.line = w.cursor.line;
	finish.offset = end;
	buffer_undo.begin_command(w.buffer, w.cursor);
	if (!delete_range_undo(w, begin, finish)) {
		buffer_undo.end_command();
		return false;
	}
	buffer_undo.end_command();
	return true;
}

bool cmd_transpose_chars(bool f, int n) {
	Window *w = window_current();
	Line *line = {0};
	char[..] v = {0};
	size_t offset = 0;
	size_t right_start = 0;
	size_t right_end = 0;
	size_t left_start = 0;
	size_t left_end = 0;
	char *swapped = {0};
	size_t left_len = 0;
	size_t right_len = 0;
	Location begin = {0};
	Location end = {0};
	(void)f;
	(void)n;
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	line = w.buffer.line(w.cursor.line);
	if (line == NULL) {
		return false;
	}
	v = line.view();
	offset = w.cursor.offset;
	if (offset == len(v)) {
		right_end = offset;
		right_start = utf8_prev_offset(v, right_end);
	} else {
		right_start = offset;
		right_end = utf8_next_offset(v, right_start);
	}
	if (right_start == right_end) {
		return false;
	}
	left_end = right_start;
	left_start = utf8_prev_offset(v, left_end);
	if (left_start == left_end) {
		return false;
	}
	left_len = left_end - left_start;
	right_len = right_end - right_start;
	swapped = malloc(left_len + right_len);
	if (swapped == NULL) {
		return false;
	}
	{
		char *vp = ptr(v);
		memcpy(swapped, vp + right_start, right_len);
		memcpy(swapped + right_len, vp + left_start, left_len);
	}
	begin.line = w.cursor.line;
	begin.offset = left_start;
	end.line = w.cursor.line;
	end.offset = right_end;
	buffer_undo.begin_command(w.buffer, w.cursor);
	{
		char *old = region_text(w, begin, end, NULL);
		size_t olen = left_len + right_len;
		if (!w.set_text(begin, end, swapped, left_len + right_len)) {
			free(swapped);
			free(old);
			buffer_undo.end_command();
			return false;
		}
		if (old != NULL) {
			buffer_undo.record_edit(UndoDelete, begin.line, begin.offset, old, olen);
			free(old);
		}
		buffer_undo.record_edit(UndoInsert, begin.line, begin.offset, swapped, left_len + right_len);
	}
	free(swapped);
	buffer_undo.end_command();
	return true;
}

static char ascii_tolower(char c) {
	if (c >= 'A' && c <= 'Z') {
		return (char)(c + ('a' - 'A'));
	}
	return c;
}

static char ascii_toupper(char c) {
	if (c >= 'a' && c <= 'z') {
		return (char)(c - ('a' - 'A'));
	}
	return c;
}

static bool transform_word_case(int mode) {
	/* mode: 0=lower, 1=upper, 2=capitalize */
	Window *w = window_current();
	Location start = {0};
	Location end = {0};
	char *text = {0};
	size_t tlen = 0;
	size_t i = 0;
	bool first = true;
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	start = w.cursor;
	w.move_backward_word(1);
	start = w.cursor;
	w.move_forward_word(1);
	end = w.cursor;
	text = region_text(w, start, end, &tlen);
	if (text == NULL || tlen == 0) {
		free(text);
		w.set_cursor(start);
		return false;
	}
	for (i = 0; i < tlen; i++) {
		if (mode == 0) {
			text[i] = ascii_tolower(text[i]);
		} else if (mode == 1) {
			text[i] = ascii_toupper(text[i]);
		} else {
			if (first && ((text[i] >= 'a' && text[i] <= 'z') || (text[i] >= 'A' && text[i] <= 'Z'))) {
				text[i] = ascii_toupper(text[i]);
				first = false;
			} else if ((text[i] >= 'a' && text[i] <= 'z') || (text[i] >= 'A' && text[i] <= 'Z')) {
				text[i] = ascii_tolower(text[i]);
				first = false;
			}
		}
	}
	buffer_undo.begin_command(w.buffer, w.cursor);
	{
		char *old = region_text(w, start, end, NULL);
		if (!w.set_text(start, end, text, tlen)) {
			free(text);
			free(old);
			buffer_undo.end_command();
			return false;
		}
		if (old != NULL) {
			buffer_undo.record_edit(UndoDelete, start.line, start.offset, old, tlen);
			free(old);
		}
		buffer_undo.record_edit(UndoInsert, start.line, start.offset, text, tlen);
	}
	free(text);
	buffer_undo.end_command();
	return true;
}

bool cmd_lower_word(bool f, int n) {
	(void)f;
	(void)n;
	return transform_word_case(0);
}

bool cmd_upper_word(bool f, int n) {
	(void)f;
	(void)n;
	return transform_word_case(1);
}

bool cmd_cap_word(bool f, int n) {
	(void)f;
	(void)n;
	return transform_word_case(2);
}

static bool is_word_byte(char b) {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_';
}

static Location next_word_start(Buffer *buf, Location loc) {
	Line *line = {0};
	char[..] v = {0};
	size_t off = 0;
	int nlines = 0;
	if (buf == NULL) {
		return loc;
	}
	nlines = buf.line_count_i();
	line = buf.line(loc.line);
	if (line == NULL) {
		return loc;
	}
	v = line.view();
	off = loc.offset;
	while (loc.line <= nlines) {
		line = buf.line(loc.line);
		if (line == NULL) {
			return loc;
		}
		v = line.view();
		{
			char *vp = ptr(v);
			while (off < len(v) && !is_word_byte(vp[off])) {
				off++;
			}
		}
		if (off < len(v)) {
			loc.offset = off;
			return loc;
		}
		if (loc.line >= nlines) {
			loc.offset = 0;
			return loc;
		}
		loc.line++;
		off = 0;
	}
	loc.line = nlines;
	loc.offset = 0;
	return loc;
}

bool cmd_transpose_words(bool f, int n) {
	Window *w = window_current();
	Location left_start = {0};
	Location left_end = {0};
	Location right_start = {0};
	Location right_end = {0};
	char *left_text = {0};
	char *right_text = {0};
	size_t left_len = 0;
	size_t right_len = 0;
	(void)f;
	(void)n;
	if (w == NULL || w.buffer == NULL || w.buffer.is_readonly) {
		return false;
	}
	left_start = w.cursor;
	w.move_backward_word(1);
	left_start = w.cursor;
	w.move_forward_word(1);
	left_end = w.cursor;
	right_start = next_word_start(w.buffer, left_end);
	if (right_start.line == left_end.line && right_start.offset == left_end.offset) {
		return false;
	}
	w.set_cursor(right_start);
	w.move_forward_word(1);
	right_end = w.cursor;
	left_text = region_text(w, left_start, left_end, &left_len);
	right_text = region_text(w, right_start, right_end, &right_len);
	if (left_text == NULL || right_text == NULL) {
		free(left_text);
		free(right_text);
		return false;
	}
	buffer_undo.begin_command(w.buffer, w.cursor);
	/* Replace right then left (jem.go CmdTransposeWords). */
	if (!w.set_text(right_start, right_end, left_text, left_len)) {
		free(left_text);
		free(right_text);
		buffer_undo.end_command();
		return false;
	}
	buffer_undo.record_edit(UndoDelete, right_start.line, right_start.offset, right_text, right_len);
	buffer_undo.record_edit(UndoInsert, right_start.line, right_start.offset, left_text, left_len);
	if (!w.set_text(left_start, left_end, right_text, right_len)) {
		free(left_text);
		free(right_text);
		buffer_undo.end_command();
		return false;
	}
	buffer_undo.record_edit(UndoDelete, left_start.line, left_start.offset, left_text, left_len);
	buffer_undo.record_edit(UndoInsert, left_start.line, left_start.offset, right_text, right_len);
	free(left_text);
	free(right_text);
	buffer_undo.end_command();
	return true;
}

bool cmd_delete_backward(bool f, int n) {
	Window *w = window_current();
	(void)f;
	if (w == NULL) {
		return true;
	}
	if (n < 1) {
		n = 1;
	}
	buffer_undo.begin_command(w.buffer, w.cursor);
	while (n-- > 0) {
		Location end = w.cursor;
		Location begin = end;
		Line *line = w.buffer.line(end.line);
		if (end.offset > 0 && line != NULL) {
			begin.offset = utf8_prev_offset(line.view(), end.offset);
		} else if (end.line > 1) {
			Line *prev = w.buffer.line(end.line - 1);
			begin.line = end.line - 1;
			begin.offset = prev != NULL ? prev.byte_len() : 0;
		} else {
			break;
		}
		if (!delete_range_undo(w, begin, end)) {
			break;
		}
	}
	buffer_undo.end_command();
	return true;
}

bool cmd_delete_forward(bool f, int n) {
	Window *w = window_current();
	(void)f;
	if (w == NULL) {
		return true;
	}
	if (n < 1) {
		n = 1;
	}
	buffer_undo.begin_command(w.buffer, w.cursor);
	while (n-- > 0) {
		Location begin = w.cursor;
		Location end = begin;
		Line *line = w.buffer.line(begin.line);
		if (line != NULL && begin.offset < line.byte_len()) {
			end.offset = utf8_next_offset(line.view(), begin.offset);
		} else if (begin.line < w.buffer.line_count_i()) {
			end.line = begin.line + 1;
			end.offset = 0;
		} else {
			break;
		}
		if (!delete_range_undo(w, begin, end)) {
			break;
		}
	}
	buffer_undo.end_command();
	return true;
}

bool cmd_set_mark(bool f, int n) {
	Window *w = window_current();
	(void)f;
	(void)n;
	if (w != NULL) {
		w.mark = w.cursor;
		markring_push(w.cursor);
		w.should_redraw = true;
		display_set_echo("Mark set");
	}
	return true;
}

bool cmd_exchange_point_and_mark(bool f, int n) {
	Window *w = window_current();
	Location t = {0};
	(void)f;
	(void)n;
	if (w == NULL || w.mark.line == 0) {
		return true;
	}
	t = w.cursor;
	w.set_cursor(w.mark);
	w.mark = t;
	w.should_redraw = true;
	return true;
}

bool cmd_kill_region(bool f, int n) {
	Window *w = window_current();
	char *text = {0};
	size_t tlen = 0;
	Location begin = {0};
	Location end = {0};
	(void)f;
	(void)n;
	if (w == NULL || w.mark.line == 0) {
		display_set_echo("No mark set");
		return true;
	}
	begin = w.mark;
	end = w.cursor;
	text = region_text(w, begin, end, &tlen);
	if (text != NULL) {
		killring_kill(ranged(text, tlen));
	}
	buffer_undo.begin_command(w.buffer, w.cursor);
	{
		Location lo = begin;
		Location hi = end;
		if (begin.line > end.line || (begin.line == end.line && begin.offset > end.offset)) {
			lo = end;
			hi = begin;
		}
		w.set_text(lo, hi, "", 0);
	}
	if (text != NULL) {
		buffer_undo.record_edit(UndoDelete, w.cursor.line, w.cursor.offset, text, tlen);
		free(text);
	}
	buffer_undo.end_command();
	w.mark.line = 0;
	w.mark.offset = 0;
	w.should_redraw = true;
	return true;
}

bool cmd_copy_region(bool f, int n) {
	Window *w = window_current();
	char *text = {0};
	size_t tlen = 0;
	(void)f;
	(void)n;
	if (w == NULL || w.mark.line == 0) {
		display_set_echo("No mark set");
		return true;
	}
	text = region_text(w, w.mark, w.cursor, &tlen);
	if (text == NULL) {
		display_set_echo("Out of memory");
		return true;
	}
	killring_kill(ranged(text, tlen));
	free(text);
	w.mark.line = 0;
	w.mark.offset = 0;
	w.should_redraw = true;
	display_set_echo("Region copied");
	return true;
}

bool cmd_kill_line(bool f, int n) {
	Window *w = window_current();
	Location begin = {0};
	Location end = {0};
	Line *line = {0};
	char *text = {0};
	size_t tlen = 0;
	(void)f;
	(void)n;
	if (w == NULL) {
		return true;
	}
	begin = w.cursor;
	line = w.buffer.line(begin.line);
	end = begin;
	if (line != NULL && begin.offset < line.byte_len()) {
		end.offset = line.byte_len();
	} else if (begin.line < w.buffer.line_count_i()) {
		end.line = begin.line + 1;
		end.offset = 0;
	} else {
		return true;
	}
	text = region_text(w, begin, end, &tlen);
	if (text != NULL) {
		killring_kill(ranged(text, tlen));
	}
	buffer_undo.begin_command(w.buffer, begin);
	w.set_text(begin, end, "", 0);
	if (text != NULL) {
		buffer_undo.record_edit(UndoDelete, begin.line, begin.offset, text, tlen);
		free(text);
	}
	buffer_undo.end_command();
	return true;
}

bool cmd_yank(bool f, int n) {
	Window *w = window_current();
	char[..] yank = ranged((char *)0, 0);
	(void)f;
	if (n < 0) {
		return false;
	}
	if (!killring_read_clipboard() && !killring_in_sequence()) {
		return false;
	}
	yank = killring_yank();
	if (w == NULL || len(yank) == 0) {
		return false;
	}
	buffer_undo.begin_command(w.buffer, w.cursor);
	while (n-- > 0) {
		Location at = w.cursor;
		if (!w.set_text(at, at, ptr(yank), len(yank))) {
			break;
		}
		buffer_undo.record_edit(UndoInsert, at.line, at.offset, ptr(yank),
			len(yank));
	}
	buffer_undo.end_command();
	return true;
}

bool cmd_undo(bool f, int n) {
	Window *w = window_current();
	Buffer *undone = NULL;
	Location before = {0};
	(void)f;
	(void)n;
	if (buffer_undo.count == 0) {
		display_set_echo("No further undo");
		term_beep();
		return true;
	}
	if (!buffer_undo.undo_once(&before, &undone)) {
		display_set_echo("No further undo");
		term_beep();
		return true;
	}
	if (undone != NULL && buffer_current() != undone) {
		switch_to_buffer(undone);
		w = window_current();
	}
	if (w != NULL && w.buffer == undone) {
		w.set_cursor(before);
	}
	display_set_echo("Undo");
	return true;
}

bool cmd_write_file(bool f, int n) {
	Buffer *b = buffer_current();
	(void)f;
	(void)n;
	push_listener(ListenerAskString);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = AskWriteFile;
		}
	}
	minibuffer_begin("Write file: ", 2);
	if (b != NULL && len(b.file_name) > 0) {
		minibuffer_set_text(b.file_name);
	}
	return true;
}

bool cmd_save_buffer(bool f, int n) {
	Buffer *b = buffer_current();
	(void)f;
	(void)n;
	if (b == NULL) {
		return true;
	}
	if (len(b.file_name) == 0) {
		return cmd_write_file(false, 1);
	}
	if (file_save(b, NULL)) {
		display_set_echo("Wrote file");
	} else {
		display_set_echo("Write failed");
		term_beep();
	}
	return true;
}

bool cmd_find_file(bool f, int n) {
	(void)f;
	(void)n;
	push_listener(ListenerAskString);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = AskFindFile;
		}
	}
	minibuffer_begin("Find file: ", 2);
	return true;
}

bool cmd_read_file(bool f, int n) {
	(void)f;
	(void)n;
	push_listener(ListenerAskString);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = AskReadFile;
		}
	}
	minibuffer_begin("Read file: ", 2);
	return true;
}

static bool goto_line_number(int target) {
	Window *w = window_current();
	Buffer *buf = {0};
	Location loc = {0};
	if (w == NULL || w.buffer == NULL) {
		return false;
	}
	buf = w.buffer;
	if (target < 1 || target > buf.line_count_i()) {
		display_set_echo("Line number out of range");
		return false;
	}
	if (w.cursor.line != target || w.cursor.offset != 0) {
		markring_push(w.cursor);
	}
	loc.line = target;
	loc.offset = 0;
	w.set_cursor(loc);
	return true;
}

static bool parse_positive_line(char *s, int *out) {
	int n = 0;
	size_t i = 0;
	if (s == NULL || s[0] == 0 || out == NULL) {
		return false;
	}
	for (i = 0; s[i] != 0; i++) {
		char c = s[i];
		if (c < '0' || c > '9') {
			return false;
		}
		n = n * 10 + (c - '0');
		if (n == 0) {
			return false;
		}
	}
	*out = n;
	return true;
}

bool cmd_goto_line(bool f, int n) {
	if (f) {
		if (n <= 0) {
			display_set_echo("Line number out of range");
			return false;
		}
		return goto_line_number(n);
	}
	push_listener(ListenerAskString);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = AskGotoLine;
		}
	}
	minibuffer_begin("Goto line: ", 0);
	return true;
}

bool cmd_grep(bool f, int n) {
	(void)f;
	(void)n;
	push_listener(ListenerAskString);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = AskGrep;
		}
	}
	minibuffer_begin("grep: ", 0);
	return true;
}

bool cmd_compile(bool f, int n) {
	(void)f;
	(void)n;
	push_listener(ListenerAskString);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = AskCompile;
		}
	}
	minibuffer_begin("Compile: ", 0);
	{
		char *last = job_last_compile_command();
		if (last != NULL && last[0] != 0) {
			minibuffer_set_text(str_from_cstr(last));
		}
	}
	return true;
}

bool cmd_goto_tag(bool f, int n) {
	char sym[256] = {0};
	bool ok = false;
	(void)f;
	(void)n;
	if (tags_symbol_at_point(sym, sizeof(sym))) {
		ok = tags_goto_name(sym);
		if (ok && tags_picker_active()) {
			push_listener(ListenerTagPicker);
		}
		return ok;
	}
	push_listener(ListenerAskString);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = AskGotoTag;
		}
	}
	minibuffer_begin("Goto tag: ", 0);
	return true;
}

bool cmd_goto_buffer_tag(bool f, int n) {
	bool ok = false;
	(void)f;
	(void)n;
	ok = tags_goto_buffer();
	if (ok && tags_picker_active()) {
		push_listener(ListenerTagPicker);
	}
	return ok;
}

bool cmd_build_tags(bool f, int n) {
	(void)f;
	(void)n;
	return tags_build();
}

bool cmd_describe_symbol(bool f, int n) {
	(void)f;
	(void)n;
	return tags_describe_symbol();
}

static bool insert_tag_completion(TagEntry *entry) {
	Window *w = window_current();
	char prefix[TagNameMax] = {0};
	size_t plen = 0;
	size_t n = 0;
	Location at = {0};
	if (w == NULL || w.buffer == NULL || entry == NULL || entry.name == NULL) {
		return false;
	}
	if (w.buffer.is_readonly) {
		display_set_echo("[read-only buffer]");
		return false;
	}
	if (!tags_prefix_at_point(prefix, sizeof(prefix))) {
		display_set_echo("[completion: no prefix at point]");
		return false;
	}
	plen = strlen(prefix);
	if (strncmp(entry.name, prefix, plen) != 0) {
		return false;
	}
	n = strlen(entry.name + plen);
	if (n == 0) {
		return true;
	}
	at = w.cursor;
	buffer_undo.begin_command(w.buffer, at);
	if (!w.set_text(at, at, entry.name + plen, n)) {
		buffer_undo.end_command();
		display_set_echo("[completion: insert failed]");
		return false;
	}
	buffer_undo.record_edit(UndoInsert, at.line, at.offset,
		entry.name + plen, n);
	buffer_undo.end_command();
	display_set_echo("");
	return true;
}

bool cmd_complete_symbol(bool f, int n) {
	TagList matches = {0};
	char prefix[TagNameMax] = {0};
	bool ok = false;
	(void)f;
	(void)n;
	if (!tags_prefix_at_point(prefix, sizeof(prefix))) {
		display_set_echo("[completion: no prefix at point]");
		return false;
	}
	if (!tags_ensure_loaded(false)) {
		return false;
	}
	if (!tags_collect_completions(prefix, &matches)) {
		display_set_echo("[completion: cannot collect matches]");
		return false;
	}
	if (matches.count == 0) {
		display_set_echo("[completion: no matches]");
		tags_list_free(&matches);
		return false;
	}
	if (matches.count == 1) {
		ok = insert_tag_completion(matches.items[0]);
		tags_list_free(&matches);
		return ok;
	}
	if (!tags_picker_open(&matches, TagPickerComplete,
		"Complete: ", true)) {
		tags_list_free(&matches);
		return false;
	}
	push_listener(ListenerTagPicker);
	return true;
}

static void switch_to_buffer(Buffer *buf) {
	if (buf == NULL) {
		return;
	}
	window_switch_buffer(buf);
	tags_preload();
	display_set_echo("");
}

bool cmd_use_buffer(bool f, int n) {
	if (f && n > 0) {
		if (n <= (int)len(buffer_all) && buffer_all[n - 1] != NULL) {
			switch_to_buffer(buffer_all[n - 1]);
			return true;
		}
		display_set_echo("No such buffer");
		return false;
	}
	push_listener(ListenerUseBuffer);
	fuzzy_buffer_open();
	return true;
}

bool cmd_kill_buffer(bool f, int n) {
	Buffer *b = buffer_current();
	Buffer *replacement = NULL;
	(void)f;
	(void)n;
	if (b == NULL || b.is_scratch()) {
		display_set_echo("Can't kill *scratch*");
		return true;
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

bool cmd_split_window(bool f, int n) {
	(void)f;
	(void)n;
	window_split();
	return true;
}

bool cmd_delete_other_windows(bool f, int n) {
	(void)f;
	(void)n;
	window_only();
	return true;
}

bool cmd_delete_window(bool f, int n) {
	(void)f;
	(void)n;
	window_delete();
	return true;
}

bool cmd_other_window(bool f, int n) {
	(void)f;
	(void)n;
	window_next();
	return true;
}

bool cmd_isearch_forward(bool f, int n) {
	(void)f;
	(void)n;
	search_begin(true);
	push_listener(ListenerIsearch);
	display_set_echo("I-search: ");
	return true;
}

bool cmd_isearch_backward(bool f, int n) {
	(void)f;
	(void)n;
	search_begin(false);
	push_listener(ListenerIsearch);
	display_set_echo("I-search backward: ");
	return true;
}

bool cmd_query_replace(bool f, int n) {
	(void)f;
	(void)n;
	search_begin_replace();
	push_listener(ListenerReplace);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = 0;
		}
	}
	minibuffer_begin("Query replace: ", 0);
	return true;
}

bool cmd_toggle_theme(bool f, int n) {
	(void)f;
	(void)n;
	display_toggle_theme();
	display_set_echo("Theme toggled");
	return true;
}

bool cmd_start_macro(bool f, int n) {
	(void)f;
	(void)n;
	g_app.macro_recording = true;
	g_app.macro_len = 0;
	display_set_macro_state(true, false);
	display_set_echo("Defining keyboard macro...");
	return true;
}

bool cmd_end_macro(bool f, int n) {
	(void)f;
	(void)n;
	g_app.macro_recording = false;
	display_set_macro_state(false, g_app.macro_len > 0);
	display_set_echo("Keyboard macro defined");
	return true;
}

bool cmd_call_macro(bool f, int n) {
	int i = 0;
	(void)f;
	if (n < 1) {
		n = 1;
	}
	if (g_app.macro_len == 0) {
		return true;
	}
	g_app.macro_playing = true;
	while (n-- > 0) {
		for (i = 0; i < g_app.macro_len; i++) {
			event_enqueue(g_app.macro_tape[i]);
		}
	}
	g_app.macro_playing = false;
	return true;
}

bool cmd_universal_argument(bool f, int n) {
	(void)f;
	(void)n;
	push_listener(ListenerUniversal);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.universal_n = 4;
			L.universal_f = true;
		}
	}
	display_set_echo("C-u");
	return true;
}

bool cmd_quit(bool f, int n) {
	(void)n;
	if (!f && any_unsaved()) {
		push_listener(ListenerAskYesNo);
		minibuffer_begin("Quit with unsaved buffers? (y/n) ", 1);
		return true;
	}
	g_app.running = false;
	return true;
}

static void append_key_text(char *out, size_t cap, const char *s) {
	size_t n = 0;
	size_t sl = 0;
	if (out == NULL || s == NULL || cap == 0) {
		return;
	}
	n = strlen(out);
	sl = strlen(s);
	if (n + sl >= cap) {
		return;
	}
	memcpy(out + n, s, sl);
	out[n + sl] = 0;
}

static void format_key_chord(uint32_t code, char *out, size_t cap) {
	uint32_t base = 0;
	char ch[2] = {0};
	out[0] = 0;
	if ((code & term_key_value(TermMETA)) != 0) {
		append_key_text(out, cap, "M-");
	}
	if ((code & term_key_value(TermCTL)) != 0) {
		append_key_text(out, cap, "C-");
	}
	if ((code & term_key_value(TermSHIFT)) != 0) {
		append_key_text(out, cap, "S-");
	}
	base = code & ~term_key_value(TermKeyMask);
	if (base == term_key_value(TermKeyTab)) {
		append_key_text(out, cap, "TAB");
	} else if (base == term_key_value(TermKeyEnter)) {
		append_key_text(out, cap, "ENTER");
	} else if (base == (uint32_t)' ') {
		append_key_text(out, cap, "SPC");
	} else if (base == 0x7F) {
		append_key_text(out, cap, "BACKSPACE");
	} else if (base == term_key_value(TermKeyUp)) {
		append_key_text(out, cap, "UP");
	} else if (base == term_key_value(TermKeyDown)) {
		append_key_text(out, cap, "DOWN");
	} else if (base == term_key_value(TermKeyLeft)) {
		append_key_text(out, cap, "LEFT");
	} else if (base == term_key_value(TermKeyRight)) {
		append_key_text(out, cap, "RIGHT");
	} else if (base == term_key_value(TermKeyHome)) {
		append_key_text(out, cap, "HOME");
	} else if (base == term_key_value(TermKeyEnd)) {
		append_key_text(out, cap, "END");
	} else if (base == term_key_value(TermKeyPageUp)) {
		append_key_text(out, cap, "PGUP");
	} else if (base == term_key_value(TermKeyPageDown)) {
		append_key_text(out, cap, "PGDOWN");
	} else if (base == term_key_value(TermKeyDelete)) {
		append_key_text(out, cap, "DEL");
	} else if (base < 0x80) {
		ch[0] = (char)base;
		if (ch[0] >= 'A' && ch[0] <= 'Z') {
			ch[0] = (char)(ch[0] - 'A' + 'a');
		}
		append_key_text(out, cap, ch);
	}
}

static void format_key_label(uint32_t code, char *out, size_t cap) {
	if ((code & term_key_value(TermCTLX)) != 0) {
		out[0] = 0;
		append_key_text(out, cap, "C-x ");
		format_key_chord(code & ~term_key_value(TermCTLX), out + strlen(out), cap - strlen(out));
		return;
	}
	format_key_chord(code, out, cap);
}

static bool key_is_mouse(uint32_t code) {
	uint32_t base = code & ~term_key_value(TermKeyMask);
	return base >= term_key_value(TermMouseLeft) && base <= term_key_value(TermMouseDrag);
}

static void command_bind_label(CommandFn fn, char *out, size_t cap) {
	int i = 0;
	out[0] = 0;
	if (fn == NULL) {
		return;
	}
	for (i = 0; i < g_bind_count; i++) {
		if (g_binds[i].fn != fn || key_is_mouse(g_binds[i].key)) {
			continue;
		}
		format_key_label(g_binds[i].key, out, cap);
		return;
	}
}

bool cmd_execute_extended(bool f, int n) {
	const char *names[128] = {0};
	char binds_store[128][32] = {0};
	const char *binds[128] = {0};
	int i = 0;
	int ncmd = g_cmd_count;
	(void)f;
	(void)n;
	if (ncmd <= 0) {
		display_set_echo("No commands");
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
	push_listener(ListenerMx);
	fuzzy_command_open(names, binds, ncmd, NULL);
	return true;
}

static void init_commands() {
	reg_cmd("forward-char", cmd_forward_char);
	reg_cmd("backward-char", cmd_backward_char);
	reg_cmd("next-line", cmd_next_line);
	reg_cmd("previous-line", cmd_prev_line);
	reg_cmd("forward-word", cmd_forward_word);
	reg_cmd("backward-word", cmd_backward_word);
	reg_cmd("scroll-up", cmd_forward_page);
	reg_cmd("scroll-down", cmd_backward_page);
	reg_cmd("beginning-of-line", cmd_beginning_of_line);
	reg_cmd("end-of-line", cmd_end_of_line);
	reg_cmd("beginning-of-buffer", cmd_goto_bof);
	reg_cmd("end-of-buffer", cmd_goto_eof);
	reg_cmd("back-to-indentation", cmd_back_to_indentation);
	reg_cmd("newline", cmd_newline);
	reg_cmd("newline-and-indent", cmd_newline_and_indent);
	reg_cmd("indent-line", cmd_indent_line);
	reg_cmd("close-brace", cmd_close_brace);
	reg_cmd("open-line", cmd_open_line);
	reg_cmd("delete-backward-char", cmd_delete_backward);
	reg_cmd("delete-char", cmd_delete_forward);
	reg_cmd("delete-word-forward", cmd_delete_word_forward);
	reg_cmd("delete-word-backward", cmd_delete_word_backward);
	reg_cmd("set-mark-command", cmd_set_mark);
	reg_cmd("exchange-point-and-mark", cmd_exchange_point_and_mark);
	reg_cmd("mark-whole-buffer", cmd_mark_whole_buffer);
	reg_cmd("kill-region", cmd_kill_region);
	reg_cmd("copy-region-as-kill", cmd_copy_region);
	reg_cmd("kill-line", cmd_kill_line);
	reg_cmd("yank", cmd_yank);
	reg_cmd("undo", cmd_undo);
	reg_cmd("transpose-chars", cmd_transpose_chars);
	reg_cmd("transpose-words", cmd_transpose_words);
	reg_cmd("downcase-word", cmd_lower_word);
	reg_cmd("upcase-word", cmd_upper_word);
	reg_cmd("capitalize-word", cmd_cap_word);
	reg_cmd("trim-whitespace", cmd_trim_whitespace);
	reg_cmd("recenter", cmd_refresh);
	reg_cmd("save-buffer", cmd_save_buffer);
	reg_cmd("find-file", cmd_find_file);
	reg_cmd("write-file", cmd_write_file);
	reg_cmd("read-file", cmd_read_file);
	reg_cmd("goto-line", cmd_goto_line);
	reg_cmd("grep", cmd_grep);
	reg_cmd("compile", cmd_compile);
	reg_cmd("goto-tag", cmd_goto_tag);
	reg_cmd("goto-buffer-tag", cmd_goto_buffer_tag);
	reg_cmd("build-tags", cmd_build_tags);
	reg_cmd("complete-symbol", cmd_complete_symbol);
	reg_cmd("describe-symbol", cmd_describe_symbol);
	reg_cmd("switch-to-buffer", cmd_use_buffer);
	reg_cmd("kill-buffer", cmd_kill_buffer);
	reg_cmd("split-window", cmd_split_window);
	reg_cmd("delete-other-windows", cmd_delete_other_windows);
	reg_cmd("delete-window", cmd_delete_window);
	reg_cmd("other-window", cmd_other_window);
	reg_cmd("isearch-forward", cmd_isearch_forward);
	reg_cmd("isearch-backward", cmd_isearch_backward);
	reg_cmd("query-replace", cmd_query_replace);
	reg_cmd("toggle-theme", cmd_toggle_theme);
	reg_cmd("start-kbd-macro", cmd_start_macro);
	reg_cmd("end-kbd-macro", cmd_end_macro);
	reg_cmd("call-last-kbd-macro", cmd_call_macro);
	reg_cmd("universal-argument", cmd_universal_argument);
	reg_cmd("save-buffers-kill-terminal", cmd_quit);
	reg_cmd("execute-extended-command", cmd_execute_extended);

	bind_key(term_key_value(TermKeyRight), cmd_forward_char);
	bind_key(term_key_value(TermKeyLeft), cmd_backward_char);
	bind_key(term_key_value(TermKeyDown), cmd_next_line);
	bind_key(term_key_value(TermKeyUp), cmd_prev_line);
	bind_key(term_key_value(TermSHIFT) | term_key_value(TermKeyRight), cmd_forward_word);
	bind_key(term_key_value(TermSHIFT) | term_key_value(TermKeyLeft), cmd_backward_word);
	bind_key(term_key_value(TermSHIFT) | term_key_value(TermKeyDown), cmd_forward_page);
	bind_key(term_key_value(TermSHIFT) | term_key_value(TermKeyUp), cmd_backward_page);
	bind_key(term_key_value(TermMETA) | (uint32_t)'F', cmd_forward_word);
	bind_key(term_key_value(TermMETA) | (uint32_t)'B', cmd_backward_word);
	bind_key(term_key_value(TermKeyPageDown), cmd_forward_page);
	bind_key(term_key_value(TermKeyPageUp), cmd_backward_page);
	bind_key(term_key_value(TermCTL) | (uint32_t)'V', cmd_forward_page);
	bind_key(term_key_value(TermMETA) | (uint32_t)'V', cmd_backward_page);
	bind_key(term_key_value(TermCTL) | (uint32_t)'F', cmd_forward_char);
	bind_key(term_key_value(TermCTL) | (uint32_t)'B', cmd_backward_char);
	bind_key(term_key_value(TermCTL) | (uint32_t)'N', cmd_next_line);
	bind_key(term_key_value(TermCTL) | (uint32_t)'P', cmd_prev_line);
	bind_key(term_key_value(TermCTL) | (uint32_t)'A', cmd_beginning_of_line);
	bind_key(term_key_value(TermCTL) | (uint32_t)'E', cmd_end_of_line);
	bind_key(term_key_value(TermKeyHome), cmd_beginning_of_line);
	bind_key(term_key_value(TermKeyEnd), cmd_end_of_line);
	bind_key(term_key_value(TermMETA) | (uint32_t)'<', cmd_goto_bof);
	bind_key(term_key_value(TermMETA) | (uint32_t)'>', cmd_goto_eof);
	bind_key(term_key_value(TermMETA) | (uint32_t)'M', cmd_back_to_indentation);
	bind_key(term_key_value(TermKeyEnter), cmd_newline_and_indent);
	bind_key('\r', cmd_newline_and_indent);
	bind_key('\n', cmd_newline_and_indent);
	bind_key(term_key_value(TermCTL) | (uint32_t)'J', cmd_newline_and_indent);
	bind_key(term_key_value(TermKeyTab), cmd_indent_line);
	bind_key('}', cmd_close_brace);
	bind_key(term_key_value(TermCTL) | (uint32_t)'O', cmd_open_line);
	bind_key(term_key_value(TermCTL) | (uint32_t)'H', cmd_delete_backward);
	bind_key(127, cmd_delete_backward);
	bind_key(term_key_value(TermKeyDelete), cmd_delete_forward);
	bind_key(term_key_value(TermCTL) | (uint32_t)'D', cmd_delete_forward);
	bind_key(term_key_value(TermMETA) | (uint32_t)'D', cmd_delete_word_forward);
	bind_key(term_key_value(TermMETA) | (uint32_t)'H', cmd_delete_word_backward);
	bind_key(term_key_value(TermMETA) | 0x7F, cmd_delete_word_backward);
	bind_key(term_key_value(TermCTL) | (uint32_t)' ', cmd_set_mark);
	bind_key(term_key_value(TermCTL) | (uint32_t)'W', cmd_kill_region);
	bind_key(term_key_value(TermMETA) | (uint32_t)'W', cmd_copy_region);
	bind_key(term_key_value(TermCTL) | (uint32_t)'K', cmd_kill_line);
	bind_key(term_key_value(TermCTL) | (uint32_t)'Y', cmd_yank);
	bind_key(term_key_value(TermCTL) | (uint32_t)'Z', cmd_undo);
	bind_key(term_key_value(TermCTL) | (uint32_t)'T', cmd_transpose_chars);
	bind_key(term_key_value(TermMETA) | (uint32_t)'T', cmd_transpose_words);
	bind_key(term_key_value(TermMETA) | (uint32_t)'L', cmd_lower_word);
	bind_key(term_key_value(TermMETA) | (uint32_t)'U', cmd_upper_word);
	bind_key(term_key_value(TermMETA) | (uint32_t)'C', cmd_cap_word);
	bind_key(term_key_value(TermMETA) | (uint32_t)'\\', cmd_trim_whitespace);
	bind_key(term_key_value(TermCTL) | (uint32_t)'L', cmd_refresh);
	bind_key(term_key_value(TermCTL) | (uint32_t)'S', cmd_isearch_forward);
	bind_key(term_key_value(TermCTL) | (uint32_t)'R', cmd_isearch_backward);
	bind_key(term_key_value(TermCTL) | (uint32_t)'U', cmd_universal_argument);
	bind_key(term_key_value(TermMETA) | (uint32_t)'X', cmd_execute_extended);
	bind_key(term_key_value(TermMETA) | (uint32_t)'%', cmd_query_replace);
	bind_key(term_key_value(TermMETA) | (uint32_t)'G', cmd_goto_line);
	bind_key(term_key_value(TermMETA) | (uint32_t)'.', cmd_goto_tag);
	bind_key(term_key_value(TermSHIFT) | term_key_value(TermKeyTab), cmd_complete_symbol);
	bind_key(term_key_value(TermMETA) | (uint32_t)'/', cmd_complete_symbol);
	bind_key(term_key_value(TermMETA) | (uint32_t)'?', cmd_describe_symbol);
}

static bool execute(CommandFn fn, bool f, int n) {
	uint64_t echo_before = 0;
	bool ok = true;
	if (fn == NULL) {
		return true;
	}
	echo_before = display_echo_generation();
	ok = fn(f, n);
	killring_tick();
	if (g_app.listener_top == 0 && !g_app.macro_recording &&
		!g_app.macro_playing && display_echo_generation() == echo_before) {
		tags_maybe_show_call_hint();
	}
	return ok;
}

static bool handle_listener_key(uint32_t key) {
	Listener *L = top_listener();
	if (L == NULL) {
		return false;
	}
	killring_tick();
	if (L.kind == ListenerUniversal) {
		if (key >= '0' && key <= '9') {
			L.universal_n = key - '0';
			display_set_echo("C-u digit");
			return true;
		}
		{
			bool f = L.universal_f;
			int n = L.universal_n;
			CommandFn fn = {0};
			pop_listener();
			fn = find_bind(key);
			if (fn != NULL) {
				return execute(fn, f, n);
			}
			if (key >= 32 && key < 0x110000 && (key & term_key_value(TermKeyMask)) == 0 && key < 0x10000000) {
				g_pending_insert = key;
				return execute(cmd_insert_pending, f, n);
			}
			return true;
		}
	}
	if (L.kind == ListenerIsearch) {
		if (key == (term_key_value(TermCTL) | (uint32_t)'G') || key == 27) {
			search_end();
			pop_listener();
			display_set_echo("");
			return true;
		}
		if (key == (term_key_value(TermCTL) | (uint32_t)'S')) {
			search_next();
			return true;
		}
		if (key == (term_key_value(TermCTL) | (uint32_t)'R')) {
			g_search_force_backward();
			search_next();
			return true;
		}
		if (key == (term_key_value(TermCTL) | (uint32_t)'H') || key == 127) {
			search_backspace();
			search_update();
			{
				char msg[300] = {0};
				snprintf(msg, sizeof(msg), "I-search: %s", search_needle());
				display_set_echo(msg);
			}
			return true;
		}
		if (key == term_key_value(TermKeyEnter)) {
			search_end();
			pop_listener();
			display_set_echo("");
			return true;
		}
		if (key >= 32 && key < 127) {
			search_append_char(key);
			search_update();
			{
				char msg[300] = {0};
				snprintf(msg, sizeof(msg), "I-search: %s", search_needle());
				display_set_echo(msg);
			}
			return true;
		}
		return true;
	}
	if (L.kind == ListenerTagPicker) {
		FuzzyKeyResult fr = tags_picker_handle_key(key);
		if (fr == FuzzyContinue) {
			return true;
		}
		if (fr == FuzzyAccept) {
			TagEntry *entry = tags_picker_selected();
			int purpose = tags_picker_purpose();
			tags_picker_close();
			pop_listener();
			if (entry == NULL) {
				display_set_echo("[no tag selected]");
				return true;
			}
			if (purpose == (int)TagPickerComplete) {
				insert_tag_completion(entry);
			} else {
				tags_visit_entry(entry);
			}
			return true;
		}
		tags_picker_close();
		pop_listener();
		display_set_echo("Quit");
		return true;
	}
	if (L.kind == ListenerUseBuffer) {
		FuzzyKeyResult fr = fuzzy_buffer_handle_key(key);
		if (fr == FuzzyContinue) {
			return true;
		}
		if (fr == FuzzyAccept) {
			Buffer *buf = fuzzy_buffer_selected();
			fuzzy_buffer_close();
			pop_listener();
			if (buf == NULL) {
				display_set_echo("No such buffer");
				return true;
			}
			switch_to_buffer(buf);
			return true;
		}
		/* FuzzyAbort */
		fuzzy_buffer_close();
		pop_listener();
		display_set_echo("Quit");
		return true;
	}
	if (L.kind == ListenerMx) {
		FuzzyKeyResult fr = fuzzy_command_handle_key(key);
		char picked[64] = {0};
		if (fr == FuzzyContinue) {
			return true;
		}
		if (fr == FuzzyAccept) {
			char *sel = fuzzy_command_selected();
			CommandFn fn = {0};
			if (sel != NULL) {
				snprintf(picked, sizeof(picked), "%s", sel);
			}
			fuzzy_command_remember(picked);
			fuzzy_command_close();
			pop_listener();
			fn = find_cmd(picked);
			if (fn == NULL) {
				display_set_echo("Unknown command");
				return true;
			}
			return execute(fn, false, 1);
		}
		fuzzy_command_close();
		pop_listener();
		display_set_echo("Quit");
		return true;
	}
	if (L.kind == ListenerAskString || L.kind == ListenerAskYesNo || L.kind == ListenerReplace) {
		if (key == 27 || key == (term_key_value(TermCTL) | (uint32_t)'G')) {
			minibuffer_cancel();
			if (L.kind == ListenerReplace) {
				search_end();
			}
			pop_listener();
			display_set_echo("Quit");
			return true;
		}
		if (L.kind == ListenerAskYesNo) {
			if (key == 'y' || key == 'Y') {
				minibuffer_cancel();
				pop_listener();
				g_app.running = false;
				return true;
			}
			if (key == 'n' || key == 'N') {
				minibuffer_cancel();
				pop_listener();
				display_set_echo("");
				return true;
			}
			return true;
		}
		if (key == term_key_value(TermKeyEnter)) {
			char text_copy[1024] = {0};
			char *raw = minibuffer_text();
			ListenerKind kind = L.kind;
			int phase = L.ask_phase;
			if (raw != NULL) {
				snprintf(text_copy, sizeof(text_copy), "%s", raw);
			}
			minibuffer_cancel();
			pop_listener();
			if (kind == ListenerAskString) {
				Window *w = window_current();
				if (phase == (int)AskFindFile) {
					auto (buf, ok) = file_visit(text_copy);
					if (ok && buf != NULL) {
						window_switch_buffer(buf);
						tags_preload();
						display_set_echo("File visited");
					} else {
						display_set_echo("Visit failed");
					}
					return true;
				}
				if (phase == (int)AskWriteFile) {
					Buffer *b = buffer_current();
					if (text_copy[0] == 0 || b == NULL) {
						display_set_echo("Write failed");
						return true;
					}
					if (file_save(b, text_copy)) {
						if (w != NULL) {
							w.should_redraw = true;
							w.should_update_mode_line = true;
						}
						display_set_echo("Wrote file");
					} else {
						display_set_echo("Write failed");
						term_beep();
					}
					return true;
				}
				if (phase == (int)AskReadFile) {
					if (text_copy[0] == 0) {
						display_set_echo("Read failed");
						return true;
					}
					if (file_read_into_current(text_copy)) {
						if (w != NULL) {
							w.buffer = buffer_current();
							w.cursor = w.buffer.cursor;
							w.top_line = 1;
							w.should_redraw = true;
							w.should_update_mode_line = true;
						}
					} else {
						display_set_echo("Read failed");
						term_beep();
					}
					return true;
				}
				if (phase == (int)AskGrep) {
					if (text_copy[0] == 0) {
						display_set_echo("Empty pattern");
						return true;
					}
					job_start_grep(text_copy);
					return true;
				}
				if (phase == (int)AskCompile) {
					if (text_copy[0] == 0) {
						display_set_echo("Empty command");
						return true;
					}
					job_start_compile(text_copy);
					return true;
				}
				if (phase == (int)AskGotoTag) {
					if (text_copy[0] == 0) {
						display_set_echo("Empty tag name");
						return true;
					}
					tags_goto_name(text_copy);
					if (tags_picker_active()) {
						push_listener(ListenerTagPicker);
					}
					return true;
				}
				if (phase == (int)AskGotoLine) {
					int line = 0;
					if (!parse_positive_line(text_copy, &line)) {
						display_set_echo("Invalid line number");
						return true;
					}
					goto_line_number(line);
					return true;
				}
				return true;
			}
			if (kind == ListenerReplace) {
				if (phase == 0) {
					search_set_needle(text_copy);
					push_listener(ListenerReplace);
					{
						Listener *N = top_listener();
						if (N != NULL) {
							N.ask_phase = 1;
						}
					}
					minibuffer_begin("With: ", 0);
					return true;
				}
				if (phase == 1) {
					search_set_replacement(text_copy);
					search_update();
					push_listener(ListenerReplace);
					{
						Listener *N = top_listener();
						if (N != NULL) {
							N.ask_phase = 2;
						}
					}
					display_set_echo("Replace? (y/n/!)");
					return true;
				}
			}
			return true;
		}
		if (L.kind == ListenerReplace && L.ask_phase == 2) {
			if (key == 'y' || key == 'Y') {
				search_replace_one();
				display_set_echo("Replace? (y/n/!)");
				return true;
			}
			if (key == 'n' || key == 'N') {
				search_next();
				display_set_echo("Replace? (y/n/!)");
				return true;
			}
			if (key == '!') {
				while (search_replace_one()) {
				}
				pop_listener();
				search_end();
				display_set_echo("Replaced");
				return true;
			}
			if (key == 27 || key == (term_key_value(TermCTL) | (uint32_t)'G')) {
				pop_listener();
				search_end();
				display_set_echo("");
				return true;
			}
			return true;
		}
		if (key == (term_key_value(TermCTL) | (uint32_t)'H') || key == 127) {
			minibuffer_backspace();
			return true;
		}
		if (key >= 32 && (key & term_key_value(TermKeyMask)) == 0 && key < 0x10000000) {
			minibuffer_insert(key);
			return true;
		}
		return true;
	}
	if (L.kind == ListenerCtlx) {
		uint32_t combined = term_key_value(TermCTLX) | key;
		CommandFn fn = {0};
		bool f = L.universal_f;
		int n = L.universal_n;
		pop_listener();
		if (n < 1) {
			n = 1;
		}
		if (key == (term_key_value(TermCTL) | (uint32_t)'C') || key == (uint32_t)'c') {
			return execute(cmd_quit, f, n);
		}
		if (key == (term_key_value(TermCTL) | (uint32_t)'S') || key == (uint32_t)'s') {
			return execute(cmd_save_buffer, f, n);
		}
		if (key == (term_key_value(TermCTL) | (uint32_t)'F') || key == (uint32_t)'f') {
			return execute(cmd_find_file, f, n);
		}
		if (key == (term_key_value(TermCTL) | (uint32_t)'W') || key == (uint32_t)'w' || key == (uint32_t)'W') {
			return execute(cmd_write_file, f, n);
		}
		if (key == (term_key_value(TermCTL) | (uint32_t)'R') || key == (uint32_t)'r') {
			return execute(cmd_read_file, f, n);
		}
		if (key == (term_key_value(TermCTL) | (uint32_t)'K') || key == (uint32_t)'k') {
			return execute(cmd_kill_buffer, f, n);
		}
		if (key == (uint32_t)'b' || key == (uint32_t)'B' || key == (term_key_value(TermCTL) | (uint32_t)'B')) {
			return execute(cmd_use_buffer, f, n);
		}
		if (key == (uint32_t)'2') {
			return execute(cmd_split_window, f, n);
		}
		if (key == (uint32_t)'1') {
			return execute(cmd_delete_other_windows, f, n);
		}
		if (key == (uint32_t)'0') {
			return execute(cmd_delete_window, f, n);
		}
		if (key == (uint32_t)'o') {
			return execute(cmd_other_window, f, n);
		}
		if (key == (uint32_t)'(') {
			return execute(cmd_start_macro, f, n);
		}
		if (key == (uint32_t)')') {
			return execute(cmd_end_macro, f, n);
		}
		if (key == (uint32_t)'e') {
			return execute(cmd_call_macro, f, n);
		}
		if (key == (uint32_t)'h' || key == (uint32_t)'H') {
			return execute(cmd_mark_whole_buffer, f, n);
		}
		if (key == (uint32_t)'t' || key == (uint32_t)'T') {
			return execute(cmd_toggle_theme, f, n);
		}
		(void)combined;
		(void)fn;
		display_set_echo("C-x unknown");
		return true;
	}
	return false;
}

/* stub used above — set search direction */
void g_search_force_backward() {
	SearchState *s = search_state();
	if (s != NULL) {
		s.forward = false;
	}
}

static bool handle_key(uint32_t key) {
	CommandFn fn = {0};
	/* A live listener always consumes the key (jem.go). Do not fall through to
	 * self-insert after prompts/C-x — that corrupted quit-with-unsaved (y). */
	if (top_listener() != NULL) {
		(void)handle_listener_key(key);
		return g_app.running;
	}
	if (key == (term_key_value(TermCTL) | (uint32_t)'X')) {
		push_listener(ListenerCtlx);
		display_set_echo("C-x-");
		return true;
	}
	if (key == (term_key_value(TermCTL) | (uint32_t)'G')) {
		if (job_running()) {
			job_cancel();
			return true;
		}
		display_set_echo("Quit");
		return true;
	}
	fn = find_bind(key);
	if (fn != NULL) {
		return execute(fn, false, 1);
	}
	/* self-insert printable */
	if (key >= 32 && (key & term_key_value(TermKeyMask)) == 0 && key < 0x10000000) {
		g_pending_insert = key;
		return execute(cmd_insert_pending, false, 1);
	}
	if (key == term_key_value(TermMouseWheelUp)) {
		return execute(cmd_prev_line, false, 3);
	}
	if (key == term_key_value(TermMouseWheelDown)) {
		return execute(cmd_next_line, false, 3);
	}
	if (key == term_key_value(TermMouseLeft)) {
		Window *w = window_current();
		int row = term_mouse_row();
		int col = term_mouse_col();
		if (w != NULL && w.buffer != NULL && row >= w.screen_top_row && row < w.screen_top_row + w.height) {
			int nlines = w.buffer.line_count_i();
			int pad = window_content_row_offset(w);
			int gutter = window_gutter_width(w);
			int row_in = row - w.screen_top_row - pad;
			int bline = 0;
			int text_col = 0;
			Line *line = {0};
			Location loc = {0};
			if (row_in < 0) {
				row_in = 0;
			}
			bline = w.top_line + row_in;
			if (bline < 1) {
				bline = 1;
			}
			if (nlines < 1) {
				nlines = 1;
			}
			if (bline > nlines) {
				bline = nlines;
			}
			text_col = col - gutter + w.h_scroll;
			if (text_col < 0) {
				text_col = 0;
			}
			line = w.buffer.line(bline);
			loc.line = bline;
			loc.offset = line != NULL ? line.offset_at_display_col(text_col) : 0;
			w.set_cursor(loc);
			w.mark.line = 0;
			w.mark.offset = 0;
			w.should_redraw = true;
			w.should_update_mode_line = true;
		}
		return true;
	}
	return true;
}

static bool handle_event(Event *e) {
	if (e == NULL) {
		return true;
	}
	switch (e.kind) {
	case EventKey:
		if (g_app.macro_recording && !g_app.macro_playing && g_app.macro_len < 256) {
			g_app.macro_tape[g_app.macro_len++] = *e;
		}
		return handle_key(e.key_code);
	case EventResize:
		term_refresh_size();
		window_retile();
		return true;
	case EventQuit:
		return execute(cmd_quit, e.quit_force, 1);
	case EventPaste:
		if (e.paste_data != NULL && e.paste_len > 0) {
			if (minibuffer_active()) {
				(void)minibuffer_insert_paste(
					ranged(e.paste_data, e.paste_len, e.paste_len));
			} else {
				Window *w = window_current();
				if (w != NULL && w.buffer != NULL) {
					Location at = w.cursor;
					buffer_undo.begin_command(w.buffer, at);
					if (w.set_text(at, at, e.paste_data, e.paste_len)) {
						buffer_undo.record_edit(UndoInsert, at.line, at.offset,
							e.paste_data, e.paste_len);
					}
					buffer_undo.end_command();
				}
			}
		}
		killring_tick();
		return true;
	case EventMouse:
		return true;
	default:
		return true;
	}
}

static void config_load() {
	char path[512] = {0};
	FILE *fp = {0};
	char *home = getenv("HOME");
	char line[256] = {0};
	if (home == NULL) {
		return;
	}
	snprintf(path, sizeof(path), "%s/.jem.json", home);
	fp = fopen(path, "r");
	if (fp == NULL) {
		return;
	}
	/* minimal: look for "theme": "light" */
	while (fgets(line, sizeof(line), fp) != NULL) {
		if (strstr(line, "\"theme\"") != NULL && strstr(line, "light") != NULL) {
			display_toggle_theme();
		}
	}
	fclose(fp);
}

// runtime_run is the process entry after main.
void runtime_run(int argc, char **argv) {
	int i = 0;
	memset(&g_app, 0, sizeof(g_app));
	g_app.running = true;
	g_app.auto_indent = true;

	buffer_list_init();
	window_init();
	init_commands();
	job_init();

	if (!term_open()) {
		os_exit(1);
	}
	display_init();
	config_load();

	/* Load argv files; leave *scratch* alone; show the first path (jem.go). */
	{
		Buffer *first = NULL;
		for (i = 1; i < argc; i++) {
			auto (buf, ok) = file_visit(argv[i]);
			if (!ok || buf == NULL) {
				continue;
			}
			if (first == NULL) {
				first = buf;
			}
		}
		if (first != NULL && window_current() != NULL) {
			window_switch_buffer(first);
			tags_preload();
		}
	}
	window_retile();
	g_escape_pending = false;

	while (g_app.running) {
		job_poll();
		if (job_take_tags_changed()) {
			tags_invalidate();
			tags_preload();
		}
		display_update();
		/* drain queued events */
		{
			Event e = {EventNone};
			while (event_try_dequeue(&e)) {
				if (!handle_event(&e)) {
					g_app.running = false;
				}
				event_free(&e);
			}
		}
		if (!g_app.running) {
			break;
		}
		{
			auto (key, ok) = term_try_read_key(100);
			uint32_t editor_key = 0;
			if (ok && key != term_key_value(TermKeyPasteComplete) &&
				prepare_editor_key(key, &editor_key)) {
				event_enqueue(event_key(editor_key));
			}
		}
	}
	term_close();
	display_shutdown();
}

// runtime/sexp.mc — balanced-expression movement (jem.go/runtime/sexp.go).
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>

static bool cursor_at_eob(Window *win) {
	if (win == NULL || win.buffer == NULL) {
		return true;
	}
	return win.cursor.line >= win.buffer.eof_line();
}

static int cursor_char(Window *win, Buffer *buf) {
	Location loc = {0};
	Line *line = NULL;
	if (cursor_at_eob(win)) {
		return -1;
	}
	loc = win.cursor;
	line = buf.line(loc.line);
	if (line == NULL) {
		return -1;
	}
	if (loc.offset >= line.byte_len()) {
		return '\n';
	}
	{
		char[..] v = line.view();
		return ptr(v)[loc.offset];
	}
}

static bool forward_sexp_once(Window *win, Buffer *buf) {
	Location loc = {0};
	Location match = {0};
	int ch = 0;
	for (;;) {
		ch = cursor_char(win, buf);
		if (ch < 0) {
			return false;
		}
		if (ch != ' ' && ch != '\t' && ch != '\n') {
			break;
		}
		if (!cmd_forward_char(false, 1)) {
			return false;
		}
	}
	loc = win.cursor;
	ch = cursor_char(win, buf);
	if (ch == '(' || ch == '[' || ch == '{') {
		Line *mlp = NULL;
		size_t after = 0;
		if (!mode_find_matching_delimiter(buf, loc, &match)) {
			display_set_echo("[no matching delimiter]");
			return false;
		}
		mlp = buf.line(match.line);
		after = match.offset + 1;
		if (mlp == NULL || after > mlp.byte_len()) {
			Location next = {0};
			next.line = match.line + 1;
			next.offset = 0;
			win.set_cursor(next);
		} else {
			Location next = {0};
			next.line = match.line;
			next.offset = after;
			win.set_cursor(next);
		}
		win.did_move = true;
		return true;
	}
	return cmd_forward_word(false, 1);
}

static bool backward_sexp_once(Window *win, Buffer *buf) {
	Location orig = {0};
	Location loc = {0};
	Location match = {0};
	int ch = 0;
	orig = win.cursor;
	if (!cmd_backward_char(false, 1)) {
		return false;
	}
	for (;;) {
		ch = cursor_char(win, buf);
		if (ch < 0 || (ch != ' ' && ch != '\t' && ch != '\n')) {
			break;
		}
		if (!cmd_backward_char(false, 1)) {
			break;
		}
	}
	loc = win.cursor;
	ch = cursor_char(win, buf);
	if (ch == ')' || ch == ']' || ch == '}') {
		if (!mode_find_matching_delimiter(buf, loc, &match)) {
			display_set_echo("[no matching delimiter]");
			win.set_cursor(orig);
			return false;
		}
		win.set_cursor(match);
		win.did_move = true;
		return true;
	}
	win.set_cursor(orig);
	return cmd_backward_word(false, 1);
}

static bool move_sexp(bool forward, int n) {
	Window *win = window_current();
	Buffer *buf = buffer_current();
	int i = 0;
	if (n < 0) {
		forward = !forward;
		n = -n;
	}
	if (win == NULL || buf == NULL || n == 0) {
		return true;
	}
	for (i = 0; i < n; i++) {
		if (forward) {
			if (!forward_sexp_once(win, buf)) {
				return false;
			}
		} else {
			if (!backward_sexp_once(win, buf)) {
				return false;
			}
		}
	}
	return true;
}

// cmd_forward_sexp moves past the balanced expression at/after point.
bool cmd_forward_sexp(bool f, int n) {
	(void)f;
	return move_sexp(true, n);
}

// cmd_backward_sexp moves back past the balanced expression before point.
bool cmd_backward_sexp(bool f, int n) {
	(void)f;
	return move_sexp(false, n);
}

// mode/match.mc — delimiter matching and mode-goto-match (jem.go/syntax/nav + mode_cmds).
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

static char g_mode_echo[128];

void mode_msg(const char *msg) {
	if (msg == NULL) {
		g_mode_echo[0] = 0;
		return;
	}
	snprintf(g_mode_echo, sizeof(g_mode_echo), "%s", msg);
}

// mode_consume_echo copies and clears the last mode message. Returns true if set.
bool mode_consume_echo(char *out, size_t cap) {
	if (g_mode_echo[0] == 0) {
		return false;
	}
	if (out != NULL && cap > 0) {
		snprintf(out, cap, "%s", g_mode_echo);
	}
	g_mode_echo[0] = 0;
	return true;
}

enum MatchResult {
	MatchNone = 0,
	MatchFound = 1,
	MatchUnbalanced = 2
};

// mode_delimiter_pair maps ch to open/close and scan direction. Returns false if not a delim.
bool mode_delimiter_pair(int ch, int *open_out, int *close_out, bool *forward_out) {
	if (ch == '(' || ch == ')') {
		*open_out = '(';
		*close_out = ')';
		*forward_out = (ch == '(');
		return true;
	}
	if (ch == '[' || ch == ']') {
		*open_out = '[';
		*close_out = ']';
		*forward_out = (ch == '[');
		return true;
	}
	if (ch == '{' || ch == '}') {
		*open_out = '{';
		*close_out = '}';
		*forward_out = (ch == '{');
		return true;
	}
	return false;
}

/* Approximate code-context check for one line (ignores cross-line block comments). */
static bool offset_in_code(Line *line, size_t offset) {
	char[..] v = {0};
	const char *p = NULL;
	size_t n = 0;
	size_t i = 0;
	bool in_str = false;
	bool in_chr = false;
	bool in_line_cmt = false;
	bool in_block = false;
	char str_q = 0;
	if (line == NULL) {
		return false;
	}
	v = line.view();
	p = ptr(v);
	n = len(v);
	if (offset >= n) {
		return false;
	}
	for (i = 0; i < offset; i++) {
		char c = p[i];
		char next = 0;
		if (i + 1 < n) {
			next = p[i + 1];
		}
		if (in_line_cmt) {
			continue;
		}
		if (in_block) {
			if (c == '*' && next == '/') {
				in_block = false;
				i++;
			}
			continue;
		}
		if (in_str || in_chr) {
			if (c == '\\' && i + 1 < n) {
				i++;
				continue;
			}
			if (c == str_q) {
				in_str = false;
				in_chr = false;
			}
			continue;
		}
		if (c == '/' && next == '/') {
			in_line_cmt = true;
			i++;
			continue;
		}
		if (c == '/' && next == '*') {
			in_block = true;
			i++;
			continue;
		}
		if (c == '"') {
			in_str = true;
			str_q = '"';
			continue;
		}
		if (c == '\'') {
			in_chr = true;
			str_q = '\'';
			continue;
		}
	}
	return !in_str && !in_chr && !in_line_cmt && !in_block;
}

bool mode_char_is_structural(Buffer *buf, int line_number, size_t offset) {
	Line *line = NULL;
	(void)buf;
	if (buf == NULL || line_number < 1) {
		return false;
	}
	line = buf.line(line_number);
	if (line == NULL) {
		return false;
	}
	return offset_in_code(line, offset);
}

static int line_byte_at(Line *line, size_t offset) {
	char[..] v = line.view();
	if (offset >= len(v)) {
		return -1;
	}
	return ptr(v)[offset];
}

// mode_find_matching_delimiter finds the partner for start (jem.go FindMatchingDelimiter).
bool mode_find_matching_delimiter(Buffer *buf, Location start, Location *match_out) {
	Line *line = NULL;
	int ch = 0;
	int open_ch = 0;
	int close_ch = 0;
	bool forward = false;
	int depth = 1;
	if (buf == NULL || match_out == NULL) {
		return false;
	}
	if (start.line < 1 || start.line >= buf.eof_line()) {
		return false;
	}
	line = buf.line(start.line);
	if (line == NULL || start.offset >= line.byte_len()) {
		return false;
	}
	ch = line_byte_at(line, start.offset);
	if (!mode_delimiter_pair(ch, &open_ch, &close_ch, &forward)) {
		return false;
	}
	if (!mode_char_is_structural(buf, start.line, start.offset)) {
		return false;
	}
	if (forward) {
		int line_num = start.line;
		size_t off = start.offset + 1;
		int nlines = buf.line_count_i();
		while (line_num <= nlines) {
			line = buf.line(line_num);
			if (line == NULL) {
				return false;
			}
			while (off < line.byte_len()) {
				int c = line_byte_at(line, off);
				if ((c == open_ch || c == close_ch) &&
					mode_char_is_structural(buf, line_num, off)) {
					if (c == open_ch) {
						depth++;
					} else {
						depth--;
						if (depth == 0) {
							match_out.line = line_num;
							match_out.offset = off;
							return true;
						}
					}
				}
				off++;
			}
			line_num++;
			off = 0;
		}
	} else {
		int line_num = start.line;
		int off = (int)start.offset - 1;
		while (line_num >= 1) {
			line = buf.line(line_num);
			if (line == NULL) {
				return false;
			}
			while (off >= 0) {
				int c = line_byte_at(line, off);
				if ((c == open_ch || c == close_ch) &&
					mode_char_is_structural(buf, line_num, off)) {
					if (c == close_ch) {
						depth++;
					} else {
						depth--;
						if (depth == 0) {
							match_out.line = line_num;
							match_out.offset = off;
							return true;
						}
					}
				}
				off--;
			}
			if (line_num == 1) {
				break;
			}
			line_num--;
			line = buf.line(line_num);
			if (line == NULL) {
				return false;
			}
			off = (int)line.byte_len() - 1;
		}
	}
	return false;
}

static bool location_has_delimiter(Buffer *buf, Location loc) {
	Line *line = NULL;
	int ch = 0;
	int open_ch = 0;
	int close_ch = 0;
	bool forward = false;
	if (loc.line < 1 || loc.line >= buf.eof_line()) {
		return false;
	}
	line = buf.line(loc.line);
	if (line == NULL || loc.offset >= line.byte_len()) {
		return false;
	}
	ch = line_byte_at(line, loc.offset);
	if (!mode_delimiter_pair(ch, &open_ch, &close_ch, &forward)) {
		return false;
	}
	(void)open_ch;
	(void)close_ch;
	(void)forward;
	return mode_char_is_structural(buf, loc.line, loc.offset);
}

static MatchResult match_target(Window *win, Location *match_out) {
	Buffer *buf = NULL;
	Location cursor = {0};
	Location prior = {0};
	if (win == NULL || win.buffer == NULL) {
		return MatchNone;
	}
	buf = win.buffer;
	cursor.line = win.cursor.line;
	cursor.offset = win.cursor.offset;
	if (cursor.line < 1 || cursor.line >= buf.eof_line()) {
		return MatchNone;
	}
	if (location_has_delimiter(buf, cursor)) {
		if (mode_find_matching_delimiter(buf, cursor, match_out)) {
			return MatchFound;
		}
		return MatchUnbalanced;
	}
	if (cursor.offset == 0) {
		return MatchNone;
	}
	prior.line = cursor.line;
	prior.offset = cursor.offset - 1;
	if (location_has_delimiter(buf, prior)) {
		if (mode_find_matching_delimiter(buf, prior, match_out)) {
			return MatchFound;
		}
		return MatchUnbalanced;
	}
	return MatchNone;
}

// mode_goto_match jumps to the matching delimiter (jem.go CmdModeGotoMatch / ModeGotoMatch).
bool mode_goto_match(bool f, int n) {
	Window *win = window_current();
	Location match = {0};
	(void)f;
	(void)n;
	if (win == NULL) {
		return false;
	}
	switch (match_target(win, &match)) {
	case MatchNone:
		mode_msg("[No bracket here]");
		return false;
	case MatchUnbalanced:
		mode_msg("[No matching bracket]");
		return false;
	default:
		win.set_cursor(match);
		win.did_move = true;
		return true;
	}
}

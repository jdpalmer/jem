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

static bool syn_context_is_code(SynState *st) {
	if (st == NULL) {
		return false;
	}
	switch (st.dfa) {
	case SynStateStringD:
	case SynStateStringDEsc:
	case SynStateStringS:
	case SynStateStringSEsc:
	case SynStateCmtLine:
	case SynStateCmtBlock:
	case SynStateCmtStar:
	case SynStateCmtBrace:
	case SynStateCmtParen:
	case SynStateCmtParen2:
	case SynStateLuaBlock:
	case SynStateLuaBlkEnd:
	case SynStateHTMLCmt:
	case SynStateHTMLCmtD1:
	case SynStateHTMLCmtD2:
		return false;
	default:
		return true;
	}
}

static int line_byte_at(Line *line, size_t offset) {
	char[..] v = line.view();
	if (offset >= len(v)) {
		return -1;
	}
	return ptr(v)[offset];
}

/* Ensure lines [1, line_number) have valid syntax_end (DFA carry). */
void mode_syntax_ensure_before(Buffer *buf, int line_number) {
	SynState st = {SynStateNormal};
	int q = 0;
	int sync = 1;
	TextStyle[..] nostyle = ranged((TextStyle *)0, 0);
	if (buf == NULL || line_number < 2) {
		return;
	}
	for (q = line_number - 1; q >= 1; q--) {
		Line *line = buf.line(q);
		if (line != NULL && line.syntax_valid) {
			st = line.syntax_end;
			sync = q + 1;
			break;
		}
	}
	for (q = sync; q < line_number; q++) {
		Line *line = buf.line(q);
		char[..] v = {0};
		SynState end = {SynStateNormal};
		if (line == NULL) {
			continue;
		}
		v = line.view();
		end = syntax_highlight_line(buf.lang_mode, &st, v, nostyle, 0);
		line.syntax_end = end;
		line.syntax_valid = true;
		st = end;
	}
}

// mode_syntax_state_at writes DFA state after processing line[0..offset).
bool mode_syntax_state_at(Buffer *buf, int line_number, size_t offset, SynState *out) {
	Line *line = NULL;
	SynState st = {SynStateNormal};
	TextStyle[..] nostyle = ranged((TextStyle *)0, 0);
	char[..] v = {0};
	if (out == NULL) {
		return false;
	}
	*out = st;
	if (buf == NULL || line_number < 1) {
		return false;
	}
	mode_syntax_ensure_before(buf, line_number);
	if (line_number > 1) {
		Line *pl = buf.line(line_number - 1);
		if (pl != NULL && pl.syntax_valid) {
			st = pl.syntax_end;
		}
	}
	line = buf.line(line_number);
	if (line == NULL) {
		*out = st;
		return false;
	}
	v = line.view();
	if (offset > len(v)) {
		offset = len(v);
	}
	if (offset == 0) {
		*out = st;
		return true;
	}
	/* Non-zero limit = prefix scan without EOL carry cleanup. */
	*out = syntax_highlight_line(buf.lang_mode, &st, v, nostyle, offset);
	return true;
}

bool mode_char_is_structural(Buffer *buf, int line_number, size_t offset) {
	SynState before = {SynStateNormal};
	SynState after = {SynStateNormal};
	if (buf == NULL || line_number < 1) {
		return false;
	}
	if (!mode_syntax_state_at(buf, line_number, offset, &before) ||
		!syn_context_is_code(&before)) {
		return false;
	}
	if (!mode_syntax_state_at(buf, line_number, offset + 1, &after) ||
		!syn_context_is_code(&after)) {
		return false;
	}
	return true;
}

/* Innermost unmatched open_ch still open at `at` (virtual close before at). */
bool mode_find_enclosing_open(Buffer *buf, Location at, int open_ch, int close_ch, Location *out) {
	int line_num = 0;
	int off = 0;
	int depth = 1;
	if (buf == NULL || out == NULL || at.line < 1) {
		return false;
	}
	mode_syntax_ensure_before(buf, at.line + 1);
	line_num = at.line;
	off = (int)at.offset - 1;
	while (line_num >= 1) {
		Line *line = buf.line(line_num);
		if (line == NULL) {
			return false;
		}
		if (off >= (int)line.byte_len()) {
			off = (int)line.byte_len() - 1;
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
						out.line = line_num;
						out.offset = off;
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
	return false;
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
	/* Structural checks ensure lines on demand; do not warm the whole buffer. */
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

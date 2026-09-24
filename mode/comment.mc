// mode/comment.mc — CommentDwim (toggle line comments).
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

// mode_line_comment returns the line-comment prefix for lang, or "" if none.
const char *mode_line_comment(LangMode lang) {
	switch (lang) {
	case LModePython:
	case LModeMake:
	case LModeR:
		return "#";
	case LModeLisp:
		return ";";
	case LModeLua:
		return "--";
	case LModeMarkdown:
	case LModePascal:
	case LModeHTML:
	case LModeCSS:
		return "";
	case LModeNone:
	case LModeC:
	case LModeJava:
	case LModeVerilog:
	case LModeSwift:
	case LModeJavaScript:
	case LModeActionScript:
	case LModeTypeScript:
	case LModeDart:
	case LModeGo:
	case LModeCSharp:
	case LModeRust:
	case LModeKotlin:
		return "//";
	}
	return "//";
}

static bool line_has_comment_prefix(Line *line, const char *prefix, size_t plen) {
	size_t pos = 0;
	char[..] v = {0};
	if (line == NULL || prefix == NULL || plen == 0) {
		return false;
	}
	pos = line.first_nonblank();
	v = line.view();
	if (pos + plen > len(v)) {
		return false;
	}
	return memcmp(ptr(v) + pos, prefix, plen) == 0;
}

static void comment_prefix_span(Line *line, const char *prefix, size_t plen, size_t *pos_out, size_t *end_out) {
	size_t pos = line.first_nonblank();
	size_t end = pos + plen;
	char[..] v = line.view();
	(void)prefix;
	if (end < len(v) && ptr(v)[end] == ' ') {
		end++;
	}
	*pos_out = pos;
	*end_out = end;
}

static void adjust_loc_for_delete(Location *loc, int line_num, size_t begin, size_t end) {
	if (loc == NULL || loc.line != line_num) {
		return;
	}
	if (loc.offset >= end) {
		loc.offset = loc.offset - (end - begin);
	} else if (loc.offset >= begin) {
		loc.offset = begin;
	}
}

static void adjust_loc_for_insert(Location *loc, int line_num, size_t at, size_t n) {
	if (loc == NULL || loc.line != line_num) {
		return;
	}
	if (loc.offset >= at) {
		loc.offset = loc.offset + n;
	}
}

static bool mode_uncomment_lines(Window *win, Buffer *buf, const char *prefix, size_t plen,
	int start_line, int end_line) {
	Location saved_cursor = {0};
	Location saved_mark = {0};
	int line_num = 0;

	saved_cursor = win.cursor;
	saved_mark = win.mark;
	buffer_undo.begin_command(buf, win.cursor);
	for (line_num = start_line; line_num <= end_line; line_num++) {
		Line *line = buf.line(line_num);
		size_t pos = 0;
		size_t end = 0;
		Location begin = {0};
		Location finish = {0};
		char *old = NULL;
		size_t old_len = 0;
		if (line == NULL || !line_has_comment_prefix(line, prefix, plen)) {
			continue;
		}
		comment_prefix_span(line, prefix, plen, &pos, &end);
		old_len = end - pos;
		old = malloc(old_len + 1);
		if (old == NULL) {
			win.set_cursor(saved_cursor);
			win.mark = saved_mark;
			buffer_undo.end_command();
			return false;
		}
		memcpy(old, ptr(line.data) + pos, old_len);
		old[old_len] = 0;
		begin.line = line_num;
		begin.offset = pos;
		finish.line = line_num;
		finish.offset = end;
		if (!win.set_text(begin, finish, "", 0)) {
			free(old);
			win.set_cursor(saved_cursor);
			win.mark = saved_mark;
			buffer_undo.end_command();
			return false;
		}
		buffer_undo.record_edit(UndoDelete, line_num, pos, old, old_len);
		free(old);
		adjust_loc_for_delete(&saved_cursor, line_num, pos, end);
		adjust_loc_for_delete(&saved_mark, line_num, pos, end);
	}
	buffer_undo.end_command();
	win.set_cursor(saved_cursor);
	win.mark = saved_mark;
	win.did_move = true;
	return true;
}

static bool mode_comment_lines(Window *win, Buffer *buf, const char *prefix, size_t plen,
	int start_line, int end_line) {
	char ins[16] = {0};
	size_t ins_len = 0;
	Location saved_cursor = {0};
	Location saved_mark = {0};
	int line_num = 0;

	if (plen == 0 || plen + 2 >= sizeof(ins)) {
		return false;
	}
	memcpy(ins, prefix, plen);
	ins_len = plen;
	if (prefix[plen - 1] != ' ') {
		ins[ins_len] = ' ';
		ins_len++;
	}
	ins[ins_len] = 0;
	saved_cursor = win.cursor;
	saved_mark = win.mark;
	buffer_undo.begin_command(buf, win.cursor);
	for (line_num = start_line; line_num <= end_line; line_num++) {
		Line *line = buf.line(line_num);
		size_t pos = 0;
		Location at = {0};
		if (line == NULL) {
			continue;
		}
		pos = line.first_nonblank();
		at.line = line_num;
		at.offset = pos;
		if (!win.set_text(at, at, ins, ins_len)) {
			win.set_cursor(saved_cursor);
			win.mark = saved_mark;
			buffer_undo.end_command();
			return false;
		}
		buffer_undo.record_edit(UndoInsert, line_num, pos, ins, ins_len);
		adjust_loc_for_insert(&saved_cursor, line_num, pos, ins_len);
		adjust_loc_for_insert(&saved_mark, line_num, pos, ins_len);
	}
	buffer_undo.end_command();
	win.set_cursor(saved_cursor);
	win.mark = saved_mark;
	win.did_move = true;
	return true;
}

static bool mode_toggle_comment_lines(Window *win, Buffer *buf, const char *prefix, size_t plen,
	int start_line, int end_line) {
	bool all_commented = true;
	int line_num = 0;
	for (line_num = start_line; line_num <= end_line; line_num++) {
		if (!line_has_comment_prefix(buf.line(line_num), prefix, plen)) {
			all_commented = false;
			break;
		}
	}
	if (all_commented) {
		return mode_uncomment_lines(win, buf, prefix, plen, start_line, end_line);
	}
	return mode_comment_lines(win, buf, prefix, plen, start_line, end_line);
}

// mode_comment_dwim comments or uncomments the current line / region.
bool mode_comment_dwim(bool f, int n) {
	Window *win = window_current();
	Buffer *buf = NULL;
	const char *prefix = NULL;
	size_t plen = 0;
	int start_line = 0;
	int end_line = 0;
	(void)f;
	(void)n;
	if (win == NULL || win.buffer == NULL) {
		return false;
	}
	buf = win.buffer;
	if (buf.is_readonly) {
		return false;
	}
	prefix = mode_line_comment(buf.lang_mode);
	if (prefix == NULL || prefix[0] == 0) {
		return false;
	}
	plen = strlen(prefix);
	start_line = win.cursor.line;
	end_line = win.cursor.line;
	if (win.mark.line != 0 && win.mark.line != win.cursor.line) {
		start_line = win.mark.line;
		end_line = win.cursor.line;
		if (start_line > end_line) {
			int tmp = start_line;
			start_line = end_line;
			end_line = tmp;
		}
	}
	return mode_toggle_comment_lines(win, buf, prefix, plen, start_line, end_line);
}

// mode/fill.mc — fill-paragraph (reflow words to fill_col).
import "str";

#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

enum {
	FillDefaultCol = 72,
	FillWordMax = 4096
};

static bool fill_is_space(char c) {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r';
}

// mode_fill_paragraph reflows the paragraph around point to buf.fill_col.
bool mode_fill_paragraph(bool f, int n) {
	Window *win = window_current();
	int start = 0;
	int ln = start;
	int fill_col = FillDefaultCol;
	char[..] blob = {0};
	char *words[FillWordMax] = {0};
	int word_count = 0;
	int wi = 0;
	(void)f;
	(void)n;

	if (win == NULL || win.buffer == NULL) {
		return false;
	}
	Buffer *buf = win.buffer;
	if (buf.is_readonly) {
		return false;
	}
	defer str_free(&blob);
	int line_num = win.cursor.line;
	start = line_num;
	while (start > 1) {
		Line *prev = buf.line(start - 1);
		if (prev == NULL || prev.is_blank()) {
			break;
		}
		start--;
	}
	int end = line_num;
	while (end < buf.line_count_i()) {
		Line *next = buf.line(end + 1);
		if (next == NULL || next.is_blank()) {
			break;
		}
		end++;
	}
	if (buf.fill_col > 0) {
		fill_col = buf.fill_col;
	}

	for (; ln <= end; ln++) {
		Line *line = buf.line(ln);
		char[..] v = {0};
		size_t i = 0;
		if (line == NULL) {
			continue;
		}
		v = line.view();
		while (i < len(v)) {
			size_t j = 0;
			size_t wlen = 0;
			size_t blob_len = 0;
			while (i < len(v) && fill_is_space(v[i])) {
				i++;
			}
			if (i >= len(v)) {
				break;
			}
			j = i;
			while (j < len(v) && !fill_is_space(v[j])) {
				j++;
			}
			wlen = j - i;
			if (word_count >= FillWordMax) {
				return false;
			}
			blob_len = len(blob);
			{
				char *old_base = ptr(blob);
				if (!str_reserve(&blob, blob_len + wlen + 1)) {
					return false;
				}
				char *new_base = ptr(blob);
				if (old_base != NULL && old_base != new_base) {
					int k = 0;
					for (k = 0; k < word_count; k++) {
						words[k] = new_base + (words[k] - old_base);
					}
				}
			}
			{
				char[..] slot = ranged(ptr(blob), blob_len + wlen + 1, cap(blob));
				memcpy(ptr(blob) + blob_len, ptr(v) + i, wlen);
				slot[blob_len + wlen] = 0;
				words[word_count] = ptr(blob) + blob_len;
				word_count++;
				blob = ranged(ptr(blob), blob_len + wlen + 1, cap(blob));
			}
			i = j;
		}
	}
	if (word_count == 0) {
		return true;
	}

	char[..] out = {0};
	size_t cur_len = 0;
	defer str_free(&out);
	for (; wi < word_count; wi++) {
		size_t wlen = strlen(words[wi]);
		if (cur_len == 0) {
			if (!str_append(&out, ranged(words[wi], wlen))) {
				return false;
			}
			cur_len = wlen;
			continue;
		}
		if (cur_len + 1 + wlen <= (size_t)fill_col) {
			if (!str_append_byte(&out, ' ')) {
				return false;
			}
			if (!str_append(&out, ranged(words[wi], wlen))) {
				return false;
			}
			cur_len = cur_len + 1 + wlen;
		} else {
			if (!str_append_byte(&out, '\n')) {
				return false;
			}
			if (!str_append(&out, ranged(words[wi], wlen))) {
				return false;
			}
			cur_len = wlen;
		}
	}

	Location begin = { .line = start, .offset = 0 };
	Line *end_line = buf.line(end);
	Location end_loc = { .line = end, .offset = end_line != NULL ? end_line.byte_len() : 0 };

	/* Capture old paragraph text for undo. */
	char[..] old = {0};
	defer str_free(&old);
	for (ln = start; ln <= end; ln++) {
		Line *line = buf.line(ln);
		if (line != NULL && line.byte_len() > 0) {
			if (!str_append(&old, line.data)) {
				return false;
			}
		}
		if (ln < end) {
			if (!str_append_byte(&old, '\n')) {
				return false;
			}
		}
	}

	buffer_undo.begin_command(buf, win.cursor);
	if (!win.set_text(begin, end_loc, ptr(out), len(out))) {
		buffer_undo.end_command();
		return false;
	}
	if (len(old) != 0) {
		buffer_undo.record_edit(UndoDelete, begin.line, begin.offset, ptr(old), len(old));
	}
	buffer_undo.record_edit(UndoInsert, begin.line, begin.offset, ptr(out), len(out));
	buffer_undo.end_command();
	return true;
}

// mode/fill.mc — fill-paragraph (reflow words to fill_col).
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
	Buffer *buf = NULL;
	int line_num = 0;
	int start = 0;
	int end = 0;
	int ln = 0;
	int fill_col = FillDefaultCol;
	char *blob = NULL;
	size_t blob_cap = 0;
	size_t blob_len = 0;
	char *words[FillWordMax] = {0};
	int word_count = 0;
	char *out = NULL;
	size_t out_cap = 0;
	size_t out_len = 0;
	size_t cur_len = 0;
	int wi = 0;
	Location begin = {0};
	Location end_loc = {0};
	Line *end_line = NULL;
	char *old = NULL;
	size_t old_len = 0;
	(void)f;
	(void)n;

	if (win == NULL || win.buffer == NULL) {
		return false;
	}
	buf = win.buffer;
	if (buf.is_readonly) {
		return false;
	}
	line_num = win.cursor.line;
	start = line_num;
	while (start > 1) {
		Line *prev = buf.line(start - 1);
		if (prev == NULL || prev.is_blank()) {
			break;
		}
		start--;
	}
	end = line_num;
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

	for (ln = start; ln <= end; ln++) {
		Line *line = buf.line(ln);
		char[..] v = {0};
		size_t i = 0;
		if (line == NULL) {
			continue;
		}
		v = line.view();
		while (i < len(v)) {
			char *vp = ptr(v);
			size_t j = 0;
			size_t wlen = 0;
			while (i < len(v) && fill_is_space(vp[i])) {
				i++;
			}
			if (i >= len(v)) {
				break;
			}
			j = i;
			while (j < len(v) && !fill_is_space(vp[j])) {
				j++;
			}
			wlen = j - i;
			if (word_count >= FillWordMax) {
				free(blob);
				return false;
			}
			if (blob_len + wlen + 1 > blob_cap) {
				size_t ncap = blob_cap != 0 ? blob_cap * 2 : 256;
				char *p = NULL;
				while (ncap < blob_len + wlen + 1) {
					ncap = ncap * 2;
				}
				p = realloc(blob, ncap);
				if (p == NULL) {
					free(blob);
					return false;
				}
				/* Rebase word pointers after realloc. */
				{
					int k = 0;
					for (k = 0; k < word_count; k++) {
						words[k] = p + (words[k] - blob);
					}
				}
				blob = p;
				blob_cap = ncap;
			}
			memcpy(blob + blob_len, vp + i, wlen);
			blob[blob_len + wlen] = 0;
			words[word_count] = blob + blob_len;
			word_count++;
			blob_len = blob_len + wlen + 1;
			i = j;
		}
	}
	if (word_count == 0) {
		free(blob);
		return true;
	}

	out_cap = blob_len + (size_t)word_count + 16;
	out = malloc(out_cap);
	if (out == NULL) {
		free(blob);
		return false;
	}
	out_len = 0;
	cur_len = 0;
	for (wi = 0; wi < word_count; wi++) {
		size_t wlen = strlen(words[wi]);
		if (cur_len == 0) {
			if (out_len + wlen + 2 > out_cap) {
				size_t ncap = out_cap * 2 + wlen + 2;
				char *p = realloc(out, ncap);
				if (p == NULL) {
					free(out);
					free(blob);
					return false;
				}
				out = p;
				out_cap = ncap;
			}
			memcpy(out + out_len, words[wi], wlen);
			cur_len = wlen;
			continue;
		}
		if (cur_len + 1 + wlen <= (size_t)fill_col) {
			if (out_len + cur_len + 1 + wlen + 2 > out_cap) {
				size_t ncap = out_cap * 2 + wlen + 2;
				char *p = realloc(out, ncap);
				if (p == NULL) {
					free(out);
					free(blob);
					return false;
				}
				out = p;
				out_cap = ncap;
			}
			out[out_len + cur_len] = ' ';
			memcpy(out + out_len + cur_len + 1, words[wi], wlen);
			cur_len = cur_len + 1 + wlen;
		} else {
			if (out_len + cur_len + 1 + wlen + 2 > out_cap) {
				size_t ncap = out_cap * 2 + cur_len + wlen + 2;
				char *p = realloc(out, ncap);
				if (p == NULL) {
					free(out);
					free(blob);
					return false;
				}
				out = p;
				out_cap = ncap;
			}
			out_len = out_len + cur_len;
			out[out_len] = '\n';
			out_len++;
			memcpy(out + out_len, words[wi], wlen);
			cur_len = wlen;
		}
	}
	out_len = out_len + cur_len;
	out[out_len] = 0;

	begin.line = start;
	begin.offset = 0;
	end_line = buf.line(end);
	end_loc.line = end;
	end_loc.offset = end_line != NULL ? end_line.byte_len() : 0;

	/* Capture old paragraph text for undo. */
	{
		size_t need = 0;
		size_t off = 0;
		for (ln = start; ln <= end; ln++) {
			Line *line = buf.line(ln);
			if (line != NULL) {
				need = need + line.byte_len();
			}
			if (ln < end) {
				need++;
			}
		}
		old = malloc(need + 1);
		if (old != NULL) {
			for (ln = start; ln <= end; ln++) {
				Line *line = buf.line(ln);
				if (line != NULL && line.byte_len() > 0) {
					memcpy(old + off, ptr(line.data), line.byte_len());
					off = off + line.byte_len();
				}
				if (ln < end) {
					old[off] = '\n';
					off++;
				}
			}
			old[off] = 0;
			old_len = off;
		}
	}

	buffer_undo.begin_command(buf, win.cursor);
	if (!win.set_text(begin, end_loc, out, out_len)) {
		free(old);
		free(out);
		free(blob);
		buffer_undo.end_command();
		return false;
	}
	if (old != NULL) {
		buffer_undo.record_edit(UndoDelete, begin.line, begin.offset, old, old_len);
	}
	buffer_undo.record_edit(UndoInsert, begin.line, begin.offset, out, out_len);
	free(old);
	free(out);
	free(blob);
	buffer_undo.end_command();
	return true;
}

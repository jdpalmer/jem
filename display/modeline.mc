// display/modeline.mc — Go-style modeline (jem.go renderModeline).
#include <stdio.h>
#include <string.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

static bool g_macro_recording;
static bool g_macro_present;

void display_set_macro_state(bool recording, bool present) {
	g_macro_recording = recording;
	g_macro_present = present;
}

static void fit_buffer_name(const char[..] name, char *out, size_t cap) {
	size_t n = len(name);
	int width = 0;
	size_t end = 0;
	int max_cols = 16;
	if (out == NULL || cap == 0) {
		return;
	}
	out[0] = 0;
	if (n == 0 || ptr(name) == NULL) {
		return;
	}
	{
		int col = 0;
		size_t at = 0;
		while (at < n) {
			auto (r, sz) = utf8_decode_rune(name, at);
			int w = 0;
			if (sz == 0) {
				break;
			}
			w = rune_display_width(r);
			if (w <= 0) {
				w = 1;
			}
			col += w;
			at += sz;
		}
		width = col;
	}
	if (width <= max_cols) {
		if (n >= cap) {
			n = cap - 1;
		}
		memcpy(out, ptr(name), n);
		out[n] = 0;
		return;
	}
	{
		int budget = max_cols - 1;
		int col = 0;
		size_t at = 0;
		end = 0;
		while (at < n) {
			auto (r, sz) = utf8_decode_rune(name, at);
			int w = 0;
			if (sz == 0) {
				break;
			}
			w = rune_display_width(r);
			if (w <= 0) {
				w = 1;
			}
			if (col + w > budget) {
				break;
			}
			col += w;
			at += sz;
			end = at;
		}
	}
	if (end + 4 >= cap) {
		if (cap > 4) {
			end = cap - 4;
		} else {
			end = 0;
		}
	}
	if (end > 0) {
		memcpy(out, ptr(name), end);
	}
	out[end] = 0xE2;
	out[end + 1] = 0x80;
	out[end + 2] = 0xA6;
	out[end + 3] = 0;
}

static void put_ascii(int row, int *col, const char *s, TextStyle st) {
	int cols = term_cols();
	if (s == NULL || col == NULL) {
		return;
	}
	while (*s != 0 && *col < cols) {
		put_rune(&g_screen, row, *col, *s, st);
		(*col)++;
		s++;
	}
}

void paint_modeline(Window *win, int row) {
	TextStyle gut = style_gutter();
	TextStyle name_st = style_modeline_name();
	TextStyle dirty_st = style_modeline_dirty();
	char fitted[32] = {0};
	char pos[64] = {0};
	const char *lang = {0};
	const char *eol = {0};
	int col = 0;
	int cols = 0;
	int gutter_w = 0;
	int marker_col = 0;
	int hint_col = 0;
	int pct = 0;
	int lineno = 1;
	int tot = 1;
	int colno = 1;
	Line *cline = {0};
	static char hint[] = "Esc+x = Commands";
	int hint_len = 0;
	bool show_hint = false;
	int pad_end = 0;
	if (win == NULL || win.buffer == NULL || row < 0) {
		return;
	}
	if (row >= term_rows()) {
		row = term_rows() - 1;
	}
	cols = term_cols();
	hint_len = (int)strlen(hint);
	gutter_w = window_gutter_width(win);
	lineno = win.cursor.line;
	tot = win.buffer.line_count_i();
	if (tot < 1) {
		tot = 1;
	}
	if (lineno >= tot) {
		pct = 100;
	} else if (tot > 0) {
		pct = (lineno * 100) / tot;
	}
	cline = win.buffer.line(lineno);
	if (cline != NULL) {
		colno = (int)cline.display_col_at_offset(win.cursor.offset) + 1;
	}
	snprintf(pos, sizeof(pos), "%3d%% L%d C%d", pct, lineno, colno);
	lang = mode_display_name(win.buffer.lang_mode);
	eol = "LF";
	if (win.buffer.eol_mode == EModeCRLF) {
		eol = "CRLF";
	}
	if (len(win.buffer.name) > 0) {
		fit_buffer_name(win.buffer.name, fitted, sizeof(fitted));
	} else {
		snprintf(fitted, sizeof(fitted), "[No File]");
	}
	marker_col = gutter_w + 79 - win.h_scroll;
	hint_col = cols - hint_len;

	if (win.buffer.is_changed) {
		put_rune(&g_screen, row, col, '*', dirty_st);
	} else {
		put_rune(&g_screen, row, col, ' ', gut);
	}
	col++;
	put_rune(&g_screen, row, col, ' ', gut);
	col++;
	if (g_macro_recording) {
		put_rune(&g_screen, row, col, 'm', dirty_st);
	} else if (g_macro_present) {
		put_rune(&g_screen, row, col, 'm', name_st);
	} else {
		put_rune(&g_screen, row, col, ' ', gut);
	}
	col++;
	if (markring_count() > 0) {
		put_rune(&g_screen, row, col, 's', name_st);
	} else {
		put_rune(&g_screen, row, col, ' ', gut);
	}
	col++;
	put_rune(&g_screen, row, col, 'b', name_st);
	col++;
	put_ascii(row, &col, "  jem 26.1 | ", gut);
	put_ascii(row, &col, fitted, name_st);
	put_ascii(row, &col, " | ", gut);
	put_ascii(row, &col, eol, gut);
	put_ascii(row, &col, " | ", gut);
	put_ascii(row, &col, lang, gut);
	{
		const char *git = git_modeline_text(win.buffer);
		if (git != NULL && git[0] != 0) {
			put_ascii(row, &col, " | ", gut);
			put_ascii(row, &col, git, name_st);
		}
	}
	put_ascii(row, &col, " | ", gut);
	put_ascii(row, &col, pos, gut);

	show_hint = col < hint_col;
	pad_end = cols;
	if (show_hint) {
		pad_end = hint_col;
	}
	if (show_hint && marker_col >= hint_col) {
		marker_col = -1;
	}
	if (marker_col < pad_end && (marker_col < col || marker_col < gutter_w)) {
		marker_col = -1;
	}
	while (col < pad_end && col < cols) {
		uint32_t ch = ' ';
		if (col == marker_col) {
			ch = '^';
		}
		put_rune(&g_screen, row, col, ch, gut);
		col++;
	}
	if (show_hint) {
		put_ascii(row, &col, hint, gut);
	}
	while (col < cols) {
		uint32_t ch = ' ';
		if (col == marker_col) {
			ch = '^';
		}
		put_rune(&g_screen, row, col, ch, gut);
		col++;
	}
}

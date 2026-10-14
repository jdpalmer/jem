// display — redisplay, modeline, Solarized dark theme.
import "buffer";
import "window";
import "term";
import "syntax";
import "minibuffer";
import "mode";
import "markring";
import "str";
import "os";
import "path";

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <sys/types.h>

struct DispCell {
	uint32_t ch;
	TextStyle style;
};

struct Screen {
	DispCell *cells;
	bool *row_dirty;
	int rows;
	int cols;
};

Screen g_screen;
static Screen g_prev;
static bool g_theme_light;
static bool g_screen_dirty;
static bool g_have_style;
static TextStyle g_active_style;
static char g_echo[256];
static uint64_t g_echo_generation;

static TextStyle style_default() {
	if (g_theme_light) {
		return text_style_make(TermColorBase00, TermColorBase3, 0);
	}
	return text_style_make(TermColorBase0, TermColorBase03, 0);
}

/* Map TermColorDefault onto the theme so syntax (bg=Default) matches empty cells. */
static TextStyle style_resolve(TextStyle style) {
	TextStyle def = style_default();
	TermColor fg = text_style_fg(style);
	TermColor bg = text_style_bg(style);
	uint16_t flags = (uint16_t)(style.bits & ((uint16_t)TextStyleBold |
		(uint16_t)TextStyleUnderline | (uint16_t)TextStyleReverse));
	if (fg == TermColorDefault) {
		fg = text_style_fg(def);
	}
	if (bg == TermColorDefault) {
		bg = text_style_bg(def);
	}
	return text_style_make(fg, bg, flags);
}

TextStyle style_gutter() {
	if (g_theme_light) {
		return text_style_make(TermColorBase1, TermColorBase2, 0);
	}
	return text_style_make(TermColorBase01, TermColorBase02, 0);
}

TextStyle style_modeline_name() {
	TermColor bg = text_style_bg(style_gutter());
	if (g_theme_light) {
		return text_style_make(TermColorBase01, bg, (uint16_t)TextStyleBold);
	}
	return text_style_make(TermColorBase1, bg, (uint16_t)TextStyleBold);
}

TextStyle style_modeline_dirty() {
	return text_style_make(TermColorRed, text_style_bg(style_gutter()), (uint16_t)TextStyleBold);
}

static TermColor theme_selection_bg() {
	return TermColorYellow;
}

static TermColor theme_search_match_bg() {
	if (g_theme_light) {
		return TermColorBase00;
	}
	return TermColorBase01;
}

static TextStyle selection_style(TextStyle base) {
	uint16_t flags = (uint16_t)(base.bits & (uint16_t)TextStyleBold);
	return text_style_make(TermColorBase03, theme_selection_bg(), flags);
}

static TextStyle search_match_style(TextStyle base) {
	uint16_t flags = (uint16_t)(base.bits & ((uint16_t)TextStyleBold |
		(uint16_t)TextStyleUnderline));
	return text_style_make(text_style_fg(base), theme_search_match_bg(), flags);
}

enum SelPhase {
	SelBefore = 0,
	SelInside = 1,
	SelAfter = 2
};

struct SelState {
	bool active;
	int phase;
	int start_line;
	int end_line;
	size_t start_o;
	size_t end_o;
};

static void sel_init(SelState *ss, Window *win) {
	int mark_line = 0;
	int dot_line = 0;
	size_t marko = 0;
	size_t doto = 0;
	if (ss == NULL) {
		return;
	}
	ss.active = false;
	ss.phase = SelBefore;
	if (win == NULL || win.mark.line == 0) {
		return;
	}
	mark_line = win.mark.line;
	dot_line = win.cursor.line;
	marko = win.mark.offset;
	doto = win.cursor.offset;
	if (mark_line == dot_line) {
		if (marko == doto) {
			return;
		}
		ss.active = true;
		ss.start_line = mark_line;
		ss.end_line = mark_line;
		if (marko < doto) {
			ss.start_o = marko;
			ss.end_o = doto;
		} else {
			ss.start_o = doto;
			ss.end_o = marko;
		}
		return;
	}
	ss.active = true;
	if (mark_line < dot_line) {
		ss.start_line = mark_line;
		ss.start_o = marko;
		ss.end_line = dot_line;
		ss.end_o = doto;
	} else {
		ss.start_line = dot_line;
		ss.start_o = doto;
		ss.end_line = mark_line;
		ss.end_o = marko;
	}
	if (win.top_line > ss.end_line) {
		ss.active = false;
	} else if (win.top_line > ss.start_line) {
		ss.phase = SelInside;
	}
}

/* Returns start==-1 when this line has no selection highlight. */
static void sel_line(SelState *ss, int line_number, size_t line_len, ssize_t *start, size_t *end) {
	if (start == NULL || end == NULL) {
		return;
	}
	*start = -1;
	*end = 0;
	if (ss == NULL || !ss.active) {
		return;
	}
	if (ss.phase == (int)SelBefore) {
		if (line_number != ss.start_line) {
			return;
		}
		if (ss.start_line == ss.end_line) {
			*start = ss.start_o;
			*end = ss.end_o;
			ss.phase = SelAfter;
		} else {
			*start = ss.start_o;
			*end = line_len;
			ss.phase = SelInside;
		}
	} else if (ss.phase == (int)SelInside) {
		if (line_number == ss.end_line) {
			*start = 0;
			*end = ss.end_o;
			ss.phase = SelAfter;
		} else {
			*start = 0;
			*end = line_len;
		}
	}
}

static bool in_search_span(Window *win, int line_number, size_t byte_off) {
	int i = 0;
	if (win == NULL) {
		return false;
	}
	for (i = 0; i < win.search_span_count; i++) {
		SearchSpan sp = win.search_spans[i];
		if (sp.line == line_number && byte_off >= sp.start && byte_off < sp.end) {
			return true;
		}
	}
	return false;
}

static void screen_free(Screen *s) {
	free(s.cells);
	free(s.row_dirty);
	s.cells = NULL;
	s.row_dirty = NULL;
	s.rows = 0;
	s.cols = 0;
}

static bool screen_resize(Screen *s, int rows, int cols) {
	DispCell *p = {0};
	bool *dirty = {0};
	int n = 0;
	int i = 0;
	TextStyle def = style_default();
	if (rows < 1) {
		rows = 1;
	}
	if (cols < 1) {
		cols = 1;
	}
	if (s.rows == rows && s.cols == cols && s.cells != NULL && s.row_dirty != NULL) {
		return true;
	}
	n = rows * cols;
	p = calloc(n, sizeof(DispCell));
	dirty = calloc(rows, sizeof(bool));
	if (p == NULL || dirty == NULL) {
		free(p);
		free(dirty);
		return false;
	}
	for (i = 0; i < n; i++) {
		p[i].ch = ' ';
		p[i].style = def;
	}
	free(s.cells);
	free(s.row_dirty);
	s.cells = p;
	s.row_dirty = dirty;
	s.rows = rows;
	s.cols = cols;
	return true;
}

DispCell *cell_at(Screen *s, int row, int col) {
	if (row < 0 || col < 0 || row >= s.rows || col >= s.cols) {
		return NULL;
	}
	return &s.cells[row * s.cols + col];
}

void put_rune(Screen *s, int row, int col, uint32_t ch, TextStyle style) {
	DispCell *c = cell_at(s, row, col);
	if (c == NULL) {
		return;
	}
	if (c.ch == ch && text_style_eq(c.style, style)) {
		return;
	}
	c.ch = ch;
	c.style = style;
	if (s.row_dirty != NULL && row >= 0 && row < s.rows) {
		s.row_dirty[row] = true;
	}
}

/* Solarized / ANSI RGB — match jem.go/term/term_style.go (truecolor, not 256). */
static const char *color_rgb_cstr(TermColor c) {
	static const char *base[8] = {
		"7;54;66",
		"220;50;47",
		"133;153;0",
		"181;137;0",
		"38;139;210",
		"211;54;130",
		"42;161;152",
		"238;232;213",
	};
	static const char *sol[8] = {
		"0;43;54",
		"7;54;66",
		"88;110;117",
		"101;123;131",
		"131;148;150",
		"147;161;161",
		"238;232;213",
		"253;246;227",
	};
	int i = c;
	if (i >= 0 && i <= 7) {
		return base[i];
	}
	if (i >= 17 && i <= 24) {
		return sol[i - 17];
	}
	return NULL;
}

/* Append color params into SGR seq; returns new length or -1 on overflow. */
static int append_color_sgr(char *buf, size_t cap, int n, TermColor c, int base_sgr) {
	const char *rgb = {0};
	int i = c;
	int wn = 0;
	if (c == TermColorDefault || n < 0) {
		return n;
	}
	if ((size_t)n >= cap) {
		return -1;
	}
	if (i >= 8 && i < 16) {
		/* bright ANSI 90–97 / 100–107 */
		wn = snprintf(buf + n, cap - (size_t)n, ";%d", base_sgr + 60 + (i - 8));
	} else {
		rgb = color_rgb_cstr(c);
		if (rgb == NULL) {
			return n;
		}
		/* 38;2;r;g;b / 48;2;r;g;b */
		wn = snprintf(buf + n, cap - (size_t)n, ";%d;2;%s", base_sgr + 8, rgb);
	}
	if (wn < 0 || (size_t)wn >= cap - (size_t)n) {
		return -1;
	}
	return n + wn;
}

void emit_sgr(TextStyle style) {
	/* One contiguous CSI: Windows Console VT does not reassemble SGR split
	 * across WriteFile calls (fragments like "?2" appear as literal text). */
	char seq[96] = {0};
	int n = 0;
	TextStyle resolved = style_resolve(style);
	TermColor fg = text_style_fg(resolved);
	TermColor bg = text_style_bg(resolved);
	if (resolved.bits & (uint16_t)TextStyleReverse) {
		TermColor tmp = fg;
		fg = bg;
		bg = tmp;
	}
	n = snprintf(seq, sizeof(seq), "\x1b[0%s%s%s",
		(resolved.bits & (uint16_t)TextStyleBold) ? ";1" : "",
		(resolved.bits & (uint16_t)TextStyleUnderline) ? ";4" : "",
		(resolved.bits & (uint16_t)TextStyleReverse) ? ";7" : "");
	if (n < 0 || (size_t)n >= sizeof(seq)) {
		return;
	}
	n = append_color_sgr(seq, sizeof(seq), n, fg, 30);
	n = append_color_sgr(seq, sizeof(seq), n, bg, 40);
	if (n < 0 || (size_t)n + 1 >= sizeof(seq)) {
		return;
	}
	seq[n++] = 'm';
	seq[n] = 0;
	term_write(seq, n);
}

static void paint_gutter(Window *win, int screen_row, int lineno) {
	TextStyle st = style_gutter();
	TextStyle mark_st = st;
	int width = 0;
	int num_width = 0;
	int nlen = 0;
	int i = 0;
	int marker = GitLineDiffNone;
	char num[16] = {0};
	uint32_t mark_ch = ' ';
	if (win == NULL || screen_row < 0) {
		return;
	}
	width = window_gutter_width(win);
	if (width < 1) {
		return;
	}
	if (width > term_cols()) {
		width = term_cols();
	}
	if (lineno > 0 && win.buffer != NULL) {
		marker = git_line_diff(win.buffer, lineno);
	}
	if (marker == (int)GitLineDiffAdded) {
		mark_ch = '+';
		mark_st = text_style_make(TermColorGreen, text_style_bg(st), (uint16_t)TextStyleBold);
	} else if (marker == (int)GitLineDiffModified) {
		mark_ch = '~';
		mark_st = text_style_make(TermColorYellow, text_style_bg(st), (uint16_t)TextStyleBold);
	} else if (marker == (int)GitLineDiffDeleted) {
		mark_ch = '-';
		mark_st = text_style_make(TermColorRed, text_style_bg(st), (uint16_t)TextStyleBold);
	}
	put_rune(&g_screen, screen_row, 0, mark_ch, mark_st);
	num_width = width - 2;
	if (num_width < 0) {
		num_width = 0;
	}
	if (lineno > 0) {
		nlen = snprintf(num, sizeof(num), "%d", lineno);
		if (nlen < 0) {
			nlen = 0;
		}
	}
	for (i = 0; i < num_width; i++) {
		char ch = ' ';
		int src = i - (num_width - nlen);
		if (src >= 0 && src < nlen) {
			ch = num[src];
		}
		put_rune(&g_screen, screen_row, 1 + i, ch, st);
	}
	if (width >= 2) {
		uint32_t clip = ' ';
		if (win.h_scroll > 0) {
			clip = '<';
		}
		put_rune(&g_screen, screen_row, width - 1, clip, st);
	}
}

static void paint_content_cell(int screen_row, int dcol, int h_scroll, int gutter, uint32_t ch, TextStyle st) {
	int scol = 0;
	int cols = term_cols();
	if (dcol < h_scroll) {
		return;
	}
	scol = gutter + (dcol - h_scroll);
	if (scol < gutter || scol >= cols) {
		return;
	}
	put_rune(&g_screen, screen_row, scol, ch, st);
}

void paint_blank_row(Window *win, int screen_row) {
	TextStyle def = style_default();
	int col = 0;
	int cols = term_cols();
	for (col = 0; col < cols; col++) {
		put_rune(&g_screen, screen_row, col, ' ', def);
	}
	paint_gutter(win, screen_row, 0);
}

void paint_line_text(Window *win, int screen_row, int buf_line, SelState *sel) {
	Line *line = {0};
	char[..] v = {0};
	TextStyle *styles = {0};
	size_t styles_len = 0;
	int dcol = 0;
	size_t i = 0;
	int gutter = 0;
	int h_scroll = 0;
	int cols = 0;
	ssize_t sel_start = -1;
	size_t sel_end = 0;
	size_t line_len = 0;
	TextStyle def = style_default();
	SynState prev = {SynStateNormal};
	SynState end = {SynStateNormal};

	line = win.buffer.line(buf_line);
	if (line == NULL) {
		paint_blank_row(win, screen_row);
		return;
	}
	gutter = window_gutter_width(win);
	h_scroll = win.h_scroll;
	if (h_scroll < 0) {
		h_scroll = 0;
	}
	cols = term_cols();
	v = line.view();
	line_len = len(v);
	sel_line(sel, buf_line, line_len, &sel_start, &sel_end);

	/* highlight if needed (structure-only parse may set syntax_valid without styles) */
	if ((!line.syntax_valid || line.styles == NULL) && len(v) != 0) {
		TextStyle *st = calloc(len(v), sizeof(TextStyle));
		if (st != NULL) {
			if (buf_line > 1) {
				Line *pl = win.buffer.line(buf_line - 1);
				if (pl != NULL && pl.syntax_valid) {
					prev = pl.syntax_end;
				}
			}
			end = syntax_highlight_line(win.buffer.lang_mode, &prev, v, ranged(st, len(v)), 0);
			free(line.styles);
			line.styles = st;
			line.styles_len = len(v);
			line.syntax_end = end;
			line.syntax_valid = true;
		}
	}
	styles = line.styles;
	styles_len = line.styles_len;

	if (line_len == 0) {
		int fill = gutter;
		if (sel_start >= 0) {
			paint_content_cell(screen_row, 0, h_scroll, gutter, ' ', selection_style(def));
			fill = gutter + 1;
			if (fill < gutter) {
				fill = gutter;
			}
		}
		while (fill < cols) {
			put_rune(&g_screen, screen_row, fill, ' ', def);
			fill++;
		}
		paint_gutter(win, screen_row, buf_line);
		return;
	}

	while (i < line_len) {
		auto (r, sz) = utf8_decode_rune(v, i);
		int w = 0;
		TextStyle st = def;
		int c = 0;
		int scol = 0;
		if (sz == 0) {
			break;
		}
		if (styles != NULL && i < styles_len) {
			st = styles[i];
			if (st.bits == 0) {
				st = def;
			} else {
				st = style_resolve(st);
			}
		}
		if (in_search_span(win, buf_line, i)) {
			st = search_match_style(st);
		}
		if (sel_start >= 0 && i >= (size_t)sel_start && i < sel_end) {
			st = selection_style(st);
		}
		if (r == '\t') {
			w = 8 - (dcol % 8);
			if (w <= 0) {
				w = 8;
			}
		} else {
			w = rune_display_width(r);
			if (w <= 0) {
				w = 1;
			}
		}
		scol = gutter + (dcol - h_scroll);
		if (dcol >= h_scroll && scol >= cols) {
			if (cols > 0) {
				put_rune(&g_screen, screen_row, cols - 1, '$', def);
			}
			break;
		}
		if (r == '\t') {
			for (c = 0; c < w; c++) {
				paint_content_cell(screen_row, dcol + c, h_scroll, gutter, ' ', st);
			}
		} else {
			paint_content_cell(screen_row, dcol, h_scroll, gutter, r, st);
			for (c = 1; c < w; c++) {
				paint_content_cell(screen_row, dcol + c, h_scroll, gutter, 0, st);
			}
		}
		dcol += w;
		i = i + sz;
	}
	{
		int fill = gutter;
		if (dcol > h_scroll) {
			fill = gutter + (dcol - h_scroll);
		}
		if (fill < gutter) {
			fill = gutter;
		}
		while (fill < cols) {
			put_rune(&g_screen, screen_row, fill, ' ', def);
			fill++;
		}
	}
	paint_gutter(win, screen_row, buf_line);
}

void paint_echo(int row) {
	TextStyle st = style_default();
	int i = 0;
	size_t n = strlen(g_echo);
	for (i = 0; i < term_cols(); i++) {
		uint32_t ch = ' ';
		if ((size_t)i < n) {
			ch = g_echo[i];
		}
		put_rune(&g_screen, row, i, ch, st);
	}
}

static void emit_sgr_cached(TextStyle style) {
	TextStyle cur = g_active_style;
	if (g_have_style && text_style_eq(cur, style)) {
		return;
	}
	emit_sgr(style);
	g_active_style = style;
	g_have_style = true;
}

static void screen_write_span(int row, int from, int to) {
	char out[4096] = {0};
	int out_len = 0;
	int i = 0;
	for (i = from; i < to; i++) {
		DispCell *c = cell_at(&g_screen, row, i);
		uint32_t ch = 0;
		char tmp[4] = {0};
		int sz = 0;
		if (c == NULL || c.ch == 0) {
			continue;
		}
		ch = c.ch;
		if (ch < 128) {
			tmp[0] = (char)ch;
			sz = 1;
		} else if (ch < 0x800) {
			tmp[0] = (char)(0xC0 | (ch >> 6));
			tmp[1] = (char)(0x80 | (ch & 0x3F));
			sz = 2;
		} else if (ch < 0x10000) {
			tmp[0] = (char)(0xE0 | (ch >> 12));
			tmp[1] = (char)(0x80 | ((ch >> 6) & 0x3F));
			tmp[2] = (char)(0x80 | (ch & 0x3F));
			sz = 3;
		} else {
			tmp[0] = (char)(0xF0 | (ch >> 18));
			tmp[1] = (char)(0x80 | ((ch >> 12) & 0x3F));
			tmp[2] = (char)(0x80 | ((ch >> 6) & 0x3F));
			tmp[3] = (char)(0x80 | (ch & 0x3F));
			sz = 4;
		}
		if (out_len + sz > (int)sizeof(out)) {
			term_write(out, out_len);
			out_len = 0;
		}
		memcpy(out + out_len, tmp, sz);
		out_len += sz;
	}
	if (out_len > 0) {
		term_write(out, out_len);
	}
}

static void copy_front_cell(int row, int col) {
	DispCell *a = cell_at(&g_screen, row, col);
	DispCell *b = cell_at(&g_prev, row, col);
	if (a == NULL || b == NULL) {
		return;
	}
	*b = *a;
}

static void row_emit_diffs(int row, int from, int to) {
	int k = from;
	int cols = g_screen.cols;
	if (to > cols) {
		to = cols;
	}
	while (k < to) {
		DispCell *a = {0};
		DispCell *b = {0};
		while (k < to) {
			a = cell_at(&g_screen, row, k);
			b = cell_at(&g_prev, row, k);
			if (a == NULL || b == NULL) {
				break;
			}
			if (a.ch != b.ch || !text_style_eq(a.style, b.style)) {
				break;
			}
			k++;
		}
		if (k >= to) {
			break;
		}
		term_move(row, k);
		while (k < to) {
			TextStyle style = {0};
			int run = 0;
			a = cell_at(&g_screen, row, k);
			b = cell_at(&g_prev, row, k);
			if (a == NULL || b == NULL) {
				break;
			}
			if (a.ch == b.ch && text_style_eq(a.style, b.style)) {
				break;
			}
			style = a.style;
			emit_sgr_cached(style);
			run = k;
			while (run < to) {
				DispCell *ra = cell_at(&g_screen, row, run);
				DispCell *rb = cell_at(&g_prev, row, run);
				if (ra == NULL || rb == NULL) {
					break;
				}
				if (!text_style_eq(ra.style, style)) {
					break;
				}
				if (ra.ch == rb.ch && text_style_eq(ra.style, rb.style)) {
					break;
				}
				run++;
			}
			screen_write_span(row, k, run);
			while (k < run) {
				copy_front_cell(row, k);
				k++;
			}
		}
	}
}

static void row_sync(int row) {
	int cols = g_screen.cols;
	int first = 0;
	int last = 0;
	int lastsp = 0;
	TextStyle normal = style_default();
	if (row < 0 || row >= g_screen.rows || cols <= 0) {
		return;
	}
	while (first < cols) {
		DispCell *a = cell_at(&g_screen, row, first);
		DispCell *b = cell_at(&g_prev, row, first);
		if (a == NULL || b == NULL || a.ch != b.ch || !text_style_eq(a.style, b.style)) {
			break;
		}
		first++;
	}
	if (first == cols) {
		return;
	}
	last = cols - 1;
	while (last > first) {
		DispCell *a = cell_at(&g_screen, row, last);
		DispCell *b = cell_at(&g_prev, row, last);
		if (a == NULL || b == NULL || a.ch != b.ch || !text_style_eq(a.style, b.style)) {
			break;
		}
		last--;
	}
	lastsp = cols;
	while (lastsp > first) {
		DispCell *a = cell_at(&g_screen, row, lastsp - 1);
		if (a == NULL || a.ch != (uint32_t)' ' || !text_style_eq(a.style, normal)) {
			break;
		}
		lastsp--;
	}
	if (lastsp <= last) {
		int i = 0;
		row_emit_diffs(row, first, lastsp);
		term_move(row, lastsp);
		emit_sgr_cached(normal);
		term_erase_eol();
		for (i = lastsp; i < cols; i++) {
			DispCell *b = cell_at(&g_prev, row, i);
			if (b != NULL) {
				b.ch = ' ';
				b.style = normal;
			}
		}
	} else {
		row_emit_diffs(row, first, last + 1);
	}
}

static void mark_all_dirty() {
	int i = 0;
	if (g_screen.row_dirty == NULL) {
		return;
	}
	for (i = 0; i < g_screen.rows; i++) {
		g_screen.row_dirty[i] = true;
	}
}

static void reset_front_to_spaces() {
	int i = 0;
	int n = 0;
	TextStyle normal = style_default();
	if (g_prev.cells == NULL) {
		return;
	}
	n = g_prev.rows * g_prev.cols;
	for (i = 0; i < n; i++) {
		g_prev.cells[i].ch = ' ';
		g_prev.cells[i].style = normal;
	}
}

static void screen_sync() {
	int i = 0;
	int sync_rows = term_rows();
	if (g_screen_dirty) {
		TextStyle normal = style_default();
		mark_all_dirty();
		reset_front_to_spaces();
		term_move(0, 0);
		emit_sgr_cached(normal);
		term_erase_eos();
		g_have_style = false;
		g_screen_dirty = false;
	}
	if (term_rows() < g_screen.rows) {
		int msg = term_rows();
		if (g_screen.row_dirty != NULL && msg >= 0 && msg < g_screen.rows && g_screen.row_dirty[msg]) {
			sync_rows = term_rows() + 1;
		}
	}
	for (i = 0; i < sync_rows && i < g_screen.rows; i++) {
		if (g_screen.row_dirty != NULL && g_screen.row_dirty[i]) {
			g_screen.row_dirty[i] = false;
			row_sync(i);
		}
	}
}

static void screen_rows_scroll_up(Screen *s, int start, int length, int n) {
	DispCell *tmp = {0};
	bool *dtmp = {0};
	int cols = 0;
	if (s == NULL || s.cells == NULL || n <= 0 || n >= length) {
		return;
	}
	if (start < 0 || start + length > s.rows) {
		return;
	}
	cols = s.cols;
	tmp = malloc((size_t)n * (size_t)cols * sizeof(DispCell));
	if (tmp == NULL) {
		return;
	}
	memcpy(tmp, s.cells + start * cols, (size_t)n * (size_t)cols * sizeof(DispCell));
	memmove(s.cells + start * cols, s.cells + (start + n) * cols,
		(size_t)(length - n) * (size_t)cols * sizeof(DispCell));
	memcpy(s.cells + (start + length - n) * cols, tmp, (size_t)n * (size_t)cols * sizeof(DispCell));
	free(tmp);
	if (s.row_dirty != NULL) {
		dtmp = malloc((size_t)n * sizeof(bool));
		if (dtmp != NULL) {
			memcpy(dtmp, s.row_dirty + start, (size_t)n * sizeof(bool));
			memmove(s.row_dirty + start, s.row_dirty + start + n, (size_t)(length - n) * sizeof(bool));
			memcpy(s.row_dirty + start + length - n, dtmp, (size_t)n * sizeof(bool));
			free(dtmp);
		}
	}
}

static void screen_rows_scroll_down(Screen *s, int start, int length, int n) {
	screen_rows_scroll_up(s, start, length, length - n);
}

static void clear_front_rows(int from, int to) {
	int r = 0;
	int c = 0;
	TextStyle normal = style_default();
	for (r = from; r < to; r++) {
		if (r < 0 || r >= g_prev.rows) {
			continue;
		}
		for (c = 0; c < g_prev.cols; c++) {
			DispCell *b = cell_at(&g_prev, r, c);
			if (b != NULL) {
				b.ch = ' ';
				b.style = normal;
			}
		}
	}
}

static void apply_hardware_scroll(Window *win, int old_top) {
	int scroll_n = 0;
	int height = 0;
	int top = 0;
	int abs_n = 0;
	int clear_from = 0;
	int clear_to = 0;
	int r = 0;
	TextStyle normal = style_default();
	if (win == NULL) {
		return;
	}
	height = win.height;
	top = win.screen_top_row;
	if (height <= 0) {
		return;
	}
	if (win.bottom_align) {
		return;
	}
	if (old_top == win.top_line) {
		return;
	}
	{
		int delta = old_top - win.top_line;
		if (delta < 0) {
			delta = -delta;
		}
		if (delta >= height) {
			return;
		}
		scroll_n = win.top_line - old_top;
	}
	if (scroll_n == 0 || scroll_n <= -height || scroll_n >= height) {
		return;
	}
	abs_n = scroll_n < 0 ? -scroll_n : scroll_n;
	if (abs_n <= 0 || abs_n >= height) {
		return;
	}
	if (scroll_n > 0) {
		clear_from = top + height - abs_n;
		clear_to = top + height;
		screen_rows_scroll_up(&g_screen, top, height, abs_n);
		screen_rows_scroll_up(&g_prev, top, height, abs_n);
	} else {
		clear_from = top;
		clear_to = top + abs_n;
		screen_rows_scroll_down(&g_screen, top, height, abs_n);
		screen_rows_scroll_down(&g_prev, top, height, abs_n);
	}
	clear_front_rows(clear_from, clear_to);
	for (r = clear_from; r < clear_to; r++) {
		if (g_screen.row_dirty != NULL && r >= 0 && r < g_screen.rows) {
			g_screen.row_dirty[r] = true;
		}
	}
	emit_sgr_cached(normal);
	term_scroll_rows(top, top + height - 1, scroll_n);
	for (r = clear_from; r < clear_to; r++) {
		term_move(r, 0);
		term_erase_eol();
	}
	term_reset_scroll_region();
	g_have_style = false;
}

static int content_row_offset(Window *win) {
	return window_content_row_offset(win);
}

static void adjust_hscroll(Window *win) {
	int gutter_w = 0;
	int cc = 0;
	int visible = 0;
	int margin = 0;
	int new_h = 0;
	Line *line = {0};
	if (win == NULL || win.buffer == NULL || win != window_current()) {
		return;
	}
	gutter_w = window_gutter_width(win);
	line = win.buffer.line(win.cursor.line);
	if (line == NULL) {
		return;
	}
	cc = (int)line.display_col_at_offset(win.cursor.offset);
	visible = term_cols() - gutter_w;
	if (visible < 1) {
		visible = 1;
	}
	margin = visible / 4;
	new_h = win.h_scroll;
	if (cc < new_h) {
		if (cc > margin) {
			new_h = cc - margin;
		} else {
			new_h = 0;
		}
	} else if (cc >= new_h + visible) {
		new_h = cc - visible + margin + 1;
	}
	if (new_h < 0) {
		new_h = 0;
	}
	if (new_h != win.h_scroll) {
		win.h_scroll = new_h;
		win.should_redraw = true;
	}
}

static void paint_window_body(Window *win) {
	int pad = 0;
	int r = 0;
	int line_number = 0;
	SelState sel = {0};
	if (win == NULL || win.buffer == NULL) {
		return;
	}
	sel_init(&sel, win);
	pad = content_row_offset(win);
	line_number = win.top_line;
	for (r = 0; r < win.height; r++) {
		int row = win.screen_top_row + r;
		if (row >= term_rows()) {
			break;
		}
		if (r < pad) {
			paint_blank_row(win, row);
			continue;
		}
		if (line_number <= win.buffer.line_count_i()) {
			paint_line_text(win, row, line_number, &sel);
			line_number++;
		} else {
			paint_blank_row(win, row);
		}
	}
}

static void paint_cursor_line(Window *win) {
	int pad = 0;
	int row = 0;
	SelState sel = {0};
	if (win == NULL || win.buffer == NULL) {
		return;
	}
	sel_init(&sel, win);
	pad = content_row_offset(win);
	row = win.screen_top_row + pad + (win.cursor.line - win.top_line);
	if (row < win.screen_top_row || row >= win.screen_top_row + win.height || row >= term_rows()) {
		return;
	}
	paint_line_text(win, row, win.cursor.line, &sel);
}

void display_init() {
	g_theme_light = false;
	g_echo[0] = 0;
	g_echo_generation = 0;
	g_screen_dirty = true;
	g_have_style = false;
	screen_resize(&g_screen, term_rows() + 1, term_cols());
	screen_resize(&g_prev, term_rows() + 1, term_cols());
}

void display_shutdown() {
	screen_free(&g_screen);
	screen_free(&g_prev);
}

void display_set_echo(const char[..] msg) {
	g_echo_generation++;
	if (len(msg) == 0 || ptr(msg) == NULL) {
		g_echo[0] = 0;
		return;
	}
	(void)snprintf(g_echo, sizeof(g_echo), "%s", msg);
}

const char *display_echo_text() {
	return g_echo;
}

uint64_t display_echo_generation() {
	return g_echo_generation;
}

// display_window_cursor_screen_col is the display column of the cursor in win (jem.go WindowCursorScreenCol).
int display_window_cursor_screen_col(Window *win) {
	Line *line = NULL;
	if (win == NULL || win.buffer == NULL) {
		return 0;
	}
	line = win.buffer.line(win.cursor.line);
	if (line == NULL) {
		return 0;
	}
	return (int)line.display_col_at_offset(win.cursor.offset);
}

// display_cursor_row is the 0-based screen row of the current window's cursor.
int display_cursor_row() {
	Window *cur = window_current();
	if (cur == NULL || cur.buffer == NULL) {
		return 0;
	}
	return cur.screen_top_row + content_row_offset(cur) + (cur.cursor.line - cur.top_line);
}

void display_invalidate() {
	size_t i = 0;
	g_screen_dirty = true;
	for (i = 0; i < window_active.count; i++) {
		Window *w = window_active.windows[i];
		if (w == NULL) {
			continue;
		}
		w.should_redraw = true;
		w.should_update_mode_line = true;
	}
}

void display_toggle_theme() {
	g_theme_light = !g_theme_light;
	g_have_style = false;
	display_invalidate();
}

// display_theme_mode returns 0 dark, 1 light (jem.go ThemeMode).
int display_theme_mode() {
	return g_theme_light ? 1 : 0;
}

void display_set_theme_mode(int mode) {
	bool light = mode != 0;
	if (g_theme_light == light) {
		return;
	}
	g_theme_light = light;
	g_have_style = false;
	display_invalidate();
}

void display_update() {
	size_t wi = 0;
	Window *cur = window_current();
	int cursor_row = 0;
	int cursor_col = 0;
	int total_rows = term_rows() + 1;

	if (term_refresh_size()) {
		window_retile();
		screen_resize(&g_screen, term_rows() + 1, term_cols());
		screen_resize(&g_prev, term_rows() + 1, term_cols());
		g_screen_dirty = true;
	} else {
		screen_resize(&g_screen, total_rows, term_cols());
		screen_resize(&g_prev, total_rows, term_cols());
	}

	if (g_screen_dirty) {
		size_t i = 0;
		for (i = 0; i < window_active.count; i++) {
			Window *w = window_active.windows[i];
			if (w == NULL) {
				continue;
			}
			w.should_redraw = true;
			w.should_update_mode_line = true;
		}
	}

	if (cur != NULL) {
		cur.should_update_mode_line = true;
	}

	for (wi = 0; wi < window_active.count; wi++) {
		Window *win = window_active.windows[wi];
		int old_top = 0;
		if (win == NULL || win.buffer == NULL) {
			continue;
		}
		if (!win.should_redraw && !win.did_move && !win.did_edit && !win.should_update_mode_line) {
			continue;
		}
		old_top = win.top_line;
		win.ensure_visible();
		adjust_hscroll(win);
		/* Region / search overlays need a full body paint each tick. */
		if ((win.mark.line != 0 || win.search_span_count > 0) && !win.should_redraw) {
			win.did_edit = false;
			win.did_move = false;
			win.should_redraw = true;
		}
		if (win.did_edit && !win.did_move && !win.should_redraw) {
			paint_cursor_line(win);
		} else if (win.did_edit || win.should_redraw) {
			apply_hardware_scroll(win, old_top);
			paint_window_body(win);
		}
		if (win.should_update_mode_line && !win.no_mode_line) {
			paint_modeline(win, win.screen_top_row + win.height);
		}
		win.should_redraw = false;
		win.did_move = false;
		win.did_edit = false;
		win.should_update_mode_line = false;
	}

	if (minibuffer_active()) {
		char mb_line[1024] = {0};
		minibuffer_format(mb_line, sizeof(mb_line));
		snprintf(g_echo, sizeof(g_echo), "%s", mb_line);
	}
	paint_echo(g_screen.rows - 1);

	/* Match jem.c / jem.go: never hide-cursor around ordinary paints.
	 * jem.mc used to emit \x1b[?25l every dirty frame; a torn write left
	 * "?25l" as literal text that then needed a forced redraw to clear. */
	screen_sync();

	if (cur != NULL && cur.buffer != NULL) {
		Line *line = cur.buffer.line(cur.cursor.line);
		cursor_row = cur.screen_top_row + content_row_offset(cur) + (cur.cursor.line - cur.top_line);
		if (line != NULL) {
			int gutter_w = window_gutter_width(cur);
			cursor_col = gutter_w + (int)line.display_col_at_offset(cur.cursor.offset) - cur.h_scroll;
			if (cursor_col < gutter_w) {
				cursor_col = gutter_w;
			}
		} else {
			cursor_col = window_gutter_width(cur);
		}
		if (cursor_col >= term_cols()) {
			cursor_col = term_cols() - 1;
		}
		if (cursor_row >= 0 && cursor_row < term_rows()) {
			emit_sgr_cached(style_default());
			term_move(cursor_row, cursor_col);
		}
	}
	term_flush();
}

/* ---- mouse hit testing (jem.go/display/mouse_hit.go) ---- */

// display_win_at returns the window occupying screen row r, or NULL.
Window *display_win_at(int r) {
	size_t i = 0;
	for (i = 0; i < window_active.count; i++) {
		Window *win = window_active.windows[i];
		if (win != NULL && r >= win.screen_top_row && r < win.screen_top_row + win.height) {
			return win;
		}
	}
	return NULL;
}

// display_mouse_loc_in_window maps term mouse col/row into a buffer location in win.
Location display_mouse_loc_in_window(Window *win) {
	Location loc = {0};
	int row = term_mouse_row();
	int col = term_mouse_col();
	int pad = 0;
	int gutter = 0;
	int row_in = 0;
	int bline = 0;
	int text_col = 0;
	int nlines = 0;
	Line *line = NULL;
	loc.line = 1;
	loc.offset = 0;
	if (win == NULL || win.buffer == NULL) {
		return loc;
	}
	pad = window_content_row_offset(win);
	gutter = window_gutter_width(win);
	row_in = row - win.screen_top_row - pad;
	if (row_in < 0) {
		row_in = 0;
	}
	nlines = win.buffer.line_count_i();
	if (nlines < 1) {
		nlines = 1;
	}
	bline = win.top_line + row_in;
	if (bline < 1) {
		bline = 1;
	}
	if (bline > nlines) {
		bline = nlines;
	}
	text_col = col - gutter + win.h_scroll;
	if (text_col < 0) {
		text_col = 0;
	}
	line = win.buffer.line(bline);
	loc.line = bline;
	loc.offset = line != NULL ? line.offset_at_display_col(text_col) : 0;
	return loc;
}


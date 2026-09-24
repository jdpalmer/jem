// term — Kitty decode, screen escapes (host TTY via std tty/fs/os).
import "event";
import "fs";
import "tty";
import "os";
import "str";

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

enum TermKeys {
	TermCTL = 0x01000000,
	TermMETA = 0x02000000,
	TermCTLX = 0x04000000,
	TermSHIFT = 0x08000000,
	TermKeyMask = (int)TermCTL | (int)TermMETA | (int)TermCTLX |
		(int)TermSHIFT,
	TermKeyUp = 0x10000001,
	TermKeyDown = 0x10000002,
	TermKeyLeft = 0x10000003,
	TermKeyRight = 0x10000004,
	TermKeyTab = 0x10000005,
	TermKeyEnter = 0x10000006,
	TermKeyHome = 0x10000007,
	TermKeyEnd = 0x10000008,
	TermKeyPageUp = 0x10000009,
	TermKeyPageDown = 0x1000000A,
	TermKeyDelete = 0x1000000B,
	TermMouseLeft = 0x20000001,
	TermMouseWheelUp = 0x20000002,
	TermMouseWheelDown = 0x20000003,
	TermMouseDrag = 0x20000004,
	TermKittyKeyLeft = 57351,
	TermKittyKeyUp = 57352,
	TermKittyKeyDown = 57353,
	TermKittyKeyRight = 57354,
	TermKittyKeyHome = 57358,
	TermKittyKeyEnd = 57359,
	TermKittyKeyPageUp = 57361,
	TermKittyKeyPageDown = 57362,
	TermKittyKeyDelete = 57363,
	TermKeyPasteComplete = 0x00FF0002,
};

enum TermBufLimits {
	TermOutCap = 8192,
	TermInCap = 4096
};

uint32_t term_key_value(TermKeys key) {
	return key;
}

static int g_rows = 24;
static int g_cols = 80;
static int g_scroll_top = -1;
static int g_scroll_bottom = -1;
static File g_fin = {
	0 };
static File g_fout = {
	0 };
static bool g_tty_std_ok;
static bool g_is_tty;
static bool g_tty_raw;
static bool g_altbuf;
static bool g_kitty;
static bool g_open;
static int g_mouse_col;
static int g_mouse_row;

static char g_obuf[TermOutCap];
static size_t g_olen;

static char g_ibuf[TermInCap];
static int g_ilen;
static int g_ipos;

static int term_write_out(const char *buf, size_t n) {
	size_t left = n;
	const char *p = buf;
	if (!g_tty_std_ok || buf == NULL || n == 0) {
		return n == 0 ? 0 : -1;
	}
	while (left > 0) {
		auto (wn, ok) = g_fout.write(str_from_bytes(p, left));
		if (!ok) {
			return -1;
		}
		if (wn == 0) {
			break;
		}
		p = p + (size_t)wn;
		left = left - (size_t)wn;
	}
	return (int)(n - left);
}

static bool term_wait_readable(int timeout_ms) {
	Poll pol = {
		0 };
	if (!g_tty_std_ok || file_fd(&g_fin) < 0) {
		return false;
	}
	pol.init();
	if (!pol.add_file(&g_fin, PollIn)) {
		pol.close();
		return false;
	}
	{
		auto (n, ok) = pol.wait(timeout_ms);
		if (!ok || n <= 0) {
			pol.close();
			return false;
		}
		if (pol.ready_count() <= 0) {
			pol.close();
			return false;
		}
		auto (rf, rev) = pol.ready_file(0);
		(void)rf;
		pol.close();
		return (rev & (PollIn | PollHup | PollErr)) != 0;
	}
}

static bool term_query_size(int *rows, int *cols) {
	if (!g_tty_std_ok) {
		return false;
	}
	auto (wz, ok) = tty_size(&g_fin);
	if (!ok || wz.rows < 2 || wz.cols < 1) {
		return false;
	}
	if (rows != NULL) {
		*rows = wz.rows;
	}
	if (cols != NULL) {
		*cols = wz.cols;
	}
	return true;
}

static void term_buf_reset() {
	g_olen = 0;
}

static void term_buf_flush() {
	if (g_olen == 0) {
		return;
	}
	term_write_out(g_obuf, g_olen);
	g_olen = 0;
}

static void term_buf_write(const char *buf, size_t n) {
	if (buf == NULL || n == 0) {
		return;
	}
	if (g_olen + n >= (size_t)TermOutCap) {
		term_buf_flush();
	}
	if (n >= (size_t)TermOutCap) {
		term_write_out(buf, n);
		return;
	}
	memcpy(g_obuf + g_olen, buf, n);
	g_olen = g_olen + n;
}

static void term_in_reset() {
	g_ilen = 0;
	g_ipos = 0;
}

static int term_in_append_read(int maxn) {
	if (maxn > (int)TermInCap) {
		maxn = TermInCap;
	}
	if (!g_tty_std_ok || file_fd(&g_fin) < 0) {
		return -1;
	}
	auto (n, ok) = g_fin.read(ranged(g_ibuf, (size_t)maxn));
	if (!ok) {
		g_ilen = 0;
		g_ipos = 0;
		return -1;
	}
	g_ilen = (int)n;
	g_ipos = 0;
	return (int)n;
}

static void term_in_unshift(char b) {
	if (g_ipos > 0) {
		g_ipos--;
		g_ibuf[g_ipos] = b;
		return;
	}
	if (g_ilen + 1 > (int)TermInCap) {
		return;
	}
	memmove(g_ibuf + 1, g_ibuf, g_ilen);
	g_ibuf[0] = b;
	g_ilen++;
}

void term_out_flush() {
	term_buf_flush();
}

void term_emit(const char *s, size_t n) {
	if (s == NULL || n == 0) {
		return;
	}
	term_buf_write(s, n);
}

void term_emit_cstr(const char[..] s) {
	if (len(s) == 0 || ptr(s) == NULL) {
		return;
	}
	term_emit(ptr(s), len(s));
}

void term_emit_u32(uint32_t v) {
	char buf[16] = {0};
	int n = snprintf(buf, sizeof(buf), "%u", v);
	if (n > 0) {
		term_emit(buf, n);
	}
}

static void term_emit_csi(int first, bool has_second, int second, char final_b) {
	char buf[16] = {0};
	int n = 0;
	term_emit_cstr("\x1b[");
	n = snprintf(buf, sizeof(buf), "%u", (unsigned)first);
	if (n > 0) {
		term_emit(buf, n);
	}
	if (has_second) {
		term_emit_cstr(";");
		n = snprintf(buf, sizeof(buf), "%u", (unsigned)second);
		if (n > 0) {
			term_emit(buf, n);
		}
	}
	term_emit(&final_b, 1);
}

static bool term_fill_in(int timeout_ms) {
	int n = 0;
	if (g_ipos < g_ilen) {
		return true;
	}
	term_in_reset();
	if (timeout_ms != 0) {
		if (!term_wait_readable(timeout_ms)) {
			return false;
		}
	}
	n = term_in_append_read(TermInCap);
	if (n <= 0) {
		return false;
	}
	return true;
}

(char, bool) term_read_byte(int timeout_ms) {
	char b = 0;
	if (g_ipos >= g_ilen) {
		if (!term_fill_in(timeout_ms)) {
			return (0, false);
		}
	}
	b = g_ibuf[g_ipos];
	g_ipos++;
	return (b, true);
}

static void term_unread_byte(char b) {
	term_in_unshift(b);
}

static uint32_t csi_modifier_flags(int modifier_param) {
	int bits = 0;
	uint32_t flags = 0;
	if (modifier_param > 0) {
		bits = modifier_param - 1;
	}
	if (bits & 1) {
		flags |= term_key_value(TermSHIFT);
	}
	if (bits & 2) {
		flags |= term_key_value(TermMETA);
	}
	if (bits & 4) {
		flags |= term_key_value(TermCTL);
	}
	return flags;
}

(uint32_t, bool) cursor_key_code(int final_b, uint32_t modifiers) {
	uint32_t base = 0;
	switch (final_b) {
	case 'A':
		base = TermKeyUp;
		break;
	case 'B':
		base = TermKeyDown;
		break;
	case 'C':
		base = TermKeyRight;
		break;
	case 'D':
		base = TermKeyLeft;
		break;
	case 'H':
		base = TermKeyHome;
		break;
	case 'F':
		base = TermKeyEnd;
		break;
	default:
		return (0, false);
	}
	return (base | modifiers, true);
}

static uint32_t csi_u_key_code(int codepoint, int modifier_param, int event_type) {
	uint32_t modifiers = 0;
	uint32_t res = 0;
	if (event_type == 3) {
		return 0;
	}
	if (modifier_param > 0) {
		modifiers = csi_modifier_flags(modifier_param);
	}
	switch (codepoint) {
	case 27:
		return 27 | modifiers;
	case 13:
		return term_key_value(TermKeyEnter) | (modifiers &
			(term_key_value(TermCTL) | term_key_value(TermSHIFT) | term_key_value(TermMETA)));
	case 10:
		return term_key_value(TermCTL) | (uint32_t)'J';
	case 9:
		if ((modifiers & term_key_value(TermSHIFT)) != 0) {
			return term_key_value(TermSHIFT) | term_key_value(TermKeyTab);
		}
		return TermKeyTab;
	case 127:
		return term_key_value(TermCTL) | (uint32_t)'H';
	case 32:
		if ((modifiers & term_key_value(TermCTL)) != 0) {
			return term_key_value(TermCTL) | (uint32_t)' ';
		}
		return 32 | modifiers;
	case TermKittyKeyLeft:
		return term_key_value(TermKeyLeft) | modifiers;
	case TermKittyKeyUp:
		return term_key_value(TermKeyUp) | modifiers;
	case TermKittyKeyDown:
		return term_key_value(TermKeyDown) | modifiers;
	case TermKittyKeyRight:
		return term_key_value(TermKeyRight) | modifiers;
	case TermKittyKeyHome:
		return term_key_value(TermKeyHome) | modifiers;
	case TermKittyKeyEnd:
		return term_key_value(TermKeyEnd) | modifiers;
	case TermKittyKeyPageUp:
		return term_key_value(TermKeyPageUp) | modifiers;
	case TermKittyKeyPageDown:
		return term_key_value(TermKeyPageDown) | modifiers;
	case TermKittyKeyDelete:
		return term_key_value(TermKeyDelete) | modifiers;
	default:
		break;
	}
	if (codepoint >= 32 && codepoint < 128) {
		modifiers &= ~term_key_value(TermSHIFT);
		if ((modifiers & term_key_value(TermCTL)) != 0 &&
			(modifiers & term_key_value(TermMETA)) == 0) {
			if (codepoint >= 'a' && codepoint <= 'z') {
				codepoint = codepoint - ('a' - 'A');
			}
			return term_key_value(TermCTL) | (uint32_t)codepoint;
		}
		if ((modifiers & term_key_value(TermMETA)) != 0) {
			/* Match Esc-as-Meta: M-x is META|'X' (bindings use uppercase). */
			if (codepoint >= 'a' && codepoint <= 'z') {
				codepoint = codepoint - ('a' - 'A');
			}
			return (uint32_t)codepoint | term_key_value(TermMETA);
		}
		return codepoint;
	}
	if (codepoint >= 128 && codepoint < 0x110000) {
		modifiers &= ~term_key_value(TermSHIFT);
		return (uint32_t)codepoint | modifiers;
	}
	return res;
}

static bool term_probe_kitty() {
	term_write_out("\x1b[?u", 4);
	auto (b, ok) = term_read_byte(1000);
	if (!ok || b != 0x1B) {
		return false;
	}
	auto (b1, ok1) = term_read_byte(100);
	if (!ok1 || b1 != '[') {
		return false;
	}
	auto (b2, ok2) = term_read_byte(100);
	if (!ok2 || b2 != '?') {
		return false;
	}
	for (;;) {
		auto (bx, okx) = term_read_byte(100);
		if (!okx) {
			return false;
		}
		if (bx >= '0' && bx <= '9') {
			continue;
		}
		return bx == 'u';
	}
}

static void term_enable_modes() {
	term_emit_cstr("\x1b[?1049h\x1b[>1u\x1b[?2004h\x1b[?1000h\x1b[?1002h\x1b[?1006h\x1b[?2048h");
	term_out_flush();
	g_altbuf = true;
	term_emit_cstr("\x1b[?25h\x1b[?2q");
	term_out_flush();
}

// term_open enters raw mode and probes Kitty keyboard support.
bool term_open() {
	int rows = 0;
	int cols = 0;
	bool fin_ok = false;
	bool fout_ok = false;
	g_tty_raw = false;
	g_tty_std_ok = false;
	term_buf_reset();
	term_in_reset();
	{
		auto (fin, ok) = tty_stdin();
		g_fin = fin;
		fin_ok = ok;
	}
	{
		auto (fout, ok) = tty_stdout();
		g_fout = fout;
		fout_ok = ok;
	}
	g_tty_std_ok = fin_ok && fout_ok;
	if (!g_tty_std_ok || !tty_isatty(&g_fin)) {
		fprintf(stderr, "jem: not a TTY; continuing with default size 24x80\n");
		g_rows = 24;
		g_cols = 80;
		g_is_tty = false;
		g_open = true;
		return true;
	}
	g_is_tty = true;
	if (!term_query_size(&rows, &cols)) {
		fprintf(stderr, "jem: could not get terminal size; using 24x80\n");
		rows = 25;
		cols = 80;
	}
	g_rows = rows - 1;
	g_cols = cols;
	if (!tty_set_raw(&g_fin)) {
		fprintf(stderr, "jem: failed to set raw mode\n");
		g_is_tty = false;
		g_open = true;
		return true;
	}
	g_tty_raw = true;
	if (!term_probe_kitty()) {
		tty_set_cooked(&g_fin);
		g_tty_raw = false;
		fprintf(stderr, "jem: this terminal does not support the Kitty keyboard protocol.\n");
		fprintf(stderr, "jem: please use Kitty, WezTerm, Ghostty, foot, or Windows Terminal.\n");
		return false;
	}
	g_kitty = true;
	term_enable_modes();
	g_open = true;
	return true;
}

// term_close restores the terminal.
void term_close() {
	if (!g_open) {
		return;
	}
	term_emit_cstr("\x1b[?25h");
	term_emit_cstr("\x1b[<u\x1b[?2048l\x1b[?2004l\x1b[?1006l\x1b[?1002l\x1b[?1000l");
	if (g_altbuf) {
		term_emit_cstr("\x1b[?1049l");
		g_altbuf = false;
	}
	term_out_flush();
	if (g_is_tty && g_tty_raw) {
		tty_set_cooked(&g_fin);
		g_tty_raw = false;
	}
	g_open = false;
}

int term_rows() {
	return g_rows;
}

int term_cols() {
	return g_cols;
}

bool term_refresh_size() {
	int rows = 0;
	int cols = 0;
	int new_rows = 0;
	if (!g_is_tty) {
		return false;
	}
	if (!term_query_size(&rows, &cols)) {
		return false;
	}
	new_rows = rows - 1;
	if (new_rows == g_rows && cols == g_cols) {
		return false;
	}
	g_rows = new_rows;
	g_cols = cols;
	g_scroll_top = -1;
	g_scroll_bottom = -1;
	return true;
}

void term_flush() {
	term_out_flush();
}

void term_hide_cursor() {
	term_emit_cstr("\x1b[?25l");
}

void term_show_cursor() {
	term_emit_cstr("\x1b[?25h");
}

void term_move(int row, int col) {
	term_emit_csi(row + 1, true, col + 1, 'H');
}

void term_erase_eol() {
	term_emit_cstr("\x1b[K");
}

void term_erase_eos() {
	term_reset_scroll_region();
	term_emit_cstr("\x1b[J");
}

void term_set_scroll_region(int top, int bottom) {
	if (g_scroll_top == top && g_scroll_bottom == bottom) {
		return;
	}
	term_emit_csi(top + 1, true, bottom + 1, 'r');
	g_scroll_top = top;
	g_scroll_bottom = bottom;
}

void term_reset_scroll_region() {
	int rows = term_rows();
	if (g_scroll_top == 0 && g_scroll_bottom == rows) {
		return;
	}
	term_set_scroll_region(0, rows);
}

/* Scroll [top, bottom] inclusive. Positive row_count scrolls content up. */
void term_scroll_rows(int top, int bottom, int row_count) {
	int abs_n = row_count;
	int height = bottom - top + 1;
	if (abs_n < 0) {
		abs_n = -abs_n;
	}
	if (abs_n <= 0 || abs_n >= height) {
		return;
	}
	term_set_scroll_region(top, bottom);
	if (row_count > 0) {
		term_emit_csi(abs_n, false, 0, 'S');
	} else {
		term_emit_csi(abs_n, false, 0, 'T');
	}
}

void term_write(const char *s, size_t n) {
	term_emit(s, n);
}

void term_write_cstr(const char[..] s) {
	term_emit_cstr(s);
}

void term_beep() {
	term_emit_cstr("\a");
	term_out_flush();
}

int term_mouse_col() {
	return g_mouse_col;
}

int term_mouse_row() {
	return g_mouse_row;
}

static bool term_paste_terminator(char *data, size_t n) {
	char terminator[6] = {0x1B, '[', '2', '0', '1', '~'};
	return data != NULL && n >= sizeof(terminator) &&
		memcmp(data + n - sizeof(terminator), terminator,
			sizeof(terminator)) == 0;
}

static void term_read_bracketed_paste() {
	enum { PasteMax = 64 * 1024 };
	char *data = malloc((size_t)PasteMax + 6);
	char recent[6] = {0};
	size_t used = 0;
	size_t recent_len = 0;
	bool truncated = false;
	if (data == NULL) {
		return;
	}
	for (;;) {
		auto (byte, ok) = term_read_byte(-1);
		if (!ok) {
			free(data);
			return;
		}
		if (recent_len < sizeof(recent)) {
			recent[recent_len++] = byte;
		} else {
			memmove(recent, recent + 1, sizeof(recent) - 1);
			recent[sizeof(recent) - 1] = byte;
		}
		if (truncated) {
			if (term_paste_terminator(recent, recent_len)) {
				return;
			}
			continue;
		}
		data[used++] = byte;
		if (term_paste_terminator(data, used)) {
			size_t payload_len = used - 6;
			if (payload_len > 0) {
				event_enqueue(event_paste(data, payload_len));
				data = NULL;
			}
			free(data);
			return;
		}
		if (used > (size_t)PasteMax) {
			event_enqueue(event_paste(data, PasteMax));
			data = NULL;
			truncated = true;
		}
	}
}

static uint32_t term_decode_csi() {
	int params[8] = {0};
	int nparams = 0;
	int val = 0;
	bool in_num = false;
	bool is_u = false;
	bool is_mouse = false;
	int i = 0;

	for (i = 0; i < 64; i++) {
		auto (b, ok) = term_read_byte(50);
		if (!ok) {
			return 0;
		}
		if (b >= '0' && b <= '9') {
			val = val * 10 + (b - '0');
			in_num = true;
			continue;
		}
		if (b == ';' || b == ':') {
			if (nparams < 8) {
				params[nparams++] = in_num ? val : 0;
			}
			val = 0;
			in_num = false;
			continue;
		}
		if (b == '<') {
			is_mouse = true;
			continue;
		}
		if (b == 'u') {
			is_u = true;
			if (in_num && nparams < 8) {
				params[nparams++] = val;
			}
			break;
		}
		if (b >= 0x40 && b <= 0x7E) {
			if (in_num && nparams < 8) {
				params[nparams++] = val;
			}
			if (is_mouse && (b == 'M' || b == 'm') && nparams >= 3) {
				int button = params[0];
				g_mouse_col = params[1] - 1;
				g_mouse_row = params[2] - 1;
				event_enqueue(event_mouse(g_mouse_col, g_mouse_row));
				if (b == 'm') {
					return 0;
				}
				if (button == 64) {
					return TermMouseWheelUp;
				}
				if (button == 65) {
					return TermMouseWheelDown;
				}
				if (button == 32) {
					return TermMouseDrag;
				}
				if (button == 0) {
					return TermMouseLeft;
				}
				return 0;
			}
			if (b == 'A' || b == 'B' || b == 'C' || b == 'D' || b == 'H' || b == 'F') {
				uint32_t mod = 0;
				if (nparams >= 2) {
					mod = csi_modifier_flags(params[1]);
				} else if (nparams == 1 && params[0] > 1) {
					mod = csi_modifier_flags(params[0]);
				}
				auto (k, kok) = cursor_key_code(b, mod);
				return kok ? k : 0;
			}
			if (b == '~' && nparams >= 1) {
				if (params[0] == 200) {
					term_read_bracketed_paste();
					return TermKeyPasteComplete;
				}
				uint32_t mod = nparams >= 2 ? csi_modifier_flags(params[1]) : 0;
				switch (params[0]) {
				case 1:
				case 7:
					return term_key_value(TermKeyHome) | mod;
				case 3:
					return term_key_value(TermKeyDelete) | mod;
				case 4:
				case 8:
					return term_key_value(TermKeyEnd) | mod;
				case 5:
					return term_key_value(TermKeyPageUp) | mod;
				case 6:
					return term_key_value(TermKeyPageDown) | mod;
				default:
					return 0;
				}
			}
			if (b == 't' && nparams >= 1 && params[0] == 48) {
				event_enqueue(event_resize());
				return 0;
			}
			return 0;
		}
	}
	if (is_u) {
		int cp = nparams >= 1 ? params[0] : 0;
		int mod = nparams >= 2 ? params[1] : 1;
		int ev = nparams >= 3 ? params[2] : 1;
		return csi_u_key_code(cp, mod, ev);
	}
	return 0;
}

static uint32_t term_decode_utf8(char first) {
	char buf[4] = {0};
	int need = 1;
	int i = 1;
	uint32_t r = 0;
	buf[0] = first;
	if ((first & 0xE0) == 0xC0) {
		need = 2;
	} else if ((first & 0xF0) == 0xE0) {
		need = 3;
	} else if ((first & 0xF8) == 0xF0) {
		need = 4;
	} else {
		return first;
	}
	for (i = 1; i < need; i++) {
		auto (bx, okx) = term_read_byte(50);
		if (!okx) {
			return 0;
		}
		buf[i] = bx;
	}
	if (need == 2) {
		r = (uint32_t)(first & 0x1F) << 6;
		r |= buf[1] & 0x3F;
	} else if (need == 3) {
		r = (uint32_t)(first & 0x0F) << 12;
		r |= (uint32_t)(buf[1] & 0x3F) << 6;
		r |= buf[2] & 0x3F;
	} else {
		r = (uint32_t)(first & 0x07) << 18;
		r |= (uint32_t)(buf[1] & 0x3F) << 12;
		r |= (uint32_t)(buf[2] & 0x3F) << 6;
		r |= buf[3] & 0x3F;
	}
	return r;
}

// term_read_key blocks for one decoded key.
(uint32_t, bool) term_read_key() {
	for (;;) {
		auto (b, ok) = term_read_byte(-1);
		char first = b;
		if (!ok) {
			return (0, false);
		}
		if (first >= 0x80) {
			uint32_t k = term_decode_utf8(first);
			if (k != 0) {
				return (k, true);
			}
			continue;
		}
		if (first != 0x1B) {
			return (first, true);
		}
		auto (b2, ok2) = term_read_byte(50);
		if (!ok2) {
			return (0x1B, true);
		}
		if (b2 == '[') {
			uint32_t k = term_decode_csi();
			if (k != 0) {
				return (k, true);
			}
			continue;
		}
		if (b2 == 'O') {
			auto (b3, ok3) = term_read_byte(50);
			if (!ok3) {
				return (0, false);
			}
			auto (k, kok) = cursor_key_code(b3, 0);
			if (kok) {
				return (k, true);
			}
			continue;
		}
		if (b2 >= 0x20 && b2 < 0x7F) {
			term_unread_byte(b2);
			return (0x1B, true);
		}
	}
}

// term_try_read_key polls with timeout_ms.
(uint32_t, bool) term_try_read_key(int timeout_ms) {
	if (g_ipos >= g_ilen) {
		if (!term_wait_readable(timeout_ms) && g_ipos >= g_ilen) {
			return (0, false);
		}
	}
	return term_read_key();
}


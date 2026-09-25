// buffer — document model, line storage, undo, UTF-8 helpers.
import "syntax";
import "str";

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

enum EolMode {
	EModeLF = 0,
	EModeCRLF = 1
};

struct IndentConfig {
	int width;
	int brace;
	int label;
	int continued;
};

struct Location {
	int line;
	size_t offset;
};

struct Line {
	char[..] data; /* owned heap */
	SynState syntax_end;
	bool syntax_valid;
	TextStyle *styles;
	size_t styles_len;
	bool cache_valid;
	LangMode lang_mode;
	/* Owned blob (free'd in line_free). Tools store jump targets here. */
	void *metadata;
};

struct Buffer {
	Line[..] lines; /* owned heap */
	uint32_t serial;
	uint32_t saved_undo_serial;
	bool is_changed;
	bool is_readonly;
	EolMode eol_mode;
	LangMode lang_mode;
	int fill_col;
	IndentConfig indent;
	bool whitespace_cleanup;
	char[..] name; /* owned heap */
	char[..] file_name; /* owned heap */
	int64_t edit_time;
	int64_t access_time;
	Location cursor;
	Location mark;
};

enum UndoKind {
	UndoDelete = 0,
	UndoInsert = 1
};

struct UndoRecord {
	UndoKind kind;
	int line_num;
	size_t offset;
	Location end;
	char[..] text; /* owned heap */
};

enum UndoLimits {
	UndoHistoryMax = 64
};

struct UndoGroup {
	Buffer *buffer;
	uint32_t buffer_serial;
	uint32_t group_serial;
	Location before;
	UndoRecord[..] records; /* owned heap */
};

struct UndoHistory {
	UndoGroup groups[UndoHistoryMax];
	UndoGroup pending;
	uint32_t next_group_serial;
	uint8_t count;
	bool is_replaying;
};

Buffer *[..] buffer_all; /* owned heap of Buffer*; Buffers owned separately */
static Buffer *g_buffer_cur;
UndoHistory buffer_undo;

static char ScratchName[] = "*scratch*";

// utf8_next_offset returns the next rune start after offset.
size_t utf8_next_offset(const char[..] data, size_t offset) {
	size_t size = 1;
	if (offset >= len(data)) {
		return offset;
	}
	char c = data[offset];
	if (c < 0x80) {
		size = 1;
	} else if ((c & 0xE0) == 0xC0) {
		size = 2;
	} else if ((c & 0xF0) == 0xE0) {
		size = 3;
	} else if ((c & 0xF8) == 0xF0) {
		size = 4;
	}
	if (offset + size > len(data)) {
		return offset + 1;
	}
	return offset + size;
}

// utf8_prev_offset returns the start of the rune before offset.
size_t utf8_prev_offset(const char[..] data, size_t offset) {
	size_t i = 0;
	if (offset == 0 || len(data) == 0) {
		return 0;
	}
	i = offset - 1;
	if (i >= len(data)) {
		i = len(data) - 1;
	}
	while (i > 0 && (data[i] & 0xC0) == 0x80) {
		i--;
	}
	return i;
}

// utf8_decode_rune reads one rune at offset; returns (rune, size).
(uint32_t, size_t) utf8_decode_rune(const char[..] data, size_t offset) {
	size_t n = len(data);
	if (offset >= n) {
		return (0, 0);
	}
	char c = data[offset];
	uint32_t r = 0;
	if (c < 0x80) {
		return (c, 1);
	}
	if ((c & 0xE0) == 0xC0 && offset + 1 < n) {
		r = (uint32_t)(c & 0x1F) << 6;
		r |= data[offset + 1] & 0x3F;
		return (r, 2);
	}
	if ((c & 0xF0) == 0xE0 && offset + 2 < n) {
		r = (uint32_t)(c & 0x0F) << 12;
		r |= (uint32_t)(data[offset + 1] & 0x3F) << 6;
		r |= data[offset + 2] & 0x3F;
		return (r, 3);
	}
	if ((c & 0xF8) == 0xF0 && offset + 3 < n) {
		r = (uint32_t)(c & 0x07) << 18;
		r |= (uint32_t)(data[offset + 1] & 0x3F) << 12;
		r |= (uint32_t)(data[offset + 2] & 0x3F) << 6;
		r |= data[offset + 3] & 0x3F;
		return (r, 4);
	}
	return (c, 1);
}

// rune_display_width returns columns for r (ASCII/CJK heuristic).
int rune_display_width(uint32_t r) {
	if (r == 0 || r < 32 || (r >= 0x7F && r < 0xA0)) {
		return 0;
	}
	if (r == (uint32_t)'\t') {
		return 1;
	}
	if (r >= 0x1100 &&
		((r <= 0x115F) || (r >= 0x2E80 && r <= 0xA4CF) ||
			(r >= 0xAC00 && r <= 0xD7A3) ||
			(r >= 0xF900 && r <= 0xFAFF) ||
			(r >= 0xFE10 && r <= 0xFE6F) ||
			(r >= 0xFF00 && r <= 0xFF60) ||
			(r >= 0xFFE0 && r <= 0xFFE6) ||
			(r >= 0x20000 && r <= 0x3FFFD))) {
		return 2;
	}
	return 1;
}

// view returns a non-owning slice of line bytes.
char[..] (Line *line).view() {
	return line.data;
}

size_t (Line *line).byte_len() {
	return len(line.data);
}

// first_nonblank returns the byte index of the first non-space/tab (jem.go Line.FirstNonblank).
size_t (Line *line).first_nonblank() {
	char[..] v = line.view();
	size_t i = 0;
	char *vp = ptr(v);
	while (i < len(v)) {
		char c = vp[i];
		if (c != ' ' && c != '\t') {
			return i;
		}
		i++;
	}
	return len(v);
}

bool (Line *line).is_blank() {
	return line.first_nonblank() >= line.byte_len();
}

size_t (Line *line).indent_column() {
	return line.display_col_at_offset(line.first_nonblank());
}

char (Line *line).last_byte() {
	size_t n = line.byte_len();
	if (n == 0) {
		return 0;
	}
	return line.data[n - 1];
}

char (Line *line).first_byte() {
	size_t i = line.first_nonblank();
	if (i >= line.byte_len()) {
		return 0;
	}
	return line.data[i];
}

void (Line *line).invalidate() {
	line.syntax_valid = false;
	line.cache_valid = false;
	free(line.styles);
	line.styles = NULL;
	line.styles_len = 0;
}

size_t (Line *line).display_col_at_offset(size_t offset) {
	char[..] v = line.view();
	size_t i = 0;
	size_t col = 0;
	if (offset > len(v)) {
		offset = len(v);
	}
	while (i < offset) {
		auto (r, sz) = utf8_decode_rune(v, i);
		if (sz == 0) {
			break;
		}
		if (r == (uint32_t)'\t') {
			col = col + (8 - (col % 8));
		} else {
			col = col + rune_display_width(r);
		}
		i = i + sz;
	}
	return col;
}

size_t (Line *line).offset_at_display_col(size_t goal) {
	char[..] v = line.view();
	size_t i = 0;
	size_t col = 0;
	size_t w = 0;
	if (goal == 0) {
		return 0;
	}
	while (i < len(v)) {
		auto (r, sz) = utf8_decode_rune(v, i);
		if (sz == 0) {
			break;
		}
		if (r == (uint32_t)'\t') {
			w = 8 - (col % 8);
		} else {
			w = rune_display_width(r);
			if (w == 0) {
				w = 1;
			}
		}
		if (col + w > goal) {
			return i;
		}
		col = col + w;
		i = i + sz;
	}
	return len(v);
}

static void line_free(Line *line) {
	if (line == NULL) {
		return;
	}
	str_free(&line.data);
	free(line.styles);
	line.styles = NULL;
	free(line.metadata);
	line.metadata = NULL;
}

bool (Buffer *buf).is_scratch() {
	if (len(buf.name) == 0) {
		return false;
	}
	return str_eq(buf.name, ScratchName);
}

void (Buffer *buf).mark_changed() {
	if (buf.is_scratch()) {
		return;
	}
	buf.is_changed = true;
}

static bool buffer_reserve_lines(Buffer *buf, size_t need) {
	Line *p = {0};
	size_t ncap = {0};
	if (need <= cap(buf.lines)) {
		return true;
	}
	ncap = cap(buf.lines) != 0 ? cap(buf.lines) : 8;
	while (ncap < need) {
		ncap = ncap * 2;
	}
	p = realloc(ptr(buf.lines), ncap * sizeof(Line));
	if (p == NULL) {
		return false;
	}
	buf.lines = ranged(p, len(buf.lines), ncap);
	return true;
}

void (Buffer *buf).ensure_min_lines() {
	Line empty = {0};
	if (len(buf.lines) != 0) {
		return;
	}
	if (!buffer_reserve_lines(buf, 1)) {
		return;
	}
	empty.lang_mode = buf.lang_mode;
	buf.lines = ranged(ptr(buf.lines), 1, cap(buf.lines));
	buf.lines[0] = empty;
}

// buffer_new allocates an empty buffer with one blank line.
Buffer *buffer_new() {
	Buffer *buf = calloc(1, sizeof(Buffer));
	if (buf == NULL) {
		return NULL;
	}
	buf.eol_mode = EModeLF;
	buf.lang_mode = LModeNone;
	buf.indent.width = 2;
	buf.fill_col = 80;
	buf.cursor.line = 1;
	buf.cursor.offset = 0;
	buf.mark.line = 0;
	buf.ensure_min_lines();
	return buf;
}

void (Buffer *buf).free() {
	for (auto *line : buf.lines) {
		line_free(line);
	}
	free(ptr(buf.lines));
	str_free(&buf.name);
	str_free(&buf.file_name);
	free(buf);
}

Line *(Buffer *buf).line(int line_num) {
	if (line_num < 1 || (size_t)line_num > len(buf.lines)) {
		return NULL;
	}
	return &buf.lines[line_num - 1];
}

int (Buffer *buf).line_count_i() {
	return (int)len(buf.lines);
}

int (Buffer *buf).eof_line() {
	return buf.line_count_i() + 1;
}

static bool buffers_reserve(size_t need) {
	Buffer **p = {0};
	size_t ncap = {0};
	if (need <= cap(buffer_all)) {
		return true;
	}
	ncap = cap(buffer_all) != 0 ? cap(buffer_all) : 4;
	while (ncap < need) {
		ncap = ncap * 2;
	}
	p = realloc(ptr(buffer_all), ncap * sizeof(Buffer *));
	if (p == NULL) {
		return false;
	}
	buffer_all = ranged(p, len(buffer_all), ncap);
	return true;
}

void buffer_list_init() {
	Buffer *scratch = {0};
	buffer_all = ranged((Buffer **)0, 0);
	g_buffer_cur = NULL;
	memset(&buffer_undo, 0, sizeof(buffer_undo));
	scratch = buffer_new();
	if (scratch == NULL) {
		return;
	}
	scratch.set_name(ScratchName);
	if (!buffers_reserve(1)) {
		scratch.free();
		return;
	}
	buffer_all = ranged(ptr(buffer_all), 1, cap(buffer_all));
	buffer_all[0] = scratch;
	g_buffer_cur = scratch;
}

void buffer_set_current(Buffer *buf) {
	static int64_t access_seq = 1;
	if (buf == NULL) {
		return;
	}
	g_buffer_cur = buf;
	buf.access_time = access_seq++;
}

Buffer *buffer_current() {
	return g_buffer_cur;
}

bool buffer_list_add(Buffer *buf) {
	if (buf == NULL) {
		return false;
	}
	size_t n = len(buffer_all);
	if (!buffers_reserve(n + 1)) {
		return false;
	}
	buffer_all = ranged(ptr(buffer_all), n + 1, cap(buffer_all));
	buffer_all[n] = buf;
	buffer_set_current(buf);
	return true;
}

/* Append without making buf current (jem.go Create for *match*). */
bool buffer_list_append(Buffer *buf) {
	if (buf == NULL) {
		return false;
	}
	size_t n = len(buffer_all);
	if (!buffers_reserve(n + 1)) {
		return false;
	}
	buffer_all = ranged(ptr(buffer_all), n + 1, cap(buffer_all));
	buffer_all[n] = buf;
	return true;
}

Buffer *buffer_find_by_name(const char[..] name) {
	for (auto b : buffer_all) {
		if (b != NULL && str_eq(b.name, name)) {
			return b;
		}
	}
	return NULL;
}

static char ascii_fold(char c) {
	if (c >= 'A' && c <= 'Z') {
		return (char)(c + ('a' - 'A'));
	}
	return c;
}

static bool name_equal_fold(const char[..] a, const char[..] b) {
	if (len(a) != len(b)) {
		return false;
	}
	for (size_t i = 0; i < len(a); i++) {
		if (ascii_fold(a[i]) != ascii_fold(b[i])) {
			return false;
		}
	}
	return true;
}

// buffer_find_by_label matches buffer name case-insensitively (jem.go findBufferByLabel).
Buffer *buffer_find_by_label(const char[..] label) {
	if (len(label) == 0) {
		return NULL;
	}
	for (auto b : buffer_all) {
		if (b != NULL && name_equal_fold(b.name, label)) {
			return b;
		}
	}
	return NULL;
}

// buffer_other returns a non-current buffer when one exists (fuzzy default stand-in).
Buffer *buffer_other() {
	Buffer *cur = buffer_current();
	for (auto b : buffer_all) {
		if (b != NULL && b != cur) {
			return b;
		}
	}
	return NULL;
}

/* Remove buf from the list and free it. Does not retarget windows (caller must). */
void buffer_release(Buffer *buf) {
	if (buf == NULL) {
		return;
	}
	if (buf.is_scratch()) {
		return;
	}
	size_t n = len(buffer_all);
	for (size_t i = 0; i < n; i++) {
		if (buffer_all[i] != buf) {
			continue;
		}
		for (size_t j = i; j + 1 < n; j++) {
			buffer_all[j] = buffer_all[j + 1];
		}
		n--;
		buffer_all = ranged(ptr(buffer_all), n, cap(buffer_all));
		if (g_buffer_cur == buf) {
			g_buffer_cur = n > 0 ? buffer_all[0] : NULL;
		}
		buf.free();
		return;
	}
}

static void undo_record_free(UndoRecord *r) {
	str_free(&r.text);
}

static void undo_group_reset(UndoGroup *g) {
	if (g == NULL) {
		return;
	}
	for (auto *r : g.records) {
		undo_record_free(r);
	}
	free(ptr(g.records));
	memset(g, 0, sizeof(*g));
}

void (UndoHistory *h).begin_command(Buffer *buf, Location before) {
	if (h.is_replaying) {
		return;
	}
	undo_group_reset(&h.pending);
	h.pending.buffer = buf;
	if (buf != NULL) {
		h.pending.buffer_serial = buf.serial;
	}
	h.pending.before = before;
}

void (UndoHistory *h).end_command() {
	int i = 0;
	if (h.is_replaying) {
		return;
	}
	if (len(h.pending.records) == 0) {
		undo_group_reset(&h.pending);
		return;
	}
	if (h.count == (int)UndoHistoryMax) {
		undo_group_reset(&h.groups[(int)UndoHistoryMax - 1]);
		h.count--;
	}
	for (i = h.count; i > 0; i--) {
		h.groups[i] = h.groups[i - 1];
	}
	h.pending.group_serial = h.next_group_serial;
	h.next_group_serial++;
	h.groups[0] = h.pending;
	memset(&h.pending, 0, sizeof(h.pending));
	h.count++;
}

void (UndoHistory *h).record_edit(UndoKind kind, int line_num, size_t offset, const char *text, size_t n) {
	if (h.is_replaying) {
		return;
	}
	size_t nrec = len(h.pending.records);
	if (nrec == cap(h.pending.records)) {
		size_t ncap = cap(h.pending.records) != 0 ? cap(h.pending.records) * 2 : 16;
		UndoRecord *records = realloc(ptr(h.pending.records),
			ncap * sizeof(UndoRecord));
		if (records == NULL) {
			return;
		}
		memset(records + nrec, 0, (ncap - nrec) * sizeof(UndoRecord));
		h.pending.records = ranged(records, nrec, ncap);
	}
	h.pending.records = ranged(ptr(h.pending.records), nrec + 1,
		cap(h.pending.records));
	UndoRecord *r = &h.pending.records[nrec];
	r.kind = kind;
	r.line_num = line_num;
	r.offset = offset;
	r.end.line = line_num;
	r.end.offset = offset;
	if (kind == UndoInsert) {
		for (size_t i = 0; i < n; i++) {
			if (text[i] == '\n') {
				r.end.line++;
				r.end.offset = 0;
			} else {
				r.end.offset++;
			}
		}
	}
	if (!str_set(&r.text, ranged(text, n))) {
		memset(r, 0, sizeof(*r));
		h.pending.records = ranged(ptr(h.pending.records), nrec,
			cap(h.pending.records));
		return;
	}
}

/* Undo the most recent group on its buffer; restores *cursor_out to pre-edit point. */
bool (UndoHistory *h).undo_once(Location *cursor_out, Buffer **buffer_out) {
	if (h.count == 0) {
		return false;
	}
	UndoGroup g = h.groups[0];
	Buffer *buf = g.buffer;
	h.is_replaying = true;
	for (int i = (int)len(g.records) - 1; i >= 0; i--) {
		UndoRecord *r = &g.records[i];
		Location at = {0};
		at.line = r.line_num;
		at.offset = r.offset;
		if (buf == NULL) {
			continue;
		}
		if (r.kind == UndoInsert) {
			buf.delete_range(at, r.end);
		} else {
			buf.insert_text(at, ptr(r.text), len(r.text), NULL);
		}
	}
	h.is_replaying = false;
	if (cursor_out != NULL) {
		*cursor_out = g.before;
	}
	if (buffer_out != NULL) {
		*buffer_out = buf;
	}
	undo_group_reset(&h.groups[0]);
	for (int j = 0; j + 1 < (int)h.count; j++) {
		h.groups[j] = h.groups[j + 1];
	}
	h.count--;
	return true;
}

void (UndoHistory *h).forget_buffer(Buffer *buf) {
	int i = 0;
	if (buf == NULL) {
		return;
	}
	if (h.pending.buffer == buf) {
		undo_group_reset(&h.pending);
	}
	while (i < (int)h.count) {
		if (h.groups[i].buffer == buf) {
			int j = 0;
			undo_group_reset(&h.groups[i]);
			for (j = i; j + 1 < (int)h.count; j++) {
				h.groups[j] = h.groups[j + 1];
			}
			memset(&h.groups[h.count - 1], 0, sizeof(UndoGroup));
			h.count--;
		} else {
			i++;
		}
	}
}

/* ---- text mutation ------------------------------------------------------- */

static bool make_line(Line *out, Buffer *buf, const char[..] data) {
	Line line = {0};
	if (out == NULL || !str_set(&line.data, data)) {
		return false;
	}
	if (buf != NULL) {
		line.lang_mode = buf.lang_mode;
	} else {
		line.lang_mode = LModeNone;
	}
	*out = line;
	return true;
}

bool (Buffer *buf).replace_raw(Location begin, Location end, const char *new_text, size_t new_len, Location *new_end_out) {
	/* jem.go Buffer.ReplaceRaw — EOF is virtual line eof_line() (line_count+1). */
	bool begin_is_eof = false;
	bool end_is_eof = false;
	int eof = 0;
	int nlines = 0;
	int end_real = 0;
	size_t keep_before = 0;
	size_t keep_after = 0;
	size_t prefix_len = 0;
	size_t suffix_len = 0;
	size_t i = 0;
	size_t part_start = 0;
	size_t n_parts = 0;
	size_t new_line_count = 0;
	size_t out = 0;
	char[..] rebuilt = {0};
	Line *bline = {0};
	Line *eline = {0};
	Line *new_lines = {0};
	Location new_end = {0};
	const char *insert = {0};
	size_t insert_len = 0;

	if (buf.is_readonly) {
		return false;
	}
	buf.ensure_min_lines();
	nlines = (int)len(buf.lines);
	eof = buf.eof_line();
	if (begin.line < 1) {
		begin.line = 1;
		begin.offset = 0;
	}
	if (end.line < begin.line || (end.line == begin.line && end.offset < begin.offset)) {
		return false;
	}
	/* Clamp past-EOF to EOF; do not collapse EOF to last-line end. */
	if (begin.line > eof) {
		begin.line = eof;
		begin.offset = 0;
	}
	if (end.line > eof) {
		end.line = eof;
		end.offset = 0;
	}
	begin_is_eof = begin.line == eof;
	end_is_eof = end.line == eof;
	end_real = end_is_eof ? nlines : end.line;

	if (begin.line == end.line && begin.offset == end.offset && (new_text == NULL || new_len == 0)) {
		if (new_end_out != NULL) {
			*new_end_out = begin;
		}
		return true;
	}

	insert = new_text;
	if (insert == NULL) {
		insert = "";
	}
	insert_len = new_len;

	if (!begin_is_eof) {
		bline = buf.line(begin.line);
		if (bline == NULL) {
			return false;
		}
		if (begin.offset > bline.byte_len()) {
			begin.offset = bline.byte_len();
		}
		prefix_len = begin.offset;
	}
	if (!end_is_eof) {
		eline = buf.line(end.line);
		if (eline == NULL) {
			return false;
		}
		if (end.offset > eline.byte_len()) {
			end.offset = eline.byte_len();
		}
		suffix_len = eline.byte_len() - end.offset;
	}

	/* Build middle text: prefix + insert + suffix (jem.go merge path). */
	if (!begin_is_eof) {
		if (!str_append(&rebuilt, bline.data[..prefix_len])) {
			str_free(&rebuilt);
			return false;
		}
	}
	if (!str_append(&rebuilt, ranged(insert, insert_len))) {
		str_free(&rebuilt);
		return false;
	}
	if (!end_is_eof && suffix_len > 0) {
		if (!str_append(&rebuilt, eline.data[end.offset..])) {
			str_free(&rebuilt);
			return false;
		}
	}

	n_parts = 1;
	for (auto c : rebuilt) {
		if (c == '\n') {
			n_parts++;
		}
	}

	keep_before = begin_is_eof ? (size_t)nlines : (size_t)(begin.line - 1);
	if (end_is_eof) {
		keep_after = 0;
	} else {
		keep_after = nlines - end.line;
	}
	new_line_count = keep_before + n_parts + keep_after;
	new_lines = calloc(new_line_count, sizeof(Line));
	if (new_lines == NULL) {
		str_free(&rebuilt);
		return false;
	}

	part_start = 0;
	out = keep_before;
	for (i = 0; i <= len(rebuilt); i++) {
		if (i == len(rebuilt) || rebuilt[i] == '\n') {
			if (!make_line(&new_lines[out], buf, rebuilt[part_start..i])) {
				size_t j = 0;
				for (j = keep_before; j < out; j++) {
					line_free(&new_lines[j]);
				}
				free(new_lines);
				str_free(&rebuilt);
				return false;
			}
			out++;
			part_start = i + 1;
		}
	}

	/* new_end at end of inserted text (jem.go: prefixLen+len(part0) or len(last part)). */
	{
		size_t last_start = 0;
		for (i = 0; i < insert_len; i++) {
			if (insert[i] == '\n') {
				last_start = i + 1;
			}
		}
		new_end.line = (int)keep_before + (int)n_parts;
		if (n_parts == 1) {
			new_end.offset = prefix_len + insert_len;
		} else {
			new_end.offset = insert_len - last_start;
		}
	}

	for (i = 0; i < keep_before; i++) {
		new_lines[i] = buf.lines[i];
		memset(&buf.lines[i], 0, sizeof(Line));
	}
	out = keep_before + n_parts;
	if (!end_is_eof) {
		int li = end.line;
		while (li < nlines) {
			new_lines[out++] = buf.lines[li];
			memset(&buf.lines[li], 0, sizeof(Line));
			li++;
		}
	}
	for (i = keep_before; i < new_line_count; i++) {
		new_lines[i].invalidate();
	}

	/* Free replaced real lines [keep_before .. end_real). */
	{
		size_t free_end = end_is_eof ? (size_t)nlines : (size_t)end_real;
		for (i = keep_before; i < free_end && i < len(buf.lines); i++) {
			line_free(&buf.lines[i]);
		}
	}
	free(ptr(buf.lines));
	buf.lines = ranged(new_lines, new_line_count, new_line_count);
	buf.serial++;
	buf.mark_changed();
	str_free(&rebuilt);

	if (new_end_out != NULL) {
		*new_end_out = new_end;
	}
	return true;
}

/* get_text returns malloc'd bytes between begin and end (jem.go GetText). EOF ok. */
(char *, size_t, bool) (Buffer *buf).get_text(Location begin, Location end) {
	bool end_is_eof = false;
	int eof = 0;
	int last_real = 0;
	int ln = 0;
	char *data = NULL;
	size_t n = 0;
	size_t cap_n = 0;

	eof = buf.eof_line();
	if (begin.line > end.line || (begin.line == end.line && begin.offset > end.offset)) {
		Location t = begin;
		begin = end;
		end = t;
	}
	if (begin.line > eof) {
		begin.line = eof;
		begin.offset = 0;
	}
	if (end.line > eof) {
		end.line = eof;
		end.offset = 0;
	}
	end_is_eof = end.line == eof;
	if (begin.line == end.line) {
		Line *line = NULL;
		size_t b = 0;
		size_t e = 0;
		if (begin.line == eof) {
			return (NULL, 0, true);
		}
		line = buf.line(begin.line);
		if (line == NULL) {
			return (NULL, 0, false);
		}
		b = begin.offset;
		if (b > line.byte_len()) {
			b = line.byte_len();
		}
		e = end.offset;
		if (e > line.byte_len()) {
			e = line.byte_len();
		}
		if (e > b) {
			n = e - b;
			data = malloc(n + 1);
			if (data == NULL) {
				return (NULL, 0, false);
			}
			memcpy(data, ptr(line.data) + b, n);
			data[n] = 0;
			return (data, n, true);
		}
		return (NULL, 0, true);
	}

	if (end_is_eof) {
		last_real = buf.line_count_i();
	} else {
		last_real = end.line;
	}
	/* Copy start line tail + interior + final segment (no trailing nl after last when EOF). */
	for (ln = begin.line; ln <= last_real; ln++) {
		Line *line = buf.line(ln);
		size_t from = 0;
		size_t to = 0;
		size_t chunk = 0;
		if (line == NULL) {
			continue;
		}
		if (ln == begin.line) {
			from = begin.offset;
			if (from > line.byte_len()) {
				from = line.byte_len();
			}
			to = line.byte_len();
		} else if (ln == last_real && !end_is_eof) {
			from = 0;
			to = end.offset;
			if (to > line.byte_len()) {
				to = line.byte_len();
			}
		} else {
			from = 0;
			to = line.byte_len();
		}
		chunk = to > from ? to - from : 0;
		if (n + chunk + 2 > cap_n) {
			size_t ncap = cap_n != 0 ? cap_n * 2 : 256;
			char *p = NULL;
			while (ncap < n + chunk + 2) {
				ncap *= 2;
			}
			p = realloc(data, ncap);
			if (p == NULL) {
				free(data);
				return (NULL, 0, false);
			}
			data = p;
			cap_n = ncap;
		}
		if (ln > begin.line) {
			data[n++] = '\n';
		}
		if (chunk > 0) {
			memcpy(data + n, ptr(line.data) + from, chunk);
			n += chunk;
		}
	}
	if (data != NULL) {
		data[n] = 0;
	}
	return (data, n, true);
}

bool (Buffer *buf).insert_text(Location at, const char *text, size_t n, Location *new_end_out) {
	return buf.replace_raw(at, at, text, n, new_end_out);
}

bool (Buffer *buf).delete_range(Location begin, Location end) {
	Location dummy = {0};
	return buf.replace_raw(begin, end, "", 0, &dummy);
}

// clear drops all lines to a single blank line (jem.go Buffer.Clear).
void (Buffer *buf).clear() {
	for (auto *line : buf.lines) {
		line_free(line);
	}
	free(ptr(buf.lines));
	buf.lines = ranged((Line *)0, 0);
	buf.ensure_min_lines();
	buf.is_changed = false;
	buf.cursor.line = 1;
	buf.cursor.offset = 0;
	buf.mark.line = 0;
	buf.mark.offset = 0;
}

// append_line appends a line view and returns it (jem.go AppendLineBytes).
Line *(Buffer *buf).append_line(const char[..] text) {
	Line line = {0};
	buf.ensure_min_lines();
	size_t n = len(buf.lines);
	if (!buffer_reserve_lines(buf, n + 1)) {
		return NULL;
	}
	if (!make_line(&line, buf, text)) {
		return NULL;
	}
	buf.lines = ranged(ptr(buf.lines), n + 1, cap(buf.lines));
	buf.lines[n] = line;
	return &buf.lines[n];
}

bool (Buffer *buf).load_bytes(const char *data, size_t n, EolMode eol) {
	size_t i = 0;
	size_t start = 0;
	size_t line_i = 0;
	size_t count = 1;
	const char *p = {0};
	size_t ln = 0;

	for (auto *line : buf.lines) {
		line_free(line);
	}
	free(ptr(buf.lines));
	buf.lines = ranged((Line *)0, 0);

	if (data == NULL) {
		n = 0;
	}
	for (i = 0; i < n; i++) {
		if (data[i] == '\n') {
			count++;
		}
	}
	if (!buffer_reserve_lines(buf, count)) {
		buf.ensure_min_lines();
		return false;
	}

	start = 0;
	line_i = 0;
	for (i = 0; i <= n; i++) {
		if (i == n || data[i] == '\n') {
			p = data + start;
			ln = i - start;
			if (eol == EModeCRLF && ln != 0 && p[ln - 1] == '\r') {
				ln--;
			}
			if (!make_line(&ptr(buf.lines)[line_i], buf, ranged(p, ln))) {
				size_t j = 0;
				for (j = 0; j < line_i; j++) {
					line_free(&ptr(buf.lines)[j]);
				}
				free(ptr(buf.lines));
				buf.lines = ranged((Line *)0, 0);
				buf.ensure_min_lines();
				return false;
			}
			line_i++;
			start = i + 1;
		}
	}
	buf.lines = ranged(ptr(buf.lines), line_i, cap(buf.lines));
	if (len(buf.lines) == 0) {
		buf.ensure_min_lines();
	}
	buf.eol_mode = eol;
	buf.is_changed = false;
	buf.serial++;
	buf.cursor.line = 1;
	buf.cursor.offset = 0;
	return true;
}

(char *, size_t, bool) (Buffer *buf).save_bytes() {
	size_t i = 0;
	size_t total = 0;
	char *out = {0};
	size_t off = 0;
	char *nl = {0};
	size_t nllen = 0;

	nl = (char *)(void *)(buf.eol_mode == EModeCRLF ? "\r\n" : "\n");
	nllen = buf.eol_mode == EModeCRLF ? 2 : 1;
	for (i = 0; i < len(buf.lines); i++) {
		total = total + len(buf.lines[i].data);
		if (i + 1 < len(buf.lines)) {
			total = total + nllen;
		}
	}
	out = malloc(total + 1);
	if (out == NULL) {
		return (NULL, 0, false);
	}
	for (i = 0; i < len(buf.lines); i++) {
		if (len(buf.lines[i].data) != 0) {
			memcpy(out + off, ptr(buf.lines[i].data), len(buf.lines[i].data));
			off = off + len(buf.lines[i].data);
		}
		if (i + 1 < len(buf.lines)) {
			memcpy(out + off, nl, nllen);
			off = off + nllen;
		}
	}
	out[off] = 0;
	return (out, off, true);
}

void (Buffer *buf).set_name(const char[..] name) {
	(void)str_set(&buf.name, name);
}

void (Buffer *buf).set_file_name(const char[..] path) {
	(void)str_set(&buf.file_name, path);
}


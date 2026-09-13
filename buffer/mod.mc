// buffer — document model, line storage, undo, UTF-8 helpers.
import "syntax";

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

enum EolMode {
	EModeLF = 0,
	EModeCRLF = 1
};

// IndentConfig matches jem.go/buffer.IndentConfig.
struct IndentConfig {
	int width;
	int brace;
	int label;
	int continued;
};

struct Location {
	int line;
	int offset;
};

struct Bytes {
	char *ptr;
	size_t len;
	size_t cap;
};

struct Line {
	Bytes data;
	SynState syntax_end;
	bool syntax_valid;
	TextStyle *styles;
	size_t styles_len;
	bool cache_valid;
	LangMode lang_mode;
};

struct Buffer {
	Line *lines;
	size_t line_count;
	size_t line_cap;
	uint32_t serial;
	uint32_t saved_undo_serial;
	bool is_changed;
	bool is_readonly;
	EolMode eol_mode;
	LangMode lang_mode;
	int fill_col;
	IndentConfig indent;
	bool whitespace_cleanup;
	char *name;
	char *file_name;
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
	int offset;
	Bytes text;
};

enum UndoLimits {
	UndoGroupMaxRecords = 64,
	UndoHistoryMax = 64
};

struct UndoGroup {
	Buffer *buffer;
	uint32_t buffer_serial;
	uint32_t group_serial;
	Location before;
	UndoRecord records[UndoGroupMaxRecords];
	int count;
};

struct UndoHistory {
	UndoGroup groups[UndoHistoryMax];
	UndoGroup pending;
	uint32_t next_group_serial;
	uint8_t count;
	bool is_replaying;
};

struct BufferList {
	Buffer **bufs;
	size_t count;
	size_t cap;
	Buffer *current;
};

BufferList buffer_all;
UndoHistory buffer_undo;

static char ScratchName[] = "*scratch*";

static Bytes bytes_empty(void) {
	Bytes b = {0};
	return b;
}

static bool bytes_reserve(Bytes *b, size_t need) {
	char *p = {0};
	size_t ncap = {0};
	if (b == NULL) {
		return false;
	}
	if (need <= b.cap) {
		return true;
	}
	ncap = b.cap != 0 ? b.cap : 32;
	while (ncap < need) {
		ncap = ncap * 2;
	}
	p = realloc(b.ptr, ncap);
	if (p == NULL) {
		return false;
	}
	b.ptr = p;
	b.cap = ncap;
	return true;
}

static bool bytes_set(Bytes *b, char *src, size_t n) {
	if (!bytes_reserve(b, n + 1)) {
		return false;
	}
	if (n != 0 && src != NULL) {
		memcpy(b.ptr, src, n);
	}
	b.len = n;
	if (b.ptr != NULL) {
		b.ptr[n] = 0;
	}
	return true;
}

static bool bytes_append(Bytes *b, char *src, size_t n) {
	if (!bytes_reserve(b, b.len + n + 1)) {
		return false;
	}
	if (n != 0 && src != NULL) {
		memcpy(b.ptr + b.len, src, n);
	}
	b.len = b.len + n;
	b.ptr[b.len] = 0;
	return true;
}

static void bytes_free(Bytes *b) {
	if (b == NULL) {
		return;
	}
	free(b.ptr);
	b.ptr = NULL;
	b.len = 0;
	b.cap = 0;
}

static char *xstrdup(char *s) {
	size_t n = {0};
	char *p = {0};
	if (s == NULL) {
		s = "";
	}
	n = strlen(s);
	p = malloc(n + 1);
	if (p == NULL) {
		return NULL;
	}
	memcpy(p, s, n + 1);
	return p;
}

// utf8_next_offset returns the next rune start after offset.
int utf8_next_offset(char[..] data, int offset) {
	size_t n = len(data);
	unsigned char c = {0};
	int size = 1;
	char *dp = ptr(data);
	if (offset < 0 || (size_t)offset >= n || dp == NULL) {
		return offset;
	}
	c = (unsigned char)dp[offset];
	if (c < (unsigned char)0x80) {
		size = 1;
	} else if ((c & (unsigned char)0xE0) == (unsigned char)0xC0) {
		size = 2;
	} else if ((c & (unsigned char)0xF0) == (unsigned char)0xE0) {
		size = 3;
	} else if ((c & (unsigned char)0xF8) == (unsigned char)0xF0) {
		size = 4;
	} else {
		size = 1;
	}
	if ((size_t)offset + (size_t)size > n) {
		return offset + 1;
	}
	return offset + size;
}

// utf8_prev_offset returns the start of the rune before offset.
int utf8_prev_offset(char[..] data, int offset) {
	int i = {0};
	char *dp = ptr(data);
	if (offset <= 0 || dp == NULL) {
		return 0;
	}
	i = offset - 1;
	while (i > 0 && ((unsigned char)dp[i] & 0xC0) == 0x80) {
		i--;
	}
	return i;
}

// utf8_decode_rune reads one rune at offset; returns (rune, size).
(uint32_t, int) utf8_decode_rune(char[..] data, int offset) {
	size_t n = len(data);
	unsigned char c = {0};
	uint32_t r = {0};
	char *dp = ptr(data);
	if (offset < 0 || (size_t)offset >= n || dp == NULL) {
		return (0, 0);
	}
	c = (unsigned char)dp[offset];
	if (c < 0x80) {
		return ((uint32_t)c, 1);
	}
	if ((c & 0xE0) == 0xC0 && (size_t)offset + 1 < n) {
		r = (uint32_t)(c & 0x1F) << 6;
		r |= (uint32_t)((unsigned char)dp[offset + 1] & 0x3F);
		return (r, 2);
	}
	if ((c & 0xF0) == 0xE0 && (size_t)offset + 2 < n) {
		r = (uint32_t)(c & 0x0F) << 12;
		r |= (uint32_t)((unsigned char)dp[offset + 1] & 0x3F) << 6;
		r |= (uint32_t)((unsigned char)dp[offset + 2] & 0x3F);
		return (r, 3);
	}
	if ((c & 0xF8) == 0xF0 && (size_t)offset + 3 < n) {
		r = (uint32_t)(c & 0x07) << 18;
		r |= (uint32_t)((unsigned char)dp[offset + 1] & 0x3F) << 12;
		r |= (uint32_t)((unsigned char)dp[offset + 2] & 0x3F) << 6;
		r |= (uint32_t)((unsigned char)dp[offset + 3] & 0x3F);
		return (r, 4);
	}
	return ((uint32_t)c, 1);
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
char[..] (Line *line).view(void) {
	if (line == NULL || line.data.ptr == NULL) {
		return ranged((char *)0, 0);
	}
	return ranged(line.data.ptr, line.data.len);
}

int (Line *line).byte_len(void) {
	if (line == NULL) {
		return 0;
	}
	return (int)line.data.len;
}

// first_nonblank returns the byte index of the first non-space/tab (jem.go Line.FirstNonblank).
int (Line *line).first_nonblank(void) {
	char[..] v = {0};
	int i = 0;
	if (line == NULL) {
		return 0;
	}
	v = line.view();
	{
		char *vp = ptr(v);
		while (i < (int)len(v)) {
			char c = vp[i];
		if (c != ' ' && c != '\t') {
			return i;
		}
			i++;
		}
	}
	return (int)len(v);
}

bool (Line *line).is_blank(void) {
	return line != NULL && line.first_nonblank() >= line.byte_len();
}

int (Line *line).indent_column(void) {
	if (line == NULL) {
		return 0;
	}
	return line.display_col_at_offset(line.first_nonblank());
}

char (Line *line).last_byte(void) {
	int n = 0;
	if (line == NULL) {
		return 0;
	}
	n = line.byte_len();
	if (n <= 0) {
		return 0;
	}
	return line.data.ptr[n - 1];
}

char (Line *line).first_byte(void) {
	int i = 0;
	if (line == NULL) {
		return 0;
	}
	i = line.first_nonblank();
	if (i >= line.byte_len()) {
		return 0;
	}
	return line.data.ptr[i];
}

void (Line *line).invalidate(void) {
	if (line == NULL) {
		return;
	}
	line.syntax_valid = false;
	line.cache_valid = false;
	free(line.styles);
	line.styles = NULL;
	line.styles_len = 0;
}

int (Line *line).display_col_at_offset(int offset) {
	char[..] v = {0};
	int i = 0;
	int col = 0;
	uint32_t r = {0};
	int sz = 0;
	if (line == NULL) {
		return 0;
	}
	v = line.view();
	if (offset < 0) {
		offset = 0;
	}
	if ((size_t)offset > len(v)) {
		offset = (int)len(v);
	}
	while (i < offset) {
		auto (rr, ss) = utf8_decode_rune(v, i);
		r = rr;
		sz = ss;
		if (sz <= 0) {
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

int (Line *line).offset_at_display_col(int goal) {
	char[..] v = {0};
	int i = 0;
	int col = 0;
	uint32_t r = {0};
	int sz = 0;
	int w = 0;
	if (line == NULL) {
		return 0;
	}
	v = line.view();
	if (goal <= 0) {
		return 0;
	}
	while (i < (int)len(v)) {
		auto (rr, ss) = utf8_decode_rune(v, i);
		r = rr;
		sz = ss;
		if (sz <= 0) {
			break;
		}
		if (r == (uint32_t)'\t') {
			w = 8 - (col % 8);
		} else {
			w = rune_display_width(r);
			if (w <= 0) {
				w = 1;
			}
		}
		if (col + w > goal) {
			return i;
		}
		col = col + w;
		i = i + sz;
	}
	return (int)len(v);
}

static void line_free(Line *line) {
	if (line == NULL) {
		return;
	}
	bytes_free(&line.data);
	free(line.styles);
	line.styles = NULL;
}

bool (Buffer *buf).is_scratch(void) {
	if (buf == NULL || buf.name == NULL) {
		return false;
	}
	return strcmp(buf.name, ScratchName) == 0;
}

void (Buffer *buf).mark_changed(void) {
	if (buf == NULL || buf.is_scratch()) {
		return;
	}
	buf.is_changed = true;
}

static bool buffer_reserve_lines(Buffer *buf, size_t need) {
	Line *p = {0};
	size_t ncap = {0};
	if (need <= buf.line_cap) {
		return true;
	}
	ncap = buf.line_cap != 0 ? buf.line_cap : 8;
	while (ncap < need) {
		ncap = ncap * 2;
	}
	p = realloc(buf.lines, ncap * sizeof(Line));
	if (p == NULL) {
		return false;
	}
	buf.lines = p;
	buf.line_cap = ncap;
	return true;
}

void (Buffer *buf).ensure_min_lines(void) {
	Line empty = {0};
	if (buf == NULL) {
		return;
	}
	if (buf.line_count != 0) {
		return;
	}
	if (!buffer_reserve_lines(buf, 1)) {
		return;
	}
	empty.data = bytes_empty();
	empty.lang_mode = buf.lang_mode;
	buf.lines[0] = empty;
	buf.line_count = 1;
}

// buffer_new allocates an empty buffer with one blank line.
Buffer *buffer_new(void) {
	Buffer *buf = calloc(1, sizeof(Buffer));
	if (buf == NULL) {
		return NULL;
	}
	buf.eol_mode = EModeLF;
	buf.lang_mode = LModeNone;
	buf.indent.width = 2;
	buf.cursor.line = 1;
	buf.cursor.offset = 0;
	buf.mark.line = 0;
	buf.ensure_min_lines();
	return buf;
}

void (Buffer *buf).free(void) {
	size_t i = 0;
	if (buf == NULL) {
		return;
	}
	for (i = 0; i < buf.line_count; i++) {
		line_free(&buf.lines[i]);
	}
	free(buf.lines);
	free(buf.name);
	free(buf.file_name);
	free(buf);
}

Line *(Buffer *buf).line(int line_num) {
	if (buf == NULL || line_num < 1 || (size_t)line_num > buf.line_count) {
		return NULL;
	}
	return &buf.lines[line_num - 1];
}

int (Buffer *buf).line_count_i(void) {
	if (buf == NULL) {
		return 0;
	}
	return (int)buf.line_count;
}

int (Buffer *buf).eof_line(void) {
	return buf.line_count_i() + 1;
}

static bool list_reserve(BufferList *list, size_t need) {
	Buffer **p = {0};
	size_t ncap = {0};
	if (need <= list.cap) {
		return true;
	}
	ncap = list.cap != 0 ? list.cap : 4;
	while (ncap < need) {
		ncap = ncap * 2;
	}
	p = realloc(list.bufs, ncap * sizeof(Buffer *));
	if (p == NULL) {
		return false;
	}
	list.bufs = p;
	list.cap = ncap;
	return true;
}

void buffer_list_init(void) {
	Buffer *scratch = {0};
	memset(&buffer_all, 0, sizeof(buffer_all));
	memset(&buffer_undo, 0, sizeof(buffer_undo));
	scratch = buffer_new();
	if (scratch == NULL) {
		return;
	}
	scratch.name = xstrdup(ScratchName);
	if (!list_reserve(&buffer_all, 1)) {
		scratch.free();
		return;
	}
	buffer_all.bufs[0] = scratch;
	buffer_all.count = 1;
	buffer_all.current = scratch;
}

void buffer_set_current(Buffer *buf) {
	static int64_t access_seq = 1;
	if (buf == NULL) {
		return;
	}
	buffer_all.current = buf;
	buf.access_time = access_seq++;
}

Buffer *buffer_current(void) {
	return buffer_all.current;
}

bool buffer_list_add(Buffer *buf) {
	if (buf == NULL) {
		return false;
	}
	if (!list_reserve(&buffer_all, buffer_all.count + 1)) {
		return false;
	}
	buffer_all.bufs[buffer_all.count] = buf;
	buffer_all.count++;
	buffer_set_current(buf);
	return true;
}

/* Append without making buf current (jem.go Create for *match*). */
bool buffer_list_append(Buffer *buf) {
	if (buf == NULL) {
		return false;
	}
	if (!list_reserve(&buffer_all, buffer_all.count + 1)) {
		return false;
	}
	buffer_all.bufs[buffer_all.count] = buf;
	buffer_all.count++;
	return true;
}

Buffer *buffer_find_by_name(char *name) {
	size_t i = 0;
	if (name == NULL) {
		return NULL;
	}
	for (i = 0; i < buffer_all.count; i++) {
		if (buffer_all.bufs[i] != NULL && buffer_all.bufs[i].name != NULL &&
			strcmp(buffer_all.bufs[i].name, name) == 0) {
			return buffer_all.bufs[i];
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

static bool name_equal_fold(char *a, char *b) {
	size_t i = 0;
	if (a == NULL || b == NULL) {
		return false;
	}
	for (i = 0; a[i] != 0 || b[i] != 0; i++) {
		if (ascii_fold(a[i]) != ascii_fold(b[i])) {
			return false;
		}
	}
	return true;
}

// buffer_find_by_label matches buffer name case-insensitively (jem.go findBufferByLabel).
Buffer *buffer_find_by_label(char *label) {
	size_t i = 0;
	if (label == NULL || label[0] == 0) {
		return NULL;
	}
	for (i = 0; i < buffer_all.count; i++) {
		Buffer *b = buffer_all.bufs[i];
		if (b != NULL && b.name != NULL && name_equal_fold(b.name, label)) {
			return b;
		}
	}
	return NULL;
}

// buffer_other returns a non-current buffer when one exists (fuzzy default stand-in).
Buffer *buffer_other(void) {
	Buffer *cur = buffer_current();
	size_t i = 0;
	for (i = 0; i < buffer_all.count; i++) {
		if (buffer_all.bufs[i] != NULL && buffer_all.bufs[i] != cur) {
			return buffer_all.bufs[i];
		}
	}
	return NULL;
}

/* Remove buf from the list and free it. Does not retarget windows (caller must). */
void buffer_release(Buffer *buf) {
	size_t i = 0;
	size_t j = 0;
	if (buf == NULL || buf.is_scratch()) {
		return;
	}
	for (i = 0; i < buffer_all.count; i++) {
		if (buffer_all.bufs[i] != buf) {
			continue;
		}
		for (j = i; j + 1 < buffer_all.count; j++) {
			buffer_all.bufs[j] = buffer_all.bufs[j + 1];
		}
		buffer_all.count--;
		buffer_all.bufs[buffer_all.count] = NULL;
		if (buffer_all.current == buf) {
			buffer_all.current = buffer_all.count > 0 ? buffer_all.bufs[0] : NULL;
		}
		buf.free();
		return;
	}
}

void (UndoHistory *h).begin_command(Buffer *buf, Location before) {
	if (h == NULL || h.is_replaying) {
		return;
	}
	memset(&h.pending, 0, sizeof(h.pending));
	h.pending.buffer = buf;
	if (buf != NULL) {
		h.pending.buffer_serial = buf.serial;
	}
	h.pending.before = before;
}

static void undo_record_free(UndoRecord *r) {
	bytes_free(&r.text);
}

static void undo_group_reset(UndoGroup *g) {
	int i = 0;
	if (g == NULL) {
		return;
	}
	for (i = 0; i < g.count; i++) {
		undo_record_free(&g.records[i]);
	}
	memset(g, 0, sizeof(*g));
}

void (UndoHistory *h).end_command(void) {
	int i = 0;
	if (h == NULL || h.is_replaying) {
		return;
	}
	if (h.pending.count == 0) {
		undo_group_reset(&h.pending);
		return;
	}
	if (h.count == UndoHistoryMax) {
		undo_group_reset(&h.groups[UndoHistoryMax - 1]);
		h.count--;
	}
	for (i = (int)h.count; i > 0; i--) {
		h.groups[i] = h.groups[i - 1];
	}
	h.pending.group_serial = h.next_group_serial;
	h.next_group_serial++;
	h.groups[0] = h.pending;
	memset(&h.pending, 0, sizeof(h.pending));
	h.count++;
}

void (UndoHistory *h).record_edit(UndoKind kind, int line_num, int offset, char *text, size_t n) {
	UndoRecord *r = {0};
	if (h == NULL || h.is_replaying) {
		return;
	}
	if (h.pending.count >= UndoGroupMaxRecords) {
		return;
	}
	r = &h.pending.records[h.pending.count];
	r.kind = kind;
	r.line_num = line_num;
	r.offset = offset;
	r.text = bytes_empty();
	bytes_set(&r.text, text, n);
	h.pending.count++;
}

/* Undo the most recent group on its buffer; restores *cursor_out to pre-edit point. */
bool (UndoHistory *h).undo_once(Location *cursor_out) {
	UndoGroup g = {0};
	int i = 0;
	int j = 0;
	Buffer *buf = {0};
	if (h == NULL || h.count == 0) {
		return false;
	}
	g = h.groups[0];
	buf = g.buffer;
	h.is_replaying = true;
	for (i = g.count - 1; i >= 0; i--) {
		UndoRecord *r = &g.records[i];
		Location at = {0};
		at.line = r.line_num;
		at.offset = r.offset;
		if (buf == NULL) {
			continue;
		}
		if (r.kind == UndoInsert) {
			Location end = at;
			end.offset = at.offset + (int)r.text.len;
			buf.delete_range(at, end);
		} else {
			buf.insert_text(at, r.text.ptr, r.text.len, NULL);
		}
	}
	h.is_replaying = false;
	if (cursor_out != NULL) {
		*cursor_out = g.before;
	}
	undo_group_reset(&h.groups[0]);
	for (j = 0; j + 1 < (int)h.count; j++) {
		h.groups[j] = h.groups[j + 1];
	}
	h.count--;
	return true;
}

/* ---- text mutation ------------------------------------------------------- */

static Line make_line(Buffer *buf, char *data, size_t n) {
	Line line = {0};
	line.data = bytes_empty();
	bytes_set(&line.data, data, n);
	if (buf != NULL) {
		line.lang_mode = buf.lang_mode;
	} else {
		line.lang_mode = LModeNone;
	}
	return line;
}

bool (Buffer *buf).replace_raw(Location begin, Location end, char *new_text, size_t new_len, Location *new_end_out) {
	Line *bline = {0};
	Line *eline = {0};
	Bytes prefix = {0};
	Bytes suffix = {0};
	Bytes rebuilt = {0};
	size_t i = 0;
	size_t part_start = 0;
	size_t n_parts = 0;
	size_t p = 0;
	size_t new_line_count = 0;
	size_t keep_before = 0;
	size_t keep_after = 0;
	Line *new_lines = {0};
	size_t out = 0;
	Location new_end = {0};
	char *insert = {0};
	size_t insert_len = 0;

	if (buf == NULL || buf.is_readonly) {
		return false;
	}
	buf.ensure_min_lines();
	if (begin.line < 1) {
		begin.line = 1;
		begin.offset = 0;
	}
	if (end.line < begin.line || (end.line == begin.line && end.offset < begin.offset)) {
		return false;
	}
	if (begin.line > (int)buf.line_count) {
		begin.line = (int)buf.line_count;
		begin.offset = buf.lines[buf.line_count - 1].byte_len();
	}
	if (end.line > (int)buf.line_count) {
		end.line = (int)buf.line_count;
		end.offset = buf.lines[buf.line_count - 1].byte_len();
	}

	insert = new_text;
	if (insert == NULL) {
		static char empty_ins[1];
		empty_ins[0] = 0;
		insert = empty_ins;
	}
	insert_len = new_len;

	bline = buf.line(begin.line);
	eline = buf.line(end.line);
	if (bline == NULL || eline == NULL) {
		return false;
	}
	if (begin.offset > bline.byte_len()) {
		begin.offset = bline.byte_len();
	}
	if (end.offset > eline.byte_len()) {
		end.offset = eline.byte_len();
	}

	prefix = bytes_empty();
	suffix = bytes_empty();
	bytes_set(&prefix, bline.data.ptr, (size_t)begin.offset);
	bytes_set(&suffix, eline.data.ptr + end.offset, eline.data.len - (size_t)end.offset);

	rebuilt = bytes_empty();
	bytes_append(&rebuilt, prefix.ptr, prefix.len);
	bytes_append(&rebuilt, insert, insert_len);
	bytes_append(&rebuilt, suffix.ptr, suffix.len);

	/* count newline-separated parts */
	n_parts = 1;
	for (i = 0; i < rebuilt.len; i++) {
		if (rebuilt.ptr[i] == '\n') {
			n_parts++;
		}
	}

	keep_before = (size_t)(begin.line - 1);
	keep_after = buf.line_count - (size_t)end.line;
	new_line_count = keep_before + n_parts + keep_after;
	new_lines = calloc(new_line_count, sizeof(Line));
	if (new_lines == NULL) {
		bytes_free(&prefix);
		bytes_free(&suffix);
		bytes_free(&rebuilt);
		return false;
	}

	for (i = 0; i < keep_before; i++) {
		new_lines[out++] = buf.lines[i];
		memset(&buf.lines[i], 0, sizeof(Line));
	}

	part_start = 0;
	p = 0;
	for (i = 0; i <= rebuilt.len; i++) {
		if (i == rebuilt.len || rebuilt.ptr[i] == '\n') {
			new_lines[out] = make_line(buf, rebuilt.ptr + part_start, i - part_start);
			out++;
			p++;
			part_start = i + 1;
		}
	}

	/* fix new_end for multi-line inserts */
	{
		size_t nl = 0;
		size_t last_start = 0;
		for (i = 0; i < insert_len; i++) {
			if (insert[i] == '\n') {
				nl++;
				last_start = i + 1;
			}
		}
		new_end.line = begin.line + (int)nl;
		if (nl == 0) {
			new_end.offset = begin.offset + (int)insert_len;
		} else {
			new_end.offset = (int)(insert_len - last_start);
		}
	}

	for (i = (size_t)end.line; i < buf.line_count; i++) {
		new_lines[out++] = buf.lines[i];
		memset(&buf.lines[i], 0, sizeof(Line));
	}

	/* free replaced lines */
	for (i = keep_before; i < (size_t)end.line && i < buf.line_count; i++) {
		line_free(&buf.lines[i]);
	}
	free(buf.lines);
	buf.lines = new_lines;
	buf.line_count = new_line_count;
	buf.line_cap = new_line_count;
	buf.serial++;
	buf.mark_changed();

	bytes_free(&prefix);
	bytes_free(&suffix);
	bytes_free(&rebuilt);

	if (new_end_out != NULL) {
		*new_end_out = new_end;
	}
	return true;
}

bool (Buffer *buf).insert_text(Location at, char *text, size_t n, Location *new_end_out) {
	return buf.replace_raw(at, at, text, n, new_end_out);
}

bool (Buffer *buf).delete_range(Location begin, Location end) {
	Location dummy = {0};
	return buf.replace_raw(begin, end, "", 0, &dummy);
}

bool (Buffer *buf).load_bytes(char *data, size_t n, EolMode eol) {
	size_t i = 0;
	size_t start = 0;
	size_t line_i = 0;
	size_t count = 1;
	char *p = {0};
	size_t ln = 0;

	if (buf == NULL) {
		return false;
	}
	for (i = 0; i < buf.line_count; i++) {
		line_free(&buf.lines[i]);
	}
	free(buf.lines);
	buf.lines = NULL;
	buf.line_count = 0;
	buf.line_cap = 0;

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
			if (eol == EModeCRLF && ln != 0 && (unsigned char)p[ln - 1] == (unsigned char)'\r') {
				ln--;
			}
			buf.lines[line_i] = make_line(buf, p, ln);
			line_i++;
			start = i + 1;
		}
	}
	buf.line_count = line_i;
	if (buf.line_count == 0) {
		buf.ensure_min_lines();
	}
	buf.eol_mode = eol;
	buf.is_changed = false;
	buf.serial++;
	buf.cursor.line = 1;
	buf.cursor.offset = 0;
	return true;
}

(char *, size_t, bool) (Buffer *buf).save_bytes(void) {
	size_t i = 0;
	size_t total = 0;
	char *out = {0};
	size_t off = 0;
	char *nl = {0};
	size_t nllen = 0;

	if (buf == NULL) {
		return (NULL, 0, false);
	}
	nl = (char *)(void *)(buf.eol_mode == EModeCRLF ? "\r\n" : "\n");
	nllen = buf.eol_mode == EModeCRLF ? 2 : 1;
	for (i = 0; i < buf.line_count; i++) {
		total = total + buf.lines[i].data.len;
		if (i + 1 < buf.line_count) {
			total = total + nllen;
		}
	}
	out = malloc(total + 1);
	if (out == NULL) {
		return (NULL, 0, false);
	}
	for (i = 0; i < buf.line_count; i++) {
		if (buf.lines[i].data.len != 0) {
			memcpy(out + off, buf.lines[i].data.ptr, buf.lines[i].data.len);
			off = off + buf.lines[i].data.len;
		}
		if (i + 1 < buf.line_count) {
			memcpy(out + off, nl, nllen);
			off = off + nllen;
		}
	}
	out[off] = 0;
	return (out, off, true);
}

void (Buffer *buf).set_name(char *name) {
	if (buf == NULL) {
		return;
	}
	free(buf.name);
	buf.name = xstrdup(name);
}

void (Buffer *buf).set_file_name(char *path) {
	if (buf == NULL) {
		return;
	}
	free(buf.file_name);
	buf.file_name = xstrdup(path);
}


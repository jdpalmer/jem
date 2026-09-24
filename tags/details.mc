// Local project-tag details and source-comment extraction.
#include <stdbool.h>
#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

enum {
	TagDetailCommentLines = 32,
	TagDetailCommentCap = 4096
};

static bool tag_detail_text_line(char *text, size_t text_len, int wanted,
	char **start_out, size_t *len_out) {
	size_t start = 0;
	size_t i = 0;
	int line = 1;
	if (text == NULL || wanted < 1 || start_out == NULL || len_out == NULL) {
		return false;
	}
	for (i = 0; i <= text_len; i++) {
		if (i == text_len || text[i] == '\n') {
			if (line == wanted) {
				size_t n = i - start;
				if (n > 0 && text[start + n - 1] == '\r') {
					n--;
				}
				*start_out = text + start;
				*len_out = n;
				return true;
			}
			line++;
			start = i + 1;
		}
	}
	return false;
}

static bool tag_detail_comment_part(char *line, size_t n, LangMode mode,
	char **part, size_t *part_len) {
	size_t start = 0;
	size_t end = n;
	if (line == NULL || part == NULL || part_len == NULL) {
		return false;
	}
	while (start < end && (line[start] == ' ' || line[start] == '\t')) {
		start++;
	}
	while (end > start && (line[end - 1] == ' ' || line[end - 1] == '\t')) {
		end--;
	}
	if (mode == LModePython) {
		if (start >= end || line[start] != '#') {
			return false;
		}
		start++;
	} else {
		if (end - start >= 2 && line[start] == '/' && line[start + 1] == '/') {
			start += 2;
		} else if (end - start >= 2 && line[start] == '/' &&
			line[start + 1] == '*') {
			start += 2;
		} else if (start < end && line[start] == '*') {
			start++;
			if (start < end && line[start] == '/') {
				start++;
			}
		} else {
			return false;
		}
		if (end - start >= 2 && line[end - 2] == '*' && line[end - 1] == '/') {
			end -= 2;
		}
	}
	while (start < end && (line[start] == ' ' || line[start] == '\t')) {
		start++;
	}
	while (end > start && (line[end - 1] == ' ' || line[end - 1] == '\t')) {
		end--;
	}
	*part = line + start;
	*part_len = end - start;
	return true;
}

static bool tag_detail_prepend(char *dst, size_t cap, char *part, size_t n) {
	size_t old = 0;
	size_t need = 0;
	if (dst == NULL || cap == 0 || part == NULL) {
		return false;
	}
	old = strlen(dst);
	need = n + (old > 0 ? 1 : 0) + old;
	if (need >= cap) {
		return false;
	}
	memmove(dst + n + (old > 0 ? 1 : 0), dst, old + 1);
	memcpy(dst, part, n);
	if (old > 0) {
		dst[n] = '\n';
	}
	return true;
}

static void tag_detail_source_comment(TagEntry *entry, char *dst, size_t cap) {
	LangMode mode = LModeNone;
	int line = 0;
	int used_lines = 0;
	if (entry == NULL || entry.path == NULL || dst == NULL || cap == 0) {
		return;
	}
	dst[0] = 0;
	auto (text, text_len, ok) = tags_read_file(entry.path);
	if (!ok) {
		return;
	}
	mode = mode_detect(str_from_cstr(entry.path));
	line = entry.line - 1;
	while (line >= 1 && used_lines < TagDetailCommentLines) {
		char *raw = NULL;
		size_t raw_len = 0;
		char *part = NULL;
		size_t part_len = 0;
		if (!tag_detail_text_line(text, text_len, line, &raw, &raw_len) ||
			!tag_detail_comment_part(raw, raw_len, mode, &part, &part_len)) {
			break;
		}
		if (!tag_detail_prepend(dst, cap, part, part_len)) {
			break;
		}
		line--;
		used_lines++;
	}
	free(text);
}

static Buffer *tag_detail_buffer() {
	Buffer *buf = buffer_find_by_name("*describe*");
	if (buf != NULL) {
		return buf;
	}
	buf = buffer_new();
	if (buf == NULL) {
		return NULL;
	}
	buf.set_name("*describe*");
	if (!buffer_list_add(buf)) {
		buf.free();
		return NULL;
	}
	return buf;
}

static Line *tag_detail_append_n(Buffer *buf, const char *text, size_t n) {
	Line *first = NULL;
	if (buf == NULL || text == NULL) {
		return NULL;
	}
	first = buf.line(1);
	if (buf.line_count_i() == 1 && first != NULL && first.byte_len() == 0) {
		Location at = {0};
		Location end = {0};
		at.line = 1;
		at.offset = 0;
		if (!buf.replace_raw(at, at, text, n, &end)) {
			return NULL;
		}
		return buf.line(1);
	}
	return buf.append_line(ranged(text, n, n));
}

static bool tag_detail_append(Buffer *buf, const char[..] text) {
	if (buf == NULL) {
		return false;
	}
	return tag_detail_append_n(buf, ptr(text), len(text)) != NULL;
}

static bool tag_detail_append_lines(Buffer *buf, char *text) {
	char *start = text;
	char *p = text;
	if (buf == NULL || text == NULL) {
		return false;
	}
	for (;;) {
		if (*p == '\n' || *p == 0) {
			if (tag_detail_append_n(buf, start, p - start) == NULL) {
				return false;
			}
			if (*p == 0) {
				return true;
			}
			start = p + 1;
		}
		p++;
	}
}

bool tags_show_details(TagEntry *entry) {
	Buffer *buf = NULL;
	char line[TagPathMax + 1024] = {0};
	char annotation[TagKindMax + TagNameMax * 2] = {0};
	char comment[TagDetailCommentCap] = {0};
	Line *source_line = NULL;
	Window *old = window_current();
	Window *shown = NULL;
	Location top = {0};
	if (entry == NULL || entry.name == NULL || entry.path == NULL) {
		return false;
	}
	buf = tag_detail_buffer();
	if (buf == NULL) {
		return false;
	}
	buf.is_readonly = false;
	buf.clear();
	buf.set_file_name(str_empty());
	buf.lang_mode = LModeMarkdown;
	buf.is_changed = false;
	snprintf(line, sizeof(line), "# %s", entry.name);
	if (!tag_detail_append(buf, str_from_cstr(line))) {
		return false;
	}
	tags_format_annotation(entry, annotation, sizeof(annotation));
	if (!tag_detail_append(buf, "") ||
		!tag_detail_append(buf, "## Kind") ||
		!tag_detail_append(buf, "")) {
		return false;
	}
	if (!tag_detail_append(buf, str_from_cstr(annotation))) {
		return false;
	}
	if (entry.signature != NULL && entry.signature[0] != 0) {
		if (!tag_detail_append(buf, "") ||
			!tag_detail_append(buf, "## Signature") ||
			!tag_detail_append(buf, "")) {
			return false;
		}
		snprintf(line, sizeof(line), "`%s%s`", entry.name, entry.signature);
		if (!tag_detail_append(buf, str_from_cstr(line))) {
			return false;
		}
	}
	tag_detail_source_comment(entry, comment, sizeof(comment));
	if (comment[0] != 0) {
		if (!tag_detail_append(buf, "") ||
			!tag_detail_append(buf, "## Documentation") ||
			!tag_detail_append(buf, "") ||
			!tag_detail_append_lines(buf, comment)) {
			return false;
		}
	}
	if (!tag_detail_append(buf, "") ||
		!tag_detail_append(buf, "## Source") ||
		!tag_detail_append(buf, "")) {
		return false;
	}
	snprintf(line, sizeof(line), "%s:%d", entry.path, entry.line);
	source_line = tag_detail_append_n(buf, line, strlen(line));
	if (source_line == NULL) {
		return false;
	}
	source_line.metadata = tags_loc_alloc(entry.path, entry.line, 1,
		entry.pattern);
	if (!tag_detail_append(buf, "") ||
		!tag_detail_append(buf, "## Provider") ||
		!tag_detail_append(buf, "") ||
		!tag_detail_append(buf, "project tags")) {
		return false;
	}
	buf.is_readonly = true;
	buf.is_changed = false;
	if (old != NULL) {
		markring_push(old.cursor);
	}
	if (!window_split()) {
		window_switch_buffer(buf);
	} else {
		window_switch_buffer(buf);
	}
	buffer_set_current(buf);
	shown = window_current();
	if (shown != NULL) {
		top.line = 1;
		top.offset = 0;
		shown.set_cursor(top);
		shown.top_line = 1;
		shown.should_redraw = true;
		shown.should_update_mode_line = true;
	}
	return true;
}

bool tags_describe_symbol() {
	char symbol[TagNameMax] = {0};
	TagList matches = {0};
	bool ok = false;
	if (!tags_symbol_at_point(symbol, sizeof(symbol))) {
		display_set_echo("[describe: no symbol at point]");
		return false;
	}
	if (!tags_ensure_loaded(false) ||
		!tags_collect_matches(symbol, &matches)) {
		return false;
	}
	if (len(matches.items) == 0) {
		display_set_echo("[describe: tag not found]");
		tags_list_free(&matches);
		return false;
	}
	ok = tags_show_details(matches.items[0]);
	tags_list_free(&matches);
	return ok;
}

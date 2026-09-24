import "buffer";

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

static void test_multiline_insert_undo() {
	Buffer *buf = buffer_new();
	UndoHistory history = {0};
	Location at = {0};
	Location end = {0};
	Location cursor = {0};
	Buffer *undone = NULL;
	char text[] = "first\nsecond";
	check(buf != NULL);
	at.line = 1;
	history.begin_command(buf, at);
	check(buf.insert_text(at, text, strlen(text), &end));
	history.record_edit(UndoInsert, at.line, at.offset, text, strlen(text));
	history.end_command();
	check(history.undo_once(&cursor, &undone));
	check(undone == buf);
	check(buf.line_count_i() == 1);
	check(buf.line(1).byte_len() == 0);
	buf.free();
}

static void test_large_undo_group() {
	Buffer *buf = buffer_new();
	UndoHistory history = {0};
	Location at = {0};
	Location cursor = {0};
	Buffer *undone = NULL;
	char text[] = "x";
	int i = 0;
	check(buf != NULL);
	at.line = 1;
	history.begin_command(buf, at);
	for (i = 0; i < 80; i++) {
		Location next = {0};
		check(buf.insert_text(at, text, 1, &next));
		history.record_edit(UndoInsert, at.line, at.offset, text, 1);
		at = next;
	}
	history.end_command();
	check(buf.line(1).byte_len() == 80);
	check(history.undo_once(&cursor, &undone));
	check(buf.line(1).byte_len() == 0);
	buf.free();
}

static void test_edit_invalidates_following_syntax() {
	Buffer *buf = buffer_new();
	Location at = {0};
	Location end = {0};
	char original[] = "one\ntwo\nthree";
	char edit[] = "x";
	int i = 0;
	check(buf != NULL);
	at.line = 1;
	check(buf.insert_text(at, original, strlen(original), &end));
	for (i = 1; i <= 3; i++) {
		buf.line(i).syntax_valid = true;
		buf.line(i).cache_valid = true;
	}
	end = at;
	end.offset = 1;
	check(buf.replace_raw(at, end, edit, 1, NULL));
	for (i = 1; i <= 3; i++) {
		check(!buf.line(i).syntax_valid);
		check(!buf.line(i).cache_valid);
	}
	buf.free();
}

int main() {
	test_multiline_insert_undo();
	test_large_undo_group();
	test_edit_invalidates_following_syntax();
	return 0;
}

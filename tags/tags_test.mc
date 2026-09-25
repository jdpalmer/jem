import "tags";
import "buffer";

#include <stdbool.h>
#include <stddef.h>
#include <stdlib.h>
#include <string.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

static void check_int(int actual, int expected) {
	check(actual == expected);
}

static void test_json_entry() {
	char json[] = "{\"_type\":\"tag\",\"name\":\"say\\\\\\\"hi\","
		"\"path\":\"src/a.mc\",\"kind\":\"function\","
		"\"pattern\":\"/^say_hi$/\",\"line\":42,"
		"\"signature\":\"(char *s)\",\"scope\":\"Api\","
		"\"typeref\":\"typename:void\","
		"\"extra\":{\"nested\":[true,null,3]}}";
	TagJsonFields fields = {0};
	int result = tag_json_parse_line(json, &fields);
	check_int(result, TagJsonEntry);
	check(strcmp(fields.name, "say\\\"hi") == 0);
	check(strcmp(fields.path, "src/a.mc") == 0);
	check(strcmp(fields.kind, "function") == 0);
	check(strcmp(fields.pattern, "/^say_hi$/") == 0);
	check_int(fields.line, 42);
	check(strcmp(fields.signature, "(char *s)") == 0);
	check(strcmp(fields.scope, "Api") == 0);
	check(strcmp(fields.typeref, "typename:void") == 0);
	tag_json_fields_free(&fields);
}

static void test_json_skip_and_errors() {
	char pseudo[] = "{\"_type\":\"ptag\",\"name\":\"TAG_FILE_FORMAT\"}";
	char malformed[] = "{\"_type\":\"tag\",\"name\":\"broken\"";
	char missing[] = "{\"_type\":\"tag\",\"name\":\"only-name\"}";
	TagJsonFields fields = {0};
	check_int(tag_json_parse_line(pseudo, &fields), TagJsonSkip);
	tag_json_fields_free(&fields);
	check_int(tag_json_parse_line(malformed, &fields),
		TagJsonMalformed);
	tag_json_fields_free(&fields);
	check_int(tag_json_parse_line(missing, &fields), TagJsonSkip);
	tag_json_fields_free(&fields);
}

static void test_stale_line_recovery() {
	Buffer *buf = buffer_new();
	Location at = {0};
	Location new_end = {0};
	char text[] = "unrelated\nanother\nvoid target() {\n}";
	char pattern[] = "/^void target() {$/";
	TagEntry entry = {0};
	check(buf != NULL);
	at.line = 1;
	at.offset = 0;
	check(buf.replace_raw(at, at, text, strlen(text), &new_end));
	entry.line = 1;
	entry.pattern = pattern;
	check_int(tags_resolve_line(buf, &entry), 3);
	buf.free();
}

static void test_path_normalization() {
	char a[] = "src/./tags/../tags/mod.mc";
	char b[] = "src/tags/mod.mc";
	char c[] = "src/tag/mod.mc";
	check(tags_paths_equal(a, b));
	check(!tags_paths_equal(a, c));
}

static void test_annotate() {
	TagEntry entry = {0};
	char ann[128] = {0};
	entry.kind = (char *)"member";
	entry.typeref = (char *)"typename:int";
	entry.scope = (char *)"main.Foo";
	tags_format_annotation(&entry, ann, sizeof(ann));
	check(strcmp(ann, "(member/int@main.Foo)") == 0);
	entry.typeref = NULL;
	tags_format_annotation(&entry, ann, sizeof(ann));
	check(strcmp(ann, "(member@main.Foo)") == 0);
}

int main() {
	test_json_entry();
	test_json_skip_and_errors();
	test_stale_line_recovery();
	test_path_normalization();
	test_annotate();
	return 0;
}

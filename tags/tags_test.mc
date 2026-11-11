import "tags";
import "buffer";
import "str";

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
	check(str_eq(fields.name, "say\\\"hi"));
	check(str_eq(fields.path, "src/a.mc"));
	check(str_eq(fields.kind, "function"));
	check(str_eq(fields.pattern, "/^say_hi$/"));
	check_int(fields.line, 42);
	check(str_eq(fields.signature, "(char *s)"));
	check(str_eq(fields.scope, "Api"));
	check(str_eq(fields.typeref, "typename:void"));
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
	Location new_end = {0};
	char text[] = "unrelated\nanother\nvoid target() {\n}";
	char pattern[] = "/^void target() {$/";
	TagEntry entry = {0};
	check(buf != NULL);
	defer buf.free();
	Location at = { .line = 1, .offset = 0 };
	check(buf.replace_raw(at, at, text, strlen(text), &new_end));
	entry.line = 1;
	entry.pattern = str_from_cstr(pattern);
	check_int(tags_resolve_line(buf, &entry), 3);
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
	check(str_set(&entry.kind, "member"));
	check(str_set(&entry.typeref, "typename:int"));
	check(str_set(&entry.scope, "main.Foo"));
	tags_format_annotation(&entry, ann, sizeof(ann));
	check(strcmp(ann, "(member/int@main.Foo)") == 0);
	check(str_set(&entry.typeref, ""));
	tags_format_annotation(&entry, ann, sizeof(ann));
	check(strcmp(ann, "(member@main.Foo)") == 0);
	str_free(&entry.kind);
	str_free(&entry.typeref);
	str_free(&entry.scope);
}

int main() {
	test_json_entry();
	test_json_skip_and_errors();
	test_stale_line_recovery();
	test_path_normalization();
	test_annotate();
	return 0;
}

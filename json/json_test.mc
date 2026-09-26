// json package smoke tests (pure %C port of jem.c util_json).
import "json";

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int g_failures;

static void check(bool value, const char *msg) {
	if (!value) {
		fprintf(stderr, "FAIL: %s\n", msg);
		g_failures++;
	}
}

static void test_roundtrip_object() {
	const char *src =
		"{\"fill-column\":100,\"theme-mode\":1,\"keybindings\":{\"C-l\":null}}";
	JsonError err = {0};
	JsonValue *v = NULL;
	char *out = NULL;
	const JsonValue *kb = NULL;
	const JsonValue *fc = NULL;
	double n = 0.0;

	v = json_parse(src, &err);
	check(v != NULL, "roundtrip parse");
	if (v == NULL) {
		fprintf(stderr, "  err=%s line=%zu col=%zu\n",
			err.message != NULL ? err.message : "(null)",
			err.line, err.column);
		return;
	}
	check(json_type(v) == JSON_OBJECT, "roundtrip type object");
	fc = json_object_get(v, "fill-column");
	check(fc != NULL, "fill-column present");
	check(json_number_value(fc, &n), "fill-column number");
	check(n == 100.0, "fill-column == 100");
	kb = json_object_get(v, "keybindings");
	check(kb != NULL, "keybindings present");
	check(json_type(kb) == JSON_OBJECT, "keybindings object");
	out = json_stringify(v);
	check(out != NULL, "stringify");
	if (out != NULL) {
		check(strstr(out, "fill-column") != NULL, "stringify has key");
		free(out);
	}
	json_free(v);
}

static void test_bad_trailing() {
	JsonError err = {0};
	JsonValue *v = json_parse("true false", &err);
	check(v == NULL, "trailing reject");
	check(err.message != NULL, "trailing error message");
}

static void test_escape_unicode() {
	JsonError err = {0};
	JsonValue *v = json_parse("\"\\u00e9\"", &err);
	const char *s = NULL;
	check(v != NULL, "unicode parse");
	if (v == NULL) {
		fprintf(stderr, "  err=%s\n",
			err.message != NULL ? err.message : "(null)");
		return;
	}
	check(json_type(v) == JSON_STRING, "unicode string type");
	s = json_string_value(v);
	check(s != NULL, "unicode string value");
	check(json_string_length(v) == 2, "unicode length 2");
	check(memcmp(s, "\xC3\xA9", 2) == 0, "unicode utf8 bytes");
	json_free(v);
}

static void test_corpus_bytes(const char *text, size_t n, bool expect_ok,
	const char *label) {
	JsonError err = {0};
	JsonValue *v = NULL;

	v = json_parse_n(text, n, &err);
	if (expect_ok) {
		check(v != NULL, label);
		if (v != NULL) {
			json_free(v);
		}
	} else {
		check(v == NULL, label);
	}
}

int main() {
	const char *good = "{\"menu\":{\"id\":\"file\",\"value\":\"File\"}}";
	const char *bad = "{\"a\":1";

	test_roundtrip_object();
	test_bad_trailing();
	test_escape_unicode();
	test_corpus_bytes(good, strlen(good), true, "good object");
	test_corpus_bytes(bad, strlen(bad), false, "bad truncated");
	if (g_failures != 0) {
		fprintf(stderr, "%d failures\n", g_failures);
		return 1;
	}
	return 0;
}

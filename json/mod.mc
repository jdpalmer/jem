// json — DOM JSON parser/serializer (pure %C port of jem.c util_json).
import "str";

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

#pragma modc c_libs(m)

enum JsonType {
	JSON_NULL = 0,
	JSON_BOOL = 1,
	JSON_NUMBER = 2,
	JSON_STRING = 3,
	JSON_ARRAY = 4,
	JSON_OBJECT = 5
};

struct JsonValue;

struct JsonArray {
	JsonValue *[..] items; /* owned heap of JsonValue* */
};

struct JsonObjectEntry {
	char[..] key;
	JsonValue *value;
};

struct JsonObject {
	JsonObjectEntry[..] entries; /* owned heap */
};

/* Flat payload: only the field matching `type` is live. Avoids %C union pitfalls. */
struct JsonValue {
	JsonType type;
	bool boolean;
	double number;
	char[..] string;
	JsonArray array;
	JsonObject object;
};

struct JsonError {
	const char *message;
	size_t line;
	size_t column;
};

typedef bool (*JsonWriteFn)(void *ctx, const char *buf, size_t len);

enum {
	JsonMaxParseDepth = 256
};

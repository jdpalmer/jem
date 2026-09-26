// json/parse.mc — recursive-descent JSON parser.
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

struct JsonParser {
	const char *cursor;
	const char *end;
	JsonError error;
	size_t line;
	size_t column;
	size_t depth;
};

static void json_set_error(JsonParser *parser, const char *message) {
	if (parser == NULL || message == NULL) {
		return;
	}
	if (parser.error.message != NULL) {
		return;
	}
	parser.error.message = message;
	parser.error.line = parser.line;
	parser.error.column = parser.column;
}

static void json_copy_error(JsonError *dst, const JsonError *src) {
	if (dst == NULL) {
		return;
	}
	if (src != NULL && src.message != NULL) {
		*dst = *src;
	} else {
		dst.message = NULL;
		dst.line = 0;
		dst.column = 0;
	}
}

static char json_peek(const JsonParser *parser) {
	if (parser == NULL || parser.cursor >= parser.end) {
		return 0;
	}
	return *parser.cursor;
}

static bool json_take(JsonParser *parser, char *out) {
	char c = 0;

	if (parser == NULL) {
		return false;
	}
	if (parser.cursor >= parser.end) {
		json_set_error(parser, "Unexpected end of input");
		return false;
	}
	c = *parser.cursor;
	parser.cursor++;
	if (out != NULL) {
		*out = c;
	}
	if (c == '\n') {
		parser.line++;
		parser.column = 1;
	} else {
		parser.column++;
	}
	return true;
}

static void json_skip_ws(JsonParser *parser) {
	char c = 0;

	if (parser == NULL) {
		return;
	}
	for (;;) {
		c = json_peek(parser);
		if (c != ' ' && c != '\t' && c != '\r' && c != '\n') {
			return;
		}
		(void)json_take(parser, NULL);
	}
}

static bool json_expect(JsonParser *parser, char expected, const char *message) {
	char c = 0;

	if (!json_take(parser, &c)) {
		return false;
	}
	if (c != expected) {
		json_set_error(parser, message);
		return false;
	}
	return true;
}

static bool json_consume_literal(JsonParser *parser, const char *literal) {
	if (parser == NULL || literal == NULL) {
		return false;
	}
	while (*literal != 0) {
		char c = 0;
		if (!json_take(parser, &c)) {
			return false;
		}
		if (c != *literal) {
			json_set_error(parser, "Invalid literal");
			return false;
		}
		literal++;
	}
	return true;
}

static bool json_parse_hex4(JsonParser *parser, uint16_t *value) {
	uint16_t acc = 0;
	int i = 0;

	if (parser == NULL || value == NULL) {
		return false;
	}
	for (i = 0; i < 4; i++) {
		char c = 0;
		unsigned int nibble = 0;
		if (!json_take(parser, &c)) {
			return false;
		}
		if (c >= '0' && c <= '9') {
			nibble = c - '0';
		} else if (c >= 'a' && c <= 'f') {
			nibble = c - 'a' + 10;
		} else if (c >= 'A' && c <= 'F') {
			nibble = c - 'A' + 10;
		} else {
			json_set_error(parser, "Invalid hex escape");
			return false;
		}
		acc = (uint16_t)((acc << 4) | nibble);
	}
	*value = acc;
	return true;
}

static bool json_append_utf8(JsonBuffer *buffer, uint32_t codepoint) {
	char bytes[4] = {0};
	size_t len = json_utf8_encode(codepoint, bytes);
	return json_buffer_append_bytes(buffer, bytes, len);
}

static bool json_parse_string_into(JsonParser *parser, JsonBuffer *buffer) {
	char c = 0;

	if (parser == NULL || buffer == NULL) {
		return false;
	}
	if (!json_expect(parser, '"', "Expected string")) {
		return false;
	}
	for (;;) {
		const char *char_start = NULL;
		uint32_t codepoint = 0;
		size_t used = 0;
		size_t available = 0;
		size_t remaining = 0;

		if (!json_take(parser, &c)) {
			return false;
		}
		if (c == '"') {
			return true;
		}
		if ((c & 0xFF) < 0x20) {
			json_set_error(parser, "Unescaped control character in string");
			return false;
		}
		if (c != '\\') {
			if ((c & 0xFF) < 0x80) {
				if (!json_buffer_append_char(buffer, c)) {
					json_set_error(parser, "Out of memory");
					return false;
				}
				continue;
			}
			char_start = parser.cursor - 1;
			remaining = parser.end - char_start;
			available = remaining >= 4 ? 4 : remaining;
			used = json_utf8_decode_strict(char_start, available, &codepoint);
			if (used == 0) {
				json_set_error(parser, "Invalid UTF-8 in string");
				return false;
			}
			if (!json_buffer_append_bytes(buffer, char_start, used)) {
				json_set_error(parser, "Out of memory");
				return false;
			}
			parser.cursor = parser.cursor + (used - 1);
			parser.column = parser.column + (used - 1);
			continue;
		}
		if (!json_take(parser, &c)) {
			return false;
		}
		switch (c) {
		case '"':
		case '\\':
		case '/':
			if (!json_buffer_append_char(buffer, c)) {
				json_set_error(parser, "Out of memory");
				return false;
			}
			break;
		case 'b':
			if (!json_buffer_append_char(buffer, '\b')) {
				json_set_error(parser, "Out of memory");
				return false;
			}
			break;
		case 'f':
			if (!json_buffer_append_char(buffer, '\f')) {
				json_set_error(parser, "Out of memory");
				return false;
			}
			break;
		case 'n':
			if (!json_buffer_append_char(buffer, '\n')) {
				json_set_error(parser, "Out of memory");
				return false;
			}
			break;
		case 'r':
			if (!json_buffer_append_char(buffer, '\r')) {
				json_set_error(parser, "Out of memory");
				return false;
			}
			break;
		case 't':
			if (!json_buffer_append_char(buffer, '\t')) {
				json_set_error(parser, "Out of memory");
				return false;
			}
			break;
		case 'u': {
			uint16_t high = 0;
			uint16_t low = 0;
			uint32_t escaped = 0;
			if (!json_parse_hex4(parser, &high)) {
				return false;
			}
			if (high >= 0xD800 && high <= 0xDBFF) {
				if (!json_expect(parser, '\\', "Expected UTF-16 surrogate pair") ||
					!json_expect(parser, 'u', "Expected UTF-16 surrogate pair") ||
					!json_parse_hex4(parser, &low)) {
					json_set_error(parser, "Expected UTF-16 surrogate pair");
					return false;
				}
				if (low < 0xDC00 || low > 0xDFFF) {
					json_set_error(parser, "Invalid UTF-16 surrogate pair");
					return false;
				}
			} else if (high >= 0xDC00 && high <= 0xDFFF) {
				json_set_error(parser, "Invalid UTF-16 surrogate pair");
				return false;
			}
			if (low == 0) {
				escaped = high;
			} else {
				escaped = (((uint32_t)high - 0xD800) << 10) +
					((uint32_t)low - 0xDC00) + 0x10000;
			}
			if (!json_append_utf8(buffer, escaped)) {
				json_set_error(parser, "Out of memory");
				return false;
			}
			break;
		}
		default:
			json_set_error(parser, "Invalid escape sequence");
			return false;
		}
	}
}

static JsonValue *json_parse_string_value(JsonParser *parser) {
	JsonBuffer buffer = {0};
	JsonValue *value = NULL;

	if (!json_parse_string_into(parser, &buffer)) {
		json_buffer_discard(&buffer);
		return NULL;
	}
	value = json_value_alloc(JSON_STRING);
	if (value == NULL) {
		json_buffer_discard(&buffer);
		json_set_error(parser, "Out of memory");
		return NULL;
	}
	if (!json_string_take_buffer(&value.string, &buffer)) {
		free(value);
		json_set_error(parser, "Out of memory");
		return NULL;
	}
	return value;
}

static JsonValue *json_parse_number_value(JsonParser *parser) {
	JsonScanner s = {0};
	double number = 0.0;
	size_t len = 0;
	JsonValue *value = NULL;

	if (parser == NULL) {
		return NULL;
	}
	s.p = parser.cursor;
	if (!json_scan_double(&s, &number)) {
		json_set_error(parser, "Invalid number format or overflow");
		return NULL;
	}
	if (json_is_number_char(*s.p)) {
		json_set_error(parser, "Invalid number format");
		return NULL;
	}
	len = s.p - parser.cursor;
	parser.cursor = s.p;
	parser.column = parser.column + len;
	value = json_new_number(number);
	if (value == NULL) {
		json_set_error(parser, "Out of memory");
	}
	return value;
}

static JsonValue *json_parse_object(JsonParser *parser) {
	JsonValue *object = NULL;

	object = json_new_object();
	if (object == NULL) {
		json_set_error(parser, "Out of memory");
		return NULL;
	}
	if (parser.depth >= JsonMaxParseDepth) {
		json_set_error(parser, "JSON nesting too deep");
		json_free(object);
		return NULL;
	}
	parser.depth++;
	if (!json_expect(parser, '{', "Expected '{'")) {
		parser.depth--;
		json_free(object);
		return NULL;
	}
	json_skip_ws(parser);
	if (json_peek(parser) == '}') {
		(void)json_take(parser, NULL);
		parser.depth--;
		return object;
	}
	for (;;) {
		JsonBuffer key = {0};
		JsonString owned_key = {0};
		JsonValue *value = NULL;

		if (!json_parse_string_into(parser, &key)) {
			json_buffer_discard(&key);
			parser.depth--;
			json_free(object);
			return NULL;
		}
		json_skip_ws(parser);
		if (!json_expect(parser, ':', "Expected ':' after object key")) {
			json_buffer_discard(&key);
			parser.depth--;
			json_free(object);
			return NULL;
		}
		value = json_parse_value(parser);
		if (value == NULL) {
			json_buffer_discard(&key);
			parser.depth--;
			json_free(object);
			return NULL;
		}
		if (!json_string_take_buffer(&owned_key, &key)) {
			json_free(value);
			parser.depth--;
			json_free(object);
			json_set_error(parser, "Out of memory");
			return NULL;
		}
		if (!json_object_put_owned(object, owned_key, value)) {
			free(owned_key.data);
			json_free(value);
			parser.depth--;
			json_free(object);
			json_set_error(parser, "Out of memory");
			return NULL;
		}
		json_skip_ws(parser);
		if (json_peek(parser) == '}') {
			(void)json_take(parser, NULL);
			parser.depth--;
			return object;
		}
		if (!json_expect(parser, ',', "Expected ',' or '}' after object value")) {
			parser.depth--;
			json_free(object);
			return NULL;
		}
		json_skip_ws(parser);
	}
}

static JsonValue *json_parse_array(JsonParser *parser) {
	JsonValue *array = NULL;

	array = json_new_array();
	if (array == NULL) {
		json_set_error(parser, "Out of memory");
		return NULL;
	}
	if (parser.depth >= JsonMaxParseDepth) {
		json_set_error(parser, "JSON nesting too deep");
		json_free(array);
		return NULL;
	}
	parser.depth++;
	if (!json_expect(parser, '[', "Expected '['")) {
		parser.depth--;
		json_free(array);
		return NULL;
	}
	json_skip_ws(parser);
	if (json_peek(parser) == ']') {
		(void)json_take(parser, NULL);
		parser.depth--;
		return array;
	}
	for (;;) {
		JsonValue *item = json_parse_value(parser);
		if (item == NULL) {
			parser.depth--;
			json_free(array);
			return NULL;
		}
		if (!json_array_append(array, item)) {
			json_free(item);
			parser.depth--;
			json_free(array);
			json_set_error(parser, "Out of memory");
			return NULL;
		}
		json_skip_ws(parser);
		if (json_peek(parser) == ']') {
			(void)json_take(parser, NULL);
			parser.depth--;
			return array;
		}
		if (!json_expect(parser, ',', "Expected ',' or ']' after array value")) {
			parser.depth--;
			json_free(array);
			return NULL;
		}
		json_skip_ws(parser);
	}
}

static JsonValue *json_parse_value(JsonParser *parser) {
	char c = 0;

	json_skip_ws(parser);
	c = json_peek(parser);
	switch (c) {
	case '{':
		return json_parse_object(parser);
	case '[':
		return json_parse_array(parser);
	case '"':
		return json_parse_string_value(parser);
	case 't': {
		JsonValue *v = NULL;
		if (!json_consume_literal(parser, "true")) {
			return NULL;
		}
		v = json_new_bool(true);
		if (v == NULL) {
			json_set_error(parser, "Out of memory");
		}
		return v;
	}
	case 'f': {
		JsonValue *v = NULL;
		if (!json_consume_literal(parser, "false")) {
			return NULL;
		}
		v = json_new_bool(false);
		if (v == NULL) {
			json_set_error(parser, "Out of memory");
		}
		return v;
	}
	case 'n': {
		JsonValue *v = NULL;
		if (!json_consume_literal(parser, "null")) {
			return NULL;
		}
		v = json_new_null();
		if (v == NULL) {
			json_set_error(parser, "Out of memory");
		}
		return v;
	}
	case 0:
		json_set_error(parser, "Expected JSON value");
		return NULL;
	default:
		if (c == '-' || json_byte_digit(c)) {
			return json_parse_number_value(parser);
		}
		break;
	}
	json_set_error(parser, "Invalid JSON value");
	return NULL;
}

JsonValue *json_parse_n(const char *text, size_t len, JsonError *error) {
	JsonParser parser = {0};
	JsonValue *value = NULL;

	if (text == NULL) {
		return NULL;
	}
	parser.cursor = text;
	parser.end = text + len;
	parser.error.message = NULL;
	parser.error.line = 1;
	parser.error.column = 1;
	parser.line = 1;
	parser.column = 1;
	parser.depth = 0;
	value = json_parse_value(&parser);
	if (value == NULL) {
		json_copy_error(error, &parser.error);
		return NULL;
	}
	json_skip_ws(&parser);
	if (json_peek(&parser) != 0) {
		json_free(value);
		json_set_error(&parser, "Trailing characters after JSON value");
		json_copy_error(error, &parser.error);
		return NULL;
	}
	json_copy_error(error, NULL);
	return value;
}

JsonValue *json_parse(const char *text, JsonError *error) {
	if (text == NULL) {
		return NULL;
	}
	return json_parse_n(text, strlen(text), error);
}

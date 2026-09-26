// json/write.mc — stringify / json_write.
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

static bool json_append_escaped_string(JsonBuffer *buffer, const char *value,
	size_t len) {
	size_t i = 0;

	if (buffer == NULL || value == NULL) {
		return false;
	}
	if (!json_buffer_append_char(buffer, '"')) {
		return false;
	}
	while (i < len) {
		char c = value[i];
		uint32_t codepoint = 0;
		size_t available = 0;
		size_t used = 0;

		switch (c) {
		case '"':
			if (!json_buffer_append_bytes(buffer, "\\\"", 2)) {
				return false;
			}
			i++;
			break;
		case '\\':
			if (!json_buffer_append_bytes(buffer, "\\\\", 2)) {
				return false;
			}
			i++;
			break;
		case '\b':
			if (!json_buffer_append_bytes(buffer, "\\b", 2)) {
				return false;
			}
			i++;
			break;
		case '\f':
			if (!json_buffer_append_bytes(buffer, "\\f", 2)) {
				return false;
			}
			i++;
			break;
		case '\n':
			if (!json_buffer_append_bytes(buffer, "\\n", 2)) {
				return false;
			}
			i++;
			break;
		case '\r':
			if (!json_buffer_append_bytes(buffer, "\\r", 2)) {
				return false;
			}
			i++;
			break;
		case '\t':
			if (!json_buffer_append_bytes(buffer, "\\t", 2)) {
				return false;
			}
			i++;
			break;
		default:
			if ((c & 0xFF) < 0x20) {
				char escape[6] = {0};
				escape[0] = '\\';
				escape[1] = 'u';
				escape[2] = '0';
				escape[3] = '0';
				escape[4] = json_hex_digit((c & 0xFF) >> 4);
				escape[5] = json_hex_digit(c & 0x0F);
				if (!json_buffer_append_bytes(buffer, escape, 6)) {
					return false;
				}
				i++;
				break;
			}
			if ((c & 0xFF) < 0x80) {
				if (!json_buffer_append_char(buffer, c)) {
					return false;
				}
				i++;
				break;
			}
			available = (len - i) >= 4 ? 4 : (len - i);
			used = json_utf8_decode_strict(value + i, available, &codepoint);
			if (used == 0) {
				return false;
			}
			if (!json_buffer_append_bytes(buffer, value + i, used)) {
				return false;
			}
			i = i + used;
			break;
		}
	}
	return json_buffer_append_char(buffer, '"');
}

static bool json_stringify_value(JsonBuffer *buffer, const JsonValue *value) {
	size_t i = 0;

	if (buffer == NULL || value == NULL) {
		return false;
	}
	switch (value.type) {
	case JSON_NULL:
		return json_buffer_append_bytes(buffer, "null", 4);
	case JSON_BOOL:
		if (value.boolean) {
			return json_buffer_append_bytes(buffer, "true", 4);
		}
		return json_buffer_append_bytes(buffer, "false", 5);
	case JSON_NUMBER:
		if (!isfinite(value.number)) {
			return json_buffer_append_bytes(buffer, "null", 4);
		}
		return json_buffer_append_number(buffer, value.number);
	case JSON_STRING:
		return json_append_escaped_string(buffer, value.string.data,
			value.string.length);
	case JSON_ARRAY:
		if (!json_buffer_append_char(buffer, '[')) {
			return false;
		}
		for (i = 0; i < value.array.count; i++) {
			if (i != 0 && !json_buffer_append_char(buffer, ',')) {
				return false;
			}
			if (!json_stringify_value(buffer, value.array.items[i])) {
				return false;
			}
		}
		return json_buffer_append_char(buffer, ']');
	case JSON_OBJECT:
		if (!json_buffer_append_char(buffer, '{')) {
			return false;
		}
		for (i = 0; i < value.object.count; i++) {
			if (i != 0 && !json_buffer_append_char(buffer, ',')) {
				return false;
			}
			if (!json_append_escaped_string(buffer,
				value.object.entries[i].key.data,
				value.object.entries[i].key.length)) {
				return false;
			}
			if (!json_buffer_append_char(buffer, ':')) {
				return false;
			}
			if (!json_stringify_value(buffer, value.object.entries[i].value)) {
				return false;
			}
		}
		return json_buffer_append_char(buffer, '}');
	}
	return false;
}

char *json_stringify(const JsonValue *value) {
	JsonBuffer buffer = {0};

	if (value == NULL) {
		return NULL;
	}
	if (!json_stringify_value(&buffer, value)) {
		json_buffer_discard(&buffer);
		return NULL;
	}
	return json_buffer_take(&buffer);
}

bool json_write(const JsonValue *value, JsonWriteFn writer, void *ctx) {
	char *text = NULL;
	size_t len = 0;
	bool ok = false;

	if (value == NULL || writer == NULL) {
		return false;
	}
	text = json_stringify(value);
	if (text == NULL) {
		return false;
	}
	len = strlen(text);
	ok = writer(ctx, text, len);
	free(text);
	return ok;
}

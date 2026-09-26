// json/util.mc — buffers, UTF-8, number scan (package-private).
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

struct JsonBuffer {
	char *data;
	size_t length;
	size_t capacity;
};

struct JsonScanner {
	const char *p;
};

static bool json_byte_digit(char c) {
	return c >= '0' && c <= '9';
}

static bool json_is_number_char(char c) {
	return json_byte_digit(c) || c == '.' || c == '+' || c == '-' ||
		c == 'e' || c == 'E';
}

static size_t json_utf8_encode(uint32_t codepoint, char *out) {
	if ((codepoint >= 0xD800 && codepoint <= 0xDFFF) || codepoint > 0x10FFFF) {
		codepoint = 0xFFFD;
	}
	if (codepoint < 0x80) {
		out[0] = (char)codepoint;
		return 1;
	}
	if (codepoint < 0x800) {
		out[0] = (char)(0xC0 | (codepoint >> 6));
		out[1] = (char)(0x80 | (codepoint & 0x3F));
		return 2;
	}
	if (codepoint < 0x10000) {
		out[0] = (char)(0xE0 | (codepoint >> 12));
		out[1] = (char)(0x80 | ((codepoint >> 6) & 0x3F));
		out[2] = (char)(0x80 | (codepoint & 0x3F));
		return 3;
	}
	out[0] = (char)(0xF0 | (codepoint >> 18));
	out[1] = (char)(0x80 | ((codepoint >> 12) & 0x3F));
	out[2] = (char)(0x80 | ((codepoint >> 6) & 0x3F));
	out[3] = (char)(0x80 | (codepoint & 0x3F));
	return 4;
}

static bool json_utf8_cont(int c) {
	return (c & 0xC0) == 0x80;
}

static size_t json_utf8_decode_strict(const char *text, size_t len, uint32_t *codepoint) {
	const char *bytes = text;
	uint32_t value = 0;
	uint32_t min_value = 0;
	size_t need = 0;
	size_t i = 0;
	uint32_t b0 = 0;

	if (len == 0 || codepoint == NULL) {
		return 0;
	}
	b0 = bytes[0];
	if (b0 < 0x80) {
		*codepoint = b0;
		return 1;
	}
	if (b0 < 0xC2 || b0 > 0xF4) {
		return 0;
	}
	if (b0 < 0xE0) {
		value = b0 & 0x1F;
		min_value = 0x80;
		need = 2;
	} else if (b0 < 0xF0) {
		value = b0 & 0x0F;
		min_value = 0x800;
		need = 3;
	} else {
		value = b0 & 0x07;
		min_value = 0x10000;
		need = 4;
	}
	if (len < need) {
		return 0;
	}
	for (i = 1; i < need; i++) {
		uint32_t bi = bytes[i];
		if (!json_utf8_cont(bi)) {
			return 0;
		}
		value = (value << 6) | (bi & 0x3F);
	}
	if (value < min_value) {
		return 0;
	}
	if ((value >= 0xD800 && value <= 0xDFFF) || value > 0x10FFFF) {
		return 0;
	}
	*codepoint = value;
	return need;
}

static bool json_utf8_validate(const char *text, size_t len) {
	size_t offset = 0;
	uint32_t codepoint = 0;

	while (offset < len) {
		size_t used = json_utf8_decode_strict(text + offset, len - offset, &codepoint);
		if (used == 0) {
			return false;
		}
		offset = offset + used;
	}
	return true;
}

/* JSON-style double scan (jem.c util_scan scan_double); no __builtin overflow. */
static bool json_scan_double(JsonScanner *s, double *out) {
	const char *p = NULL;
	double sign = 1.0;
	uint64_t significand = 0;
	int32_t exponent_offset = 0;
	bool has_digits = false;
	int32_t i = 0;

	if (s == NULL || s.p == NULL || out == NULL) {
		return false;
	}
	p = s.p;
	if (*p == '-') {
		sign = -1.0;
		p++;
	}
	if (!json_byte_digit(*p)) {
		return false;
	}
	while (json_byte_digit(*p)) {
		uint64_t digit = *p - '0';
		if (significand > (((uint64_t)0 - 1) / 10)) {
			return false;
		}
		significand = significand * 10;
		if (significand > (((uint64_t)0 - 1) - digit)) {
			return false;
		}
		significand = significand + digit;
		has_digits = true;
		p++;
	}
	if (*p == '.') {
		p++;
		if (!json_byte_digit(*p)) {
			return false;
		}
		while (json_byte_digit(*p)) {
			uint64_t digit = *p - '0';
			if (significand > (((uint64_t)0 - 1) / 10)) {
				return false;
			}
			significand = significand * 10;
			if (significand > (((uint64_t)0 - 1) - digit)) {
				return false;
			}
			significand = significand + digit;
			exponent_offset--;
			p++;
		}
	}
	if (*p == 'e' || *p == 'E') {
		int32_t exp_sign = 1;
		int32_t parsed_exp = 0;
		p++;
		if (*p == '-') {
			exp_sign = -1;
			p++;
		} else if (*p == '+') {
			p++;
		}
		if (!json_byte_digit(*p)) {
			return false;
		}
		while (json_byte_digit(*p)) {
			int32_t digit = *p - '0';
			if (parsed_exp > (2147483647 / 10)) {
				return false;
			}
			parsed_exp = parsed_exp * 10 + digit;
			p++;
		}
		exponent_offset = exponent_offset + (parsed_exp * exp_sign);
	}
	if (!has_digits) {
		return false;
	}
	*out = sign * (double)significand * pow(10.0, exponent_offset);
	s.p = p;
	(void)i;
	return true;
}

static bool json_buffer_reserve(JsonBuffer *buffer, size_t needed) {
	size_t target = 0;
	size_t new_capacity = 0;
	char *grown = NULL;

	if (buffer == NULL) {
		return false;
	}
	target = needed + 1;
	if (buffer.capacity >= target) {
		return true;
	}
	new_capacity = buffer.capacity == 0 ? 32 : buffer.capacity;
	while (new_capacity < target) {
		if (new_capacity > (((size_t)0 - 1) / 2)) {
			return false;
		}
		new_capacity = new_capacity * 2;
	}
	grown = realloc(buffer.data, new_capacity);
	if (grown == NULL) {
		return false;
	}
	buffer.data = grown;
	buffer.capacity = new_capacity;
	return true;
}

static bool json_buffer_append_char(JsonBuffer *buffer, char c) {
	if (buffer == NULL) {
		return false;
	}
	if (!json_buffer_reserve(buffer, buffer.length + 1)) {
		return false;
	}
	buffer.data[buffer.length] = c;
	buffer.length++;
	buffer.data[buffer.length] = 0;
	return true;
}

static bool json_buffer_append_bytes(JsonBuffer *buffer, const char *bytes, size_t len) {
	if (buffer == NULL) {
		return false;
	}
	if (bytes == NULL && len != 0) {
		return false;
	}
	if (!json_buffer_reserve(buffer, buffer.length + len)) {
		return false;
	}
	if (len != 0) {
		memcpy(buffer.data + buffer.length, bytes, len);
		buffer.length = buffer.length + len;
	}
	buffer.data[buffer.length] = 0;
	return true;
}

static bool json_buffer_append_number(JsonBuffer *buffer, double number) {
	char stack_buf[64] = {0};
	char *text = stack_buf;
	int written = 0;
	bool ok = false;

	if (buffer == NULL) {
		return false;
	}
	written = snprintf(stack_buf, sizeof(stack_buf), "%.17g", number);
	if (written < 0) {
		return false;
	}
	if ((size_t)written >= sizeof(stack_buf)) {
		text = malloc((size_t)written + 1);
		if (text == NULL) {
			return false;
		}
		written = snprintf(text, (size_t)written + 1, "%.17g", number);
		if (written < 0) {
			free(text);
			return false;
		}
	}
	ok = json_buffer_append_bytes(buffer, text, written);
	if (text != stack_buf) {
		free(text);
	}
	return ok;
}

static char *json_buffer_take(JsonBuffer *buffer) {
	char *data = NULL;

	if (buffer == NULL) {
		return NULL;
	}
	if (buffer.data == NULL) {
		data = calloc(1, 1);
		return data;
	}
	data = buffer.data;
	buffer.data = NULL;
	buffer.length = 0;
	buffer.capacity = 0;
	return data;
}

static void json_buffer_discard(JsonBuffer *buffer) {
	if (buffer == NULL) {
		return;
	}
	free(buffer.data);
	buffer.data = NULL;
	buffer.length = 0;
	buffer.capacity = 0;
}

static char *json_dup_bytes(const char *src, size_t len) {
	char *dst = NULL;

	if (src == NULL && len != 0) {
		return NULL;
	}
	dst = calloc(len + 1, 1);
	if (dst == NULL) {
		return NULL;
	}
	if (len != 0) {
		memcpy(dst, src, len);
	}
	return dst;
}

static bool json_grow_ptr_array(void **items, size_t *capacity, size_t item_size,
	size_t needed) {
	void *grown = NULL;
	size_t new_capacity = 0;

	if (items == NULL || capacity == NULL || item_size == 0) {
		return false;
	}
	if (*capacity >= needed) {
		return true;
	}
	new_capacity = *capacity == 0 ? 4 : *capacity;
	while (new_capacity < needed) {
		if (new_capacity > (((size_t)0 - 1) / 2)) {
			return false;
		}
		new_capacity = new_capacity * 2;
	}
	if (new_capacity > (((size_t)0 - 1) / item_size)) {
		return false;
	}
	grown = realloc(*items, new_capacity * item_size);
	if (grown == NULL) {
		return false;
	}
	*items = grown;
	*capacity = new_capacity;
	return true;
}

static char json_hex_digit(unsigned int value) {
	if (value < 10) {
		return (char)('0' + value);
	}
	return (char)('a' + value - 10);
}

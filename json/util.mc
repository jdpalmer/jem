// json/util.mc — buffers, UTF-8, number scan (package-private).
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

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

static size_t json_utf8_encode(uint32_t codepoint, char *out_raw) {
	char[..] out = ranged(out_raw, 4);
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
	const char[..] bytes = ranged(text, len);
	size_t i = 1;

	if (len == 0 || codepoint == NULL) {
		return 0;
	}
	uint32_t b0 = bytes[0];
	if (b0 < 0x80) {
		*codepoint = b0;
		return 1;
	}
	if (b0 < 0xC2 || b0 > 0xF4) {
		return 0;
	}
	uint32_t value = b0 & 0x1F;
	uint32_t min_value = 0x80;
	size_t need = 2;
	if (b0 >= 0xE0 && b0 < 0xF0) {
		value = b0 & 0x0F;
		min_value = 0x800;
		need = 3;
	} else if (b0 >= 0xF0) {
		value = b0 & 0x07;
		min_value = 0x10000;
		need = 4;
	}
	if (len < need) {
		return 0;
	}
	for (; i < need; i++) {
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
	double sign = 1.0;
	uint64_t significand = 0;
	int32_t exponent_offset = 0;
	bool has_digits = false;
	int32_t i = 0;

	if (s == NULL || s.p == NULL || out == NULL) {
		return false;
	}
	const char *p = s.p;
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

/* Ensure capacity for a trailing NUL without growing len (C-boundary handoff). */
static bool json_ensure_cstr(char[..] *s) {
	if (s == NULL) {
		return false;
	}
	size_t n = len(*s);
	if (!str_reserve(s, n + 1)) {
		return false;
	}
	{
		char[..] z = ranged(ptr(*s), n + 1, cap(*s));
		z[n] = 0;
	}
	return true;
}

static bool json_buffer_append_char(char[..] *buffer, char c) {
	if (buffer == NULL) {
		return false;
	}
	return str_append_byte(buffer, c);
}

static bool json_buffer_append_bytes(char[..] *buffer, const char *bytes, size_t nbytes) {
	if (buffer == NULL) {
		return false;
	}
	if (bytes == NULL && nbytes != 0) {
		return false;
	}
	if (nbytes == 0) {
		return true;
	}
	return str_append(buffer, ranged(bytes, nbytes));
}

static bool json_buffer_append_number(char[..] *buffer, double number) {
	char stack_buf[64] = {0};

	if (buffer == NULL) {
		return false;
	}
	int written = snprintf(stack_buf, sizeof(stack_buf), "%.17g", number);
	if (written < 0) {
		return false;
	}
	if ((size_t)written < sizeof(stack_buf)) {
		return json_buffer_append_bytes(buffer, stack_buf, written);
	}
	char[..] heap = {0};
	if (!str_reserve(&heap, (size_t)written + 1)) {
		return false;
	}
	written = snprintf(ptr(heap), (size_t)written + 1, "%.17g", number);
	if (written < 0) {
		str_free(&heap);
		return false;
	}
	heap = ranged(ptr(heap), (size_t)written, cap(heap));
	bool ok = str_append(buffer, heap);
	str_free(&heap);
	return ok;
}

static void json_buffer_discard(char[..] *buffer) {
	if (buffer == NULL) {
		return;
	}
	str_free(buffer);
}

/* Copy bytes into an owned slice and reserve a trailing NUL for C APIs. */
static bool json_dup_bytes(char[..] *dst, const char *src, size_t nbytes) {
	if (dst == NULL) {
		return false;
	}
	if (src == NULL && nbytes != 0) {
		return false;
	}
	*dst = str_empty();
	if (nbytes != 0) {
		if (!str_append(dst, ranged(src, nbytes))) {
			str_free(dst);
			return false;
		}
	}
	if (!json_ensure_cstr(dst)) {
		str_free(dst);
		return false;
	}
	return true;
}

static char json_hex_digit(unsigned int value) {
	if (value < 10) {
		return (char)('0' + value);
	}
	return (char)('a' + value - 10);
}

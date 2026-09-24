// Bounded JSON object parser for one Universal Ctags NDJSON record.
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

enum TagJsonResult {
	TagJsonMalformed = -1,
	TagJsonSkip = 0,
	TagJsonEntry = 1
};

struct TagJsonFields {
	char *type;
	char *name;
	char *path;
	char *kind;
	char *signature;
	char *scope;
	char *typeref;
	char *pattern;
	int line;
	bool line_valid;
};

struct TagJsonCursor {
	char *p;
};

void tag_json_fields_free(TagJsonFields *f) {
	if (f == NULL) {
		return;
	}
	free(f.type);
	free(f.name);
	free(f.path);
	free(f.kind);
	free(f.signature);
	free(f.scope);
	free(f.typeref);
	free(f.pattern);
	memset(f, 0, sizeof(*f));
}

static void tag_json_ws(TagJsonCursor *c) {
	while (c != NULL && (*c.p == ' ' || *c.p == '\t' ||
		*c.p == '\r' || *c.p == '\n')) {
		c.p++;
	}
}

static bool tag_json_utf8(char[..] *b, uint32_t cp) {
	if (cp <= 0x7F) {
		return str_append_byte(b, (char)cp);
	}
	if (cp <= 0x7FF) {
		return str_append_byte(b, (char)(0xC0 | (cp >> 6))) &&
			str_append_byte(b, (char)(0x80 | (cp & 0x3F)));
	}
	if (cp <= 0xFFFF) {
		return str_append_byte(b, (char)(0xE0 | (cp >> 12))) &&
			str_append_byte(b, (char)(0x80 | ((cp >> 6) & 0x3F))) &&
			str_append_byte(b, (char)(0x80 | (cp & 0x3F)));
	}
	if (cp <= 0x10FFFF) {
		return str_append_byte(b, (char)(0xF0 | (cp >> 18))) &&
			str_append_byte(b, (char)(0x80 | ((cp >> 12) & 0x3F))) &&
			str_append_byte(b, (char)(0x80 | ((cp >> 6) & 0x3F))) &&
			str_append_byte(b, (char)(0x80 | (cp & 0x3F)));
	}
	return false;
}

static int tag_json_hex(char ch) {
	if (ch >= '0' && ch <= '9') {
		return ch - '0';
	}
	if (ch >= 'a' && ch <= 'f') {
		return (ch - 'a') + 10;
	}
	if (ch >= 'A' && ch <= 'F') {
		return (ch - 'A') + 10;
	}
	return -1;
}

static bool tag_json_u16(TagJsonCursor *c, uint32_t *out) {
	uint32_t v = 0;
	int i = 0;
	if (c == NULL || out == NULL) {
		return false;
	}
	for (i = 0; i < 4; i++) {
		int h = tag_json_hex(c.p[i]);
		if (h < 0) {
			return false;
		}
		v = (v << 4) | (uint32_t)h;
	}
	c.p = c.p + 4;
	*out = v;
	return true;
}

static bool tag_json_string(TagJsonCursor *c, char **out) {
	char[..] b = {0};
	if (c == NULL || out == NULL || *c.p != '"') {
		return false;
	}
	*out = NULL;
	c.p++;
	while (*c.p != 0 && *c.p != '"') {
		char ch = *c.p++;
		if (ch < 0x20) {
			str_free(&b);
			return false;
		}
		if (ch != '\\') {
			if (!str_append_byte(&b, ch)) {
				str_free(&b);
				return false;
			}
			continue;
		}
		ch = *c.p++;
		if (ch == 0) {
			str_free(&b);
			return false;
		}
		if (ch == '"' || ch == '\\' || ch == '/') {
			if (!str_append_byte(&b, ch)) {
				str_free(&b);
				return false;
			}
		} else if (ch == 'b') {
			if (!str_append_byte(&b, '\b')) {
				str_free(&b);
				return false;
			}
		} else if (ch == 'f') {
			if (!str_append_byte(&b, '\f')) {
				str_free(&b);
				return false;
			}
		} else if (ch == 'n') {
			if (!str_append_byte(&b, '\n')) {
				str_free(&b);
				return false;
			}
		} else if (ch == 'r') {
			if (!str_append_byte(&b, '\r')) {
				str_free(&b);
				return false;
			}
		} else if (ch == 't') {
			if (!str_append_byte(&b, '\t')) {
				str_free(&b);
				return false;
			}
		} else if (ch == 'u') {
			uint32_t cp = 0;
			if (!tag_json_u16(c, &cp)) {
				str_free(&b);
				return false;
			}
			if (cp >= 0xD800 && cp <= 0xDBFF) {
				uint32_t low = 0;
				if (c.p[0] != '\\' || c.p[1] != 'u') {
					str_free(&b);
					return false;
				}
				c.p = c.p + 2;
				if (!tag_json_u16(c, &low) || low < 0xDC00 || low > 0xDFFF) {
					str_free(&b);
					return false;
				}
				cp = 0x10000 + ((cp - 0xD800) << 10) + (low - 0xDC00);
			} else if (cp >= 0xDC00 && cp <= 0xDFFF) {
				str_free(&b);
				return false;
			}
			if (!tag_json_utf8(&b, cp)) {
				str_free(&b);
				return false;
			}
		} else {
			str_free(&b);
			return false;
		}
	}
	if (*c.p != '"') {
		str_free(&b);
		return false;
	}
	c.p++;
	if (!str_ensure_z(&b)) {
		str_free(&b);
		return false;
	}
	*out = ptr(b);
	b = str_empty(); /* ownership transferred to *out */
	return true;
}

static bool tag_json_literal(TagJsonCursor *c, const char[..] word) {
	size_t n = 0;
	size_t i = 0;
	if (c == NULL) {
		return false;
	}
	n = len(word);
	for (i = 0; i < n; i++) {
		if (c.p[i] == 0 || c.p[i] != ptr(word)[i]) {
			return false;
		}
	}
	c.p = c.p + n;
	return true;
}

static bool tag_json_number(TagJsonCursor *c, int *integer, bool *is_integer) {
	char *start = NULL;
	int64_t value = 0;
	bool negative = false;
	bool integral = true;
	bool overflow = false;
	if (c == NULL) {
		return false;
	}
	start = c.p;
	if (*c.p == '-') {
		negative = true;
		c.p++;
	}
	if (*c.p == '0') {
		c.p++;
		if (*c.p >= '0' && *c.p <= '9') {
			c.p = start;
			return false;
		}
	} else if (*c.p >= '1' && *c.p <= '9') {
		while (*c.p >= '0' && *c.p <= '9') {
			int digit = *c.p - '0';
			if (value > 214748364) {
				overflow = true;
			} else {
				value = value * 10 + digit;
			}
			c.p++;
		}
	} else {
		c.p = start;
		return false;
	}
	if (*c.p == '.') {
		integral = false;
		c.p++;
		if (*c.p < '0' || *c.p > '9') {
			c.p = start;
			return false;
		}
		while (*c.p >= '0' && *c.p <= '9') {
			c.p++;
		}
	}
	if (*c.p == 'e' || *c.p == 'E') {
		integral = false;
		c.p++;
		if (*c.p == '+' || *c.p == '-') {
			c.p++;
		}
		if (*c.p < '0' || *c.p > '9') {
			c.p = start;
			return false;
		}
		while (*c.p >= '0' && *c.p <= '9') {
			c.p++;
		}
	}
	if (integer != NULL && is_integer != NULL) {
		if (negative || overflow || value < 1 || value > 2147483647) {
			*is_integer = false;
		} else {
			*integer = (int)value;
			*is_integer = integral;
		}
	}
	return true;
}

static bool tag_json_skip_value(TagJsonCursor *c, int depth) {
	if (c == NULL || depth > 32) {
		return false;
	}
	if (*c.p == '"') {
		char *value = NULL;
		bool ok = tag_json_string(c, &value);
		free(value);
		return ok;
	}
	if (*c.p == '{') {
		c.p++;
		tag_json_ws(c);
		if (*c.p == '}') {
			c.p++;
			return true;
		}
		for (;;) {
			char *key = NULL;
			if (!tag_json_string(c, &key)) {
				return false;
			}
			free(key);
			tag_json_ws(c);
			if (*c.p != ':') {
				return false;
			}
			c.p++;
			tag_json_ws(c);
			if (!tag_json_skip_value(c, depth + 1)) {
				return false;
			}
			tag_json_ws(c);
			if (*c.p == '}') {
				c.p++;
				return true;
			}
			if (*c.p != ',') {
				return false;
			}
			c.p++;
			tag_json_ws(c);
		}
	}
	if (*c.p == '[') {
		c.p++;
		tag_json_ws(c);
		if (*c.p == ']') {
			c.p++;
			return true;
		}
		for (;;) {
			if (!tag_json_skip_value(c, depth + 1)) {
				return false;
			}
			tag_json_ws(c);
			if (*c.p == ']') {
				c.p++;
				return true;
			}
			if (*c.p != ',') {
				return false;
			}
			c.p++;
			tag_json_ws(c);
		}
	}
	if (*c.p == '-' || (*c.p >= '0' && *c.p <= '9')) {
		return tag_json_number(c, NULL, NULL);
	}
	if (tag_json_literal(c, "true")) {
		return true;
	}
	if (tag_json_literal(c, "false")) {
		return true;
	}
	return tag_json_literal(c, "null");
}

static char **tag_json_field_slot(TagJsonFields *f, char *key) {
	if (strcmp(key, "_type") == 0) {
		return &f.type;
	}
	if (strcmp(key, "name") == 0) {
		return &f.name;
	}
	if (strcmp(key, "path") == 0) {
		return &f.path;
	}
	if (strcmp(key, "kind") == 0) {
		return &f.kind;
	}
	if (strcmp(key, "signature") == 0) {
		return &f.signature;
	}
	if (strcmp(key, "scope") == 0) {
		return &f.scope;
	}
	if (strcmp(key, "typeref") == 0) {
		return &f.typeref;
	}
	if (strcmp(key, "pattern") == 0) {
		return &f.pattern;
	}
	return NULL;
}

int tag_json_parse_line(char *line, TagJsonFields *out) {
	TagJsonCursor c = {0};
	TagJsonFields f = {0};
	bool semantic_ok = true;
	if (line == NULL || out == NULL) {
		return TagJsonMalformed;
	}
	c.p = line;
	tag_json_ws(&c);
	if (*c.p != '{') {
		return TagJsonMalformed;
	}
	c.p++;
	tag_json_ws(&c);
	if (*c.p != '}') {
		for (;;) {
			char *key = NULL;
			char **slot = NULL;
			if (!tag_json_string(&c, &key)) {
				tag_json_fields_free(&f);
				return TagJsonMalformed;
			}
			tag_json_ws(&c);
			if (*c.p != ':') {
				free(key);
				tag_json_fields_free(&f);
				return TagJsonMalformed;
			}
			c.p++;
			tag_json_ws(&c);
			slot = tag_json_field_slot(&f, key);
			if (strcmp(key, "line") == 0) {
				int value = 0;
				bool valid = false;
				if (!tag_json_number(&c, &value, &valid)) {
					if (!tag_json_skip_value(&c, 0)) {
						free(key);
						tag_json_fields_free(&f);
						return TagJsonMalformed;
					}
					semantic_ok = false;
				} else {
					f.line = value;
					f.line_valid = valid;
				}
			} else if (slot != NULL) {
				if (*c.p == '"') {
					char *value = NULL;
					if (!tag_json_string(&c, &value)) {
						free(key);
						tag_json_fields_free(&f);
						return TagJsonMalformed;
					}
					free(*slot);
					*slot = value;
				} else {
					if (!tag_json_skip_value(&c, 0)) {
						free(key);
						tag_json_fields_free(&f);
						return TagJsonMalformed;
					}
					semantic_ok = false;
				}
			} else if (!tag_json_skip_value(&c, 0)) {
				free(key);
				tag_json_fields_free(&f);
				return TagJsonMalformed;
			}
			free(key);
			tag_json_ws(&c);
			if (*c.p == '}') {
				break;
			}
			if (*c.p != ',') {
				tag_json_fields_free(&f);
				return TagJsonMalformed;
			}
			c.p++;
			tag_json_ws(&c);
		}
	}
	c.p++;
	tag_json_ws(&c);
	if (*c.p != 0) {
		tag_json_fields_free(&f);
		return TagJsonMalformed;
	}
	if (f.type == NULL || strcmp(f.type, "tag") != 0) {
		tag_json_fields_free(&f);
		return TagJsonSkip;
	}
	if (!semantic_ok || f.name == NULL || f.name[0] == 0 ||
		f.path == NULL || f.path[0] == 0 || !f.line_valid) {
		tag_json_fields_free(&f);
		return TagJsonSkip;
	}
	*out = f;
	return TagJsonEntry;
}

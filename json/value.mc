// json/value.mc — constructors, free, accessors, object/array mutation.
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

[[provides]] static JsonValue *json_value_alloc(JsonType type) {
	JsonValue *value = calloc(1, sizeof(JsonValue));
	if (value == NULL) {
		return NULL;
	}
	value.type = type;
	return value;
}

static bool json_string_copy(char[..] *dst, const char *src, size_t nbytes) {
	return json_dup_bytes(dst, src, nbytes);
}

static bool json_string_take_buffer(char[..] *dst, char[..] *buffer) {
	if (dst == NULL || buffer == NULL) {
		return false;
	}
	if (!json_ensure_cstr(buffer)) {
		return false;
	}
	*dst = *buffer;
	*buffer = str_empty();
	return true;
}

static bool json_string_equals(const char[..] string, const char *text, size_t nbytes) {
	if (text == NULL && nbytes != 0) {
		return false;
	}
	return str_eq(string, ranged(text != NULL ? text : "", nbytes));
}

[[provides]] JsonValue *json_new_null() {
	return json_value_alloc(JSON_NULL);
}

[[provides]] JsonValue *json_new_bool(bool boolean) {
	JsonValue *value = json_value_alloc(JSON_BOOL);
	if (value == NULL) {
		return NULL;
	}
	value.boolean = boolean;
	return value;
}

[[provides]] JsonValue *json_new_number(double number) {
	JsonValue *value = json_value_alloc(JSON_NUMBER);
	if (value == NULL) {
		return NULL;
	}
	value.number = number;
	return value;
}

[[provides]] JsonValue *json_new_string_n(const char *string, size_t nbytes) {

	if (string == NULL && nbytes != 0) {
		return NULL;
	}
	if (!json_utf8_validate(string, nbytes)) {
		return NULL;
	}
	JsonValue *value = json_value_alloc(JSON_STRING);
	if (value == NULL) {
		return NULL;
	}
	if (!json_string_copy(&value.string, string, nbytes)) {
		free(value);
		return NULL;
	}
	return value;
}

[[provides]] JsonValue *json_new_string(const char *string) {
	if (string == NULL) {
		return NULL;
	}
	return json_new_string_n(string, strlen(string));
}

[[provides]] JsonValue *json_new_array() {
	return json_value_alloc(JSON_ARRAY);
}

[[provides]] JsonValue *json_new_object() {
	return json_value_alloc(JSON_OBJECT);
}

void json_free([[releases]] JsonValue *value) {
	if (value == NULL) {
		return;
	}
	switch (value.type) {
	case JSON_NULL:
		break;
	case JSON_BOOL:
		break;
	case JSON_NUMBER:
		break;
	case JSON_STRING:
		str_free(&value.string);
		break;
	case JSON_ARRAY:
		for (auto item : value.array.items) {
			json_free(item);
		}
		release(&value.array.items);
		break;
	case JSON_OBJECT:
		for (auto *entry : value.object.entries) {
			str_free(&entry.key);
			json_free(entry.value);
		}
		release(&value.object.entries);
		break;
	}
	free(value);
}

JsonType json_type(const JsonValue *value) {
	if (value == NULL) {
		return JSON_NULL;
	}
	return value.type;
}

bool json_bool_value(const JsonValue *value, bool *out) {
	if (value == NULL || out == NULL || value.type != JSON_BOOL) {
		return false;
	}
	*out = value.boolean;
	return true;
}

bool json_number_value(const JsonValue *value, double *out) {
	if (value == NULL || out == NULL || value.type != JSON_NUMBER) {
		return false;
	}
	*out = value.number;
	return true;
}

/* Whole number in [0, 2^32-1] without float→int cast (QBE/%C limitation). */
bool json_uint32_value(const JsonValue *value, uint32_t *out) {
	double number = 0.0;
	char buf[64] = {0};
	uint32_t acc = 0;
	size_t i = 0;

	if (value == NULL || out == NULL) {
		return false;
	}
	if (!json_number_value(value, &number)) {
		return false;
	}
	if (number < 0.0) {
		return false;
	}
	snprintf(buf, sizeof(buf), "%.17g", number);
	if (buf[0] == 0) {
		return false;
	}
	for (; buf[i] != 0; i++) {
		char c = buf[i];
		uint32_t digit = 0;
		if (c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-') {
			return false;
		}
		if (c < '0' || c > '9') {
			return false;
		}
		digit = c - '0';
		if (acc > 429496729u) {
			return false;
		}
		acc = acc * 10;
		if (digit > 4294967295u - acc) {
			return false;
		}
		acc = acc + digit;
	}
	*out = acc;
	return true;
}

const char *json_string_value(const JsonValue *value) {
	if (value == NULL || value.type != JSON_STRING) {
		return NULL;
	}
	if (str_is_empty(value.string)) {
		return "";
	}
	return ptr(value.string);
}

size_t json_string_length(const JsonValue *value) {
	if (value == NULL || value.type != JSON_STRING) {
		return 0;
	}
	return len(value.string);
}

bool json_array_append(JsonValue *array, [[retains]] JsonValue *item) {
	if (array == NULL || item == NULL || array.type != JSON_ARRAY) {
		json_free(item);
		return false;
	}
	if (!push(&array.array.items, item)) {
		json_free(item);
		return false;
	}
	return true;
}

size_t json_array_size(const JsonValue *array) {
	if (array == NULL || array.type != JSON_ARRAY) {
		return 0;
	}
	return len(array.array.items);
}

const JsonValue *json_array_get(const JsonValue *array, size_t index) {
	if (array == NULL || array.type != JSON_ARRAY || index >= len(array.array.items)) {
		return NULL;
	}
	return array.array.items[index];
}

JsonValue *json_array_get_mutable(JsonValue *array, size_t index) {
	if (array == NULL || array.type != JSON_ARRAY || index >= len(array.array.items)) {
		return NULL;
	}
	return array.array.items[index];
}

static JsonObjectEntry *json_object_find_entry(JsonValue *object, const char *key,
	size_t key_len) {
	if (object == NULL || object.type != JSON_OBJECT) {
		return NULL;
	}
	for (auto *entry : object.object.entries) {
		if (json_string_equals(entry.key, key, key_len)) {
			return entry;
		}
	}
	return NULL;
}

static const JsonObjectEntry *json_object_find_entry_const(const JsonValue *object,
	const char *key, size_t key_len) {
	if (object == NULL || object.type != JSON_OBJECT) {
		return NULL;
	}
	for (auto *entry : object.object.entries) {
		if (json_string_equals(entry.key, key, key_len)) {
			return entry;
		}
	}
	return NULL;
}

static bool json_object_put_owned(JsonValue *object, char[..] key,
	[[retains]] JsonValue *value) {

	if (object == NULL || value == NULL) {
		str_free(&key);
		json_free(value);
		return false;
	}
	JsonObjectEntry *entry = json_object_find_entry(object, ptr(key), len(key));
	if (entry != NULL) {
		str_free(&key);
		json_free(entry.value);
		entry.value = value;
		return true;
	}
	size_t n = len(object.object.entries);
	if (!reserve(&object.object.entries, n + 1)) {
		str_free(&key);
		json_free(value);
		return false;
	}
	object.object.entries = ranged(ptr(object.object.entries), n + 1,
		cap(object.object.entries));
	object.object.entries[n].key = key;
	object.object.entries[n].value = value;
	return true;
}

bool json_object_set_n(JsonValue *object, const char *key, size_t key_len,
	[[retains]] JsonValue *value) {
	char[..] key_copy = {0};

	if (object == NULL || value == NULL || object.type != JSON_OBJECT) {
		json_free(value);
		return false;
	}
	if (key == NULL && key_len != 0) {
		json_free(value);
		return false;
	}
	if (!json_string_copy(&key_copy, key, key_len)) {
		json_free(value);
		return false;
	}
	return json_object_put_owned(object, key_copy, value);
}

bool json_object_set(JsonValue *object, const char *key, [[retains]] JsonValue *value) {
	if (key == NULL) {
		json_free(value);
		return false;
	}
	return json_object_set_n(object, key, strlen(key), value);
}

size_t json_object_size(const JsonValue *object) {
	if (object == NULL || object.type != JSON_OBJECT) {
		return 0;
	}
	return len(object.object.entries);
}

const char *json_object_key_at(const JsonValue *object, size_t index) {
	if (object == NULL || object.type != JSON_OBJECT ||
		index >= len(object.object.entries)) {
		return NULL;
	}
	{
		const char[..] key = object.object.entries[index].key;
		if (str_is_empty(key)) {
			return "";
		}
		return ptr(key);
	}
}

size_t json_object_key_len_at(const JsonValue *object, size_t index) {
	if (object == NULL || object.type != JSON_OBJECT ||
		index >= len(object.object.entries)) {
		return 0;
	}
	return len(object.object.entries[index].key);
}

const JsonValue *json_object_value_at(const JsonValue *object, size_t index) {
	if (object == NULL || object.type != JSON_OBJECT ||
		index >= len(object.object.entries)) {
		return NULL;
	}
	return object.object.entries[index].value;
}

const JsonValue *json_object_get_n(const JsonValue *object, const char *key,
	size_t key_len) {

	const JsonObjectEntry *entry = json_object_find_entry_const(object, key, key_len);
	if (entry == NULL) {
		return NULL;
	}
	return entry.value;
}

const JsonValue *json_object_get(const JsonValue *object, const char *key) {
	if (key == NULL) {
		return NULL;
	}
	return json_object_get_n(object, key, strlen(key));
}

JsonValue *json_object_get_mutable_n(JsonValue *object, const char *key,
	size_t key_len) {

	JsonObjectEntry *entry = json_object_find_entry(object, key, key_len);
	if (entry == NULL) {
		return NULL;
	}
	return entry.value;
}

JsonValue *json_object_get_mutable(JsonValue *object, const char *key) {
	if (key == NULL) {
		return NULL;
	}
	return json_object_get_mutable_n(object, key, strlen(key));
}

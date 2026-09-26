// json/value.mc — constructors, free, accessors, object/array mutation.
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static JsonValue *json_value_alloc(JsonType type) {
	JsonValue *value = calloc(1, sizeof(JsonValue));
	if (value == NULL) {
		return NULL;
	}
	value.type = type;
	return value;
}

static bool json_string_copy(JsonString *dst, const char *src, size_t len) {
	if (dst == NULL) {
		return false;
	}
	dst.data = json_dup_bytes(src, len);
	if (dst.data == NULL) {
		dst.length = 0;
		return false;
	}
	dst.length = len;
	return true;
}

static bool json_string_take_buffer(JsonString *dst, JsonBuffer *buffer) {
	if (dst == NULL || buffer == NULL) {
		return false;
	}
	dst.length = buffer.length;
	dst.data = json_buffer_take(buffer);
	if (dst.data == NULL) {
		dst.length = 0;
		return false;
	}
	return true;
}

static bool json_string_equals(const JsonString *string, const char *text, size_t len) {
	if (string == NULL) {
		return false;
	}
	if (string.length != len) {
		return false;
	}
	if (len == 0) {
		return true;
	}
	return memcmp(string.data, text, len) == 0;
}

JsonValue *json_new_null() {
	return json_value_alloc(JSON_NULL);
}

JsonValue *json_new_bool(bool boolean) {
	JsonValue *value = json_value_alloc(JSON_BOOL);
	if (value == NULL) {
		return NULL;
	}
	value.boolean = boolean;
	return value;
}

JsonValue *json_new_number(double number) {
	JsonValue *value = json_value_alloc(JSON_NUMBER);
	if (value == NULL) {
		return NULL;
	}
	value.number = number;
	return value;
}

JsonValue *json_new_string_n(const char *string, size_t len) {
	JsonValue *value = NULL;

	if (string == NULL && len != 0) {
		return NULL;
	}
	if (!json_utf8_validate(string, len)) {
		return NULL;
	}
	value = json_value_alloc(JSON_STRING);
	if (value == NULL) {
		return NULL;
	}
	if (!json_string_copy(&value.string, string, len)) {
		free(value);
		return NULL;
	}
	return value;
}

JsonValue *json_new_string(const char *string) {
	if (string == NULL) {
		return NULL;
	}
	return json_new_string_n(string, strlen(string));
}

JsonValue *json_new_array() {
	return json_value_alloc(JSON_ARRAY);
}

JsonValue *json_new_object() {
	return json_value_alloc(JSON_OBJECT);
}

void json_free(JsonValue *value) {
	size_t i = 0;

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
		free(value.string.data);
		break;
	case JSON_ARRAY:
		for (i = 0; i < value.array.count; i++) {
			json_free(value.array.items[i]);
		}
		free(value.array.items);
		break;
	case JSON_OBJECT:
		for (i = 0; i < value.object.count; i++) {
			free(value.object.entries[i].key.data);
			json_free(value.object.entries[i].value);
		}
		free(value.object.entries);
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
	for (i = 0; buf[i] != 0; i++) {
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
	return value.string.data;
}

size_t json_string_length(const JsonValue *value) {
	if (value == NULL || value.type != JSON_STRING) {
		return 0;
	}
	return value.string.length;
}

bool json_array_append(JsonValue *array, JsonValue *item) {
	if (array == NULL || item == NULL || array.type != JSON_ARRAY) {
		return false;
	}
	if (!json_grow_ptr_array((void **)&array.array.items, &array.array.capacity,
		sizeof(JsonValue *), array.array.count + 1)) {
		return false;
	}
	array.array.items[array.array.count] = item;
	array.array.count++;
	return true;
}

size_t json_array_size(const JsonValue *array) {
	if (array == NULL || array.type != JSON_ARRAY) {
		return 0;
	}
	return array.array.count;
}

const JsonValue *json_array_get(const JsonValue *array, size_t index) {
	if (array == NULL || array.type != JSON_ARRAY || index >= array.array.count) {
		return NULL;
	}
	return array.array.items[index];
}

JsonValue *json_array_get_mutable(JsonValue *array, size_t index) {
	if (array == NULL || array.type != JSON_ARRAY || index >= array.array.count) {
		return NULL;
	}
	return array.array.items[index];
}

static JsonObjectEntry *json_object_find_entry(JsonValue *object, const char *key,
	size_t key_len) {
	size_t i = 0;

	if (object == NULL || object.type != JSON_OBJECT) {
		return NULL;
	}
	for (i = 0; i < object.object.count; i++) {
		if (json_string_equals(&object.object.entries[i].key, key, key_len)) {
			return &object.object.entries[i];
		}
	}
	return NULL;
}

static const JsonObjectEntry *json_object_find_entry_const(const JsonValue *object,
	const char *key, size_t key_len) {
	size_t i = 0;

	if (object == NULL || object.type != JSON_OBJECT) {
		return NULL;
	}
	for (i = 0; i < object.object.count; i++) {
		if (json_string_equals(&object.object.entries[i].key, key, key_len)) {
			return &object.object.entries[i];
		}
	}
	return NULL;
}

static bool json_object_put_owned(JsonValue *object, JsonString key, JsonValue *value) {
	JsonObjectEntry *entry = NULL;

	if (object == NULL || value == NULL) {
		return false;
	}
	entry = json_object_find_entry(object, key.data, key.length);
	if (entry != NULL) {
		free(key.data);
		json_free(entry.value);
		entry.value = value;
		return true;
	}
	if (!json_grow_ptr_array((void **)&object.object.entries, &object.object.capacity,
		sizeof(JsonObjectEntry), object.object.count + 1)) {
		return false;
	}
	entry = &object.object.entries[object.object.count];
	object.object.count++;
	entry.key = key;
	entry.value = value;
	return true;
}

bool json_object_set_n(JsonValue *object, const char *key, size_t key_len,
	JsonValue *value) {
	JsonString key_copy = {0};

	if (object == NULL || value == NULL || object.type != JSON_OBJECT) {
		return false;
	}
	if (key == NULL && key_len != 0) {
		return false;
	}
	if (!json_string_copy(&key_copy, key, key_len)) {
		return false;
	}
	if (!json_object_put_owned(object, key_copy, value)) {
		free(key_copy.data);
		return false;
	}
	return true;
}

bool json_object_set(JsonValue *object, const char *key, JsonValue *value) {
	if (key == NULL) {
		return false;
	}
	return json_object_set_n(object, key, strlen(key), value);
}

size_t json_object_size(const JsonValue *object) {
	if (object == NULL || object.type != JSON_OBJECT) {
		return 0;
	}
	return object.object.count;
}

const char *json_object_key_at(const JsonValue *object, size_t index) {
	if (object == NULL || object.type != JSON_OBJECT ||
		index >= object.object.count) {
		return NULL;
	}
	return object.object.entries[index].key.data;
}

size_t json_object_key_len_at(const JsonValue *object, size_t index) {
	if (object == NULL || object.type != JSON_OBJECT ||
		index >= object.object.count) {
		return 0;
	}
	return object.object.entries[index].key.length;
}

const JsonValue *json_object_value_at(const JsonValue *object, size_t index) {
	if (object == NULL || object.type != JSON_OBJECT ||
		index >= object.object.count) {
		return NULL;
	}
	return object.object.entries[index].value;
}

const JsonValue *json_object_get_n(const JsonValue *object, const char *key,
	size_t key_len) {
	const JsonObjectEntry *entry = NULL;

	entry = json_object_find_entry_const(object, key, key_len);
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
	JsonObjectEntry *entry = NULL;

	entry = json_object_find_entry(object, key, key_len);
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

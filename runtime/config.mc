// runtime/config.mc — ~/.jem.json variables + keybindings (jem.go/runtime/config.go).
// JSON DOM via the top-level json package (jem.c util_json).
import "json";

#include <ctype.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>

enum {
	ConfigPathMax = 512,
	ConfigFileMax = 64 * 1024,
	ConfigKeyMax = 128,
	ConfigStrMax = 256
};

// config_parse_key_sequence parses Emacs-style chords (jem.go parseKeySequence).
(uint32_t, bool) config_parse_key_sequence(const char *s) {
	char buf[ConfigKeyMax] = {0};
	char *space = NULL;
	char *tok = NULL;
	uint32_t code = 0;
	size_t n = 0;

	if (s == NULL) {
		return (0, false);
	}
	while (*s == ' ' || *s == '\t') {
		s++;
	}
	if (*s == 0) {
		return (0, false);
	}
	snprintf(buf, sizeof(buf), "%s", s);
	n = strlen(buf);
	while (n > 0 && (buf[n - 1] == ' ' || buf[n - 1] == '\t')) {
		buf[--n] = 0;
	}
	space = strchr(buf, ' ');
	if (space != NULL) {
		*space = 0;
		{
			auto (pre, pok) = config_parse_key_sequence(buf);
			auto (suf, sok) = config_parse_key_sequence(space + 1);
			if (!pok || !sok) {
				return (0, false);
			}
			if (pre != (term_key_value(TermCTL) | (uint32_t)'X')) {
				return (0, false);
			}
			return (term_key_value(TermCTLX) | suf, true);
		}
	}

	tok = buf;
	while (tok[0] != 0) {
		if ((tok[0] == 'M' || tok[0] == 'm') && tok[1] == '-') {
			if ((code & term_key_value(TermMETA)) != 0) {
				return (0, false);
			}
			code |= term_key_value(TermMETA);
			tok += 2;
			continue;
		}
		if ((tok[0] == 'C' || tok[0] == 'c') && tok[1] == '-') {
			if ((code & term_key_value(TermCTL)) != 0) {
				return (0, false);
			}
			code |= term_key_value(TermCTL);
			tok += 2;
			continue;
		}
		if ((tok[0] == 'S' || tok[0] == 's') && tok[1] == '-') {
			if ((code & term_key_value(TermSHIFT)) != 0) {
				return (0, false);
			}
			code |= term_key_value(TermSHIFT);
			tok += 2;
			continue;
		}
		break;
	}

	if (tok[0] == 0) {
		return (0, false);
	}
	if (strcasecmp(tok, "TAB") == 0) {
		return (code | term_key_value(TermKeyTab), true);
	}
	if (strcasecmp(tok, "ENTER") == 0 || strcasecmp(tok, "RET") == 0) {
		return (code | term_key_value(TermKeyEnter), true);
	}
	if (strcasecmp(tok, "SPACE") == 0 || strcasecmp(tok, "SPC") == 0) {
		return (code | (uint32_t)' ', true);
	}
	if (strcasecmp(tok, "BACKSPACE") == 0 || strcasecmp(tok, "BS") == 0) {
		return (code | (uint32_t)0x7F, true);
	}
	if (strcasecmp(tok, "UP") == 0) {
		return (code | term_key_value(TermKeyUp), true);
	}
	if (strcasecmp(tok, "DOWN") == 0) {
		return (code | term_key_value(TermKeyDown), true);
	}
	if (strcasecmp(tok, "LEFT") == 0) {
		return (code | term_key_value(TermKeyLeft), true);
	}
	if (strcasecmp(tok, "RIGHT") == 0) {
		return (code | term_key_value(TermKeyRight), true);
	}
	if (strcasecmp(tok, "HOME") == 0) {
		return (code | term_key_value(TermKeyHome), true);
	}
	if (strcasecmp(tok, "END") == 0) {
		return (code | term_key_value(TermKeyEnd), true);
	}
	if (strcasecmp(tok, "PAGEUP") == 0 || strcasecmp(tok, "PGUP") == 0) {
		return (code | term_key_value(TermKeyPageUp), true);
	}
	if (strcasecmp(tok, "PAGEDOWN") == 0 || strcasecmp(tok, "PGDOWN") == 0 ||
		strcasecmp(tok, "PGDN") == 0) {
		return (code | term_key_value(TermKeyPageDown), true);
	}
	if (strcasecmp(tok, "DELETE") == 0 || strcasecmp(tok, "DEL") == 0) {
		return (code | term_key_value(TermKeyDelete), true);
	}
	if (tok[0] != 0 && tok[1] == 0) {
		char b = tok[0];
		if ((code & (term_key_value(TermCTL) | term_key_value(TermMETA))) != 0 &&
			b >= 'a' && b <= 'z') {
			b = (char)(b - ('a' - 'A'));
		}
		return (code | (uint32_t)b, true);
	}
	return (0, false);
}

static void config_apply_binding(const char *chord, const char *cmd_or_null,
	bool is_null) {
	uint32_t code = 0;
	CommandFn fn = NULL;
	{
		auto (k, ok) = config_parse_key_sequence(chord);
		if (!ok) {
			display_set_echo("invalid JSON keybinding code");
			return;
		}
		code = k;
	}
	if (is_null) {
		runtime_rebind(code, NULL);
		return;
	}
	fn = runtime_find_cmd(cmd_or_null);
	if (fn == NULL) {
		display_set_echo("invalid JSON command name");
		return;
	}
	runtime_rebind(code, fn);
}

static void config_apply_var_text(const char *key, const char *text) {
	if (!vars_known(key)) {
		return;
	}
	if (!vars_set_named_text(key, text)) {
		display_set_echo("ignoring invalid config value");
	}
}

static void config_apply_var_value(const char *key, const JsonValue *value) {
	const char *string_value = NULL;
	char numbuf[64] = {0};
	uint32_t whole = 0;

	if (key == NULL || value == NULL) {
		return;
	}
	if (json_uint32_value(value, &whole)) {
		snprintf(numbuf, sizeof(numbuf), "%u", whole);
		config_apply_var_text(key, numbuf);
		return;
	}
	if (json_type(value) == JSON_NUMBER) {
		display_set_echo("ignoring invalid config value");
		return;
	}
	string_value = json_string_value(value);
	if (string_value != NULL) {
		config_apply_var_text(key, (const char *)string_value);
	}
}

static void config_apply_keybindings_obj(const JsonValue *keybindings) {
	size_t i = 0;
	size_t n = 0;

	if (keybindings == NULL) {
		return;
	}
	if (json_type(keybindings) != JSON_OBJECT) {
		display_set_echo("invalid JSON keybindings object");
		return;
	}
	n = json_object_size(keybindings);
	for (i = 0; i < n; i++) {
		const char *obj_key = NULL;
		size_t key_len = 0;
		const JsonValue *value = NULL;
		char chord[ConfigKeyMax] = {0};

		obj_key = json_object_key_at(keybindings, i);
		if (obj_key == NULL) {
			continue;
		}
		key_len = json_object_key_len_at(keybindings, i);
		if (key_len + 1 > sizeof(chord)) {
			display_set_echo("invalid JSON keybinding code");
			continue;
		}
		memcpy(chord, obj_key, key_len);
		chord[key_len] = 0;
		value = json_object_value_at(keybindings, i);
		if (value == NULL) {
			continue;
		}
		if (json_type(value) == JSON_NULL) {
			config_apply_binding(chord, NULL, true);
			continue;
		}
		if (json_type(value) != JSON_STRING) {
			display_set_echo("invalid JSON keybinding value");
			continue;
		}
		{
			const char *cmd = json_string_value(value);
			if (cmd == NULL) {
				display_set_echo("invalid JSON command name");
				continue;
			}
			config_apply_binding(chord, (const char *)cmd, false);
		}
	}
}

// config_apply_json applies a jem.json document (variables + keybindings).
bool config_apply_json(char *text, size_t n) {
	JsonError err = {0};
	JsonValue *root = NULL;
	size_t i = 0;
	size_t count = 0;

	if (text == NULL) {
		return false;
	}
	root = json_parse_n((const char *)text, n, &err);
	if (root == NULL) {
		return false;
	}
	if (json_type(root) != JSON_OBJECT) {
		json_free(root);
		return false;
	}

	count = json_object_size(root);
	for (i = 0; i < count; i++) {
		const char *obj_key = NULL;
		size_t key_len = 0;
		const JsonValue *value = NULL;
		char name[ConfigKeyMax] = {0};

		obj_key = json_object_key_at(root, i);
		if (obj_key == NULL) {
			continue;
		}
		key_len = json_object_key_len_at(root, i);
		if (key_len + 1 > sizeof(name)) {
			continue;
		}
		memcpy(name, obj_key, key_len);
		name[key_len] = 0;
		value = json_object_value_at(root, i);
		if (value == NULL) {
			continue;
		}
		if (strcmp(name, "keybindings") == 0) {
			config_apply_keybindings_obj(value);
			continue;
		}
		if (strcmp(name, "theme") == 0 && json_type(value) == JSON_STRING) {
			const char *theme = json_string_value(value);
			if (theme != NULL && strcmp(theme, "light") == 0) {
				(void)vars_set_named("theme-mode", 1);
			} else if (theme != NULL && strcmp(theme, "dark") == 0) {
				(void)vars_set_named("theme-mode", 0);
			}
			continue;
		}
		config_apply_var_value(name, value);
	}

	json_free(root);
	return true;
}

// config_load reads ~/.jem.json when present (jem.go ConfigLoad).
void config_load() {
	char path[ConfigPathMax] = {0};
	char *data = NULL;
	size_t n = 0;
	size_t cap = 0;
	FILE *fp = NULL;
	int wn = 0;
#ifdef _WIN32
	char home_buf[ConfigPathMax] = {0};
	{
		auto (have, home) = os_env("USERPROFILE");
		if (have && len(home) > 0) {
			snprintf(home_buf, sizeof(home_buf), "%.*s",
				(int)len(home), ptr(home));
		} else {
			auto (hd, drive) = os_env("HOMEDRIVE");
			auto (hp, pathv) = os_env("HOMEPATH");
			if (hd && hp && len(drive) > 0 && len(pathv) > 0) {
				snprintf(home_buf, sizeof(home_buf), "%.*s%.*s",
					(int)len(drive), ptr(drive),
					(int)len(pathv), ptr(pathv));
			}
		}
		if (home_buf[0] == 0) {
			return;
		}
		wn = snprintf(path, sizeof(path), "%s\\.jem.json", home_buf);
	}
#else
	{
		auto (have, home) = os_env("HOME");
		if (!have || len(home) == 0) {
			return;
		}
		wn = snprintf(path, sizeof(path), "%.*s/.jem.json",
			(int)len(home), ptr(home));
	}
#endif
	if (wn < 0 || (size_t)wn >= sizeof(path)) {
		return;
	}
	fp = fopen(path, "r");
	if (fp == NULL) {
		return;
	}
	for (;;) {
		char chunk[4096] = {0};
		size_t got = fread(chunk, 1, sizeof(chunk), fp);
		if (got == 0) {
			break;
		}
		if (n + got + 1 > ConfigFileMax) {
			fclose(fp);
			free(data);
			fprintf(stderr, "jem: %s too large\n", path);
			return;
		}
		if (n + got + 1 > cap) {
			size_t ncap = cap == 0 ? 8192 : cap * 2;
			char *p = NULL;
			while (ncap < n + got + 1) {
				ncap *= 2;
			}
			p = realloc(data, ncap);
			if (p == NULL) {
				fclose(fp);
				free(data);
				return;
			}
			data = p;
			cap = ncap;
		}
		memcpy(data + n, chunk, got);
		n += got;
	}
	fclose(fp);
	if (data == NULL) {
		return;
	}
	data[n] = 0;
	if (!config_apply_json(data, n)) {
		fprintf(stderr, "jem: invalid %s\n", path);
	}
	free(data);
}

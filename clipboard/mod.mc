// clipboard — system text clipboard with OSC 52 write fallback.
//
// This package is editor-independent. It depends only on modc std packages.
import "fs";
import "os";
import "str";

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

enum ClipboardProvider {
	ClipboardNone = 0,
	ClipboardMacOS = 1,
	ClipboardWayland = 2,
	ClipboardXclip = 3,
	ClipboardXsel = 4
};

enum ClipboardLimits {
	ClipboardPathMax = 4096,
	ClipboardReadMax = 1024 * 1024
};

static ClipboardProvider g_provider = ClipboardNone;
static char g_write_path[ClipboardPathMax] = {0};
static char g_read_path[ClipboardPathMax] = {0};
static bool g_initialized;

static bool clipboard_find(char *dst, size_t cap, const char[..] name) {
	auto (found, ok) = os_look_path(ranged(dst, 0, cap), name);
	(void)found;
	return ok;
}

static bool clipboard_find_pair(ClipboardProvider provider,
	const char[..] write_name, const char[..] read_name) {
	char write_path[ClipboardPathMax] = {0};
	char read_path[ClipboardPathMax] = {0};
	if (!clipboard_find(write_path, sizeof(write_path), write_name) ||
		!clipboard_find(read_path, sizeof(read_path), read_name)) {
		return false;
	}
	cstr_write(g_write_path, str_from_cstr(write_path));
	cstr_write(g_read_path, str_from_cstr(read_path));
	g_provider = provider;
	return true;
}

// clipboard_init selects the first native provider available on PATH.
bool clipboard_init() {
	if (g_initialized) {
		return g_provider != ClipboardNone;
	}
	g_initialized = true;
	if (clipboard_find_pair(ClipboardMacOS, "pbcopy", "pbpaste") ||
		clipboard_find_pair(ClipboardWayland, "wl-copy", "wl-paste") ||
		clipboard_find_pair(ClipboardXclip, "xclip", "xclip") ||
		clipboard_find_pair(ClipboardXsel, "xsel", "xsel")) {
		return true;
	}
	return false;
}

bool clipboard_native_ready() {
	(void)clipboard_init();
	return g_provider != ClipboardNone;
}

const char[..] clipboard_provider_name() {
	(void)clipboard_init();
	switch (g_provider) {
	case ClipboardMacOS:
		return "macOS";
	case ClipboardWayland:
		return "Wayland";
	case ClipboardXclip:
		return "xclip";
	case ClipboardXsel:
		return "xsel";
	default:
		return "none";
	}
}

static size_t clipboard_args(bool writing, const char[..] args[4]) {
	if (g_provider == ClipboardXclip) {
		args[0] = "-selection";
		args[1] = "clipboard";
		if (!writing) {
			args[2] = "-o";
			return 3;
		}
		return 2;
	}
	if (g_provider == ClipboardXsel) {
		args[0] = "--clipboard";
		if (writing) {
			args[1] = "--input";
		} else {
			args[1] = "--output";
		}
		return 2;
	}
	return 0;
}

static bool clipboard_write_all(File *file, const char[..] data) {
	size_t offset = 0;
	while (offset < len(data)) {
		auto (n, ok) = file.write(ranged(ptr(data) + offset,
			len(data) - offset, len(data) - offset));
		if (!ok || n <= 0) {
			return false;
		}
		offset += n;
	}
	return true;
}

static bool clipboard_write_native(const char[..] text) {
	Cmd cmd = {0};
	const char[..] args[4] = {0};
	size_t nargs = 0;
	bool wrote = false;
	if (!clipboard_native_ready()) {
		return false;
	}
	nargs = clipboard_args(true, args);
	cmd.init(str_from_cstr(g_write_path), args, nargs);
	cmd.stdio(StdOut, IoNull);
	cmd.stdio(StdErr, IoNull);
	auto (input, ok) = cmd.pipe(StdIn);
	if (!ok) {
		cmd.close();
		return false;
	}
	if (!cmd.start()) {
		input.close();
		cmd.close();
		return false;
	}
	wrote = clipboard_write_all(&input, text);
	input.close();
	auto (exit_code, waited) = cmd.wait();
	cmd.close();
	return wrote && waited && exit_code == 0;
}

static (char *, size_t, bool) clipboard_read_native() {
	Cmd cmd = {0};
	const char[..] args[4] = {0};
	char chunk[4096] = {0};
	char *data = NULL;
	size_t used = 0;
	size_t capacity = 0;
	size_t nargs = 0;
	if (!clipboard_native_ready()) {
		return (NULL, 0, false);
	}
	nargs = clipboard_args(false, args);
	cmd.init(str_from_cstr(g_read_path), args, nargs);
	cmd.stdio(StdIn, IoNull);
	cmd.stdio(StdErr, IoNull);
	auto (output, ok) = cmd.pipe(StdOut);
	if (!ok) {
		cmd.close();
		return (NULL, 0, false);
	}
	if (!cmd.start()) {
		output.close();
		cmd.close();
		return (NULL, 0, false);
	}
	for (;;) {
		auto (n, rok) = output.read(ranged(chunk, 0, sizeof(chunk)));
		size_t keep = 0;
		if (!rok) {
			free(data);
			output.close();
			cmd.close();
			return (NULL, 0, false);
		}
		if (n == 0) {
			break;
		}
		if (used < (size_t)ClipboardReadMax) {
			keep = n;
			if (keep > (size_t)ClipboardReadMax - used) {
				keep = (size_t)ClipboardReadMax - used;
			}
			if (used + keep + 1 > capacity) {
				size_t next = capacity == 0 ? 4096 : capacity * 2;
				char *grown = NULL;
				while (next < used + keep + 1) {
					next *= 2;
				}
				if (next > (size_t)ClipboardReadMax + 1) {
					next = (size_t)ClipboardReadMax + 1;
				}
				grown = realloc(data, next);
				if (grown == NULL) {
					free(data);
					output.close();
					cmd.close();
					return (NULL, 0, false);
				}
				data = grown;
				capacity = next;
			}
			memcpy(data + used, chunk, keep);
			used += keep;
		}
	}
	output.close();
	auto (exit_code, waited) = cmd.wait();
	cmd.close();
	if (!waited || exit_code != 0 || used == 0) {
		free(data);
		return (NULL, 0, false);
	}
	data[used] = 0;
	return (data, used, true);
}

static bool clipboard_write_osc52(const char[..] text) {
	static char alphabet[] =
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
	uint8_t *bytes = (uint8_t *)ptr(text);
	File output = {0};
	char *encoded = NULL;
	size_t groups = 0;
	size_t encoded_len = 0;
	size_t total = 0;
	size_t i = 0;
	size_t out = 0;
	bool ok = false;
#ifdef _WIN32
	()text;
	return false;
#else
	if (len(text) == 0) {
		return true;
	}
	groups = (len(text) + 2) / 3;
	encoded_len = groups * 4;
	total = 7 + encoded_len + 1;
	encoded = malloc(total);
	if (encoded == NULL) {
		return false;
	}
	memcpy(encoded, "\x1b]52;c;", 7);
	out = 7;
	for (i = 0; i < len(text); i += 3) {
		uint32_t a = bytes[i];
		uint32_t b = 0;
		uint32_t c = 0;
		if (i + 1 < len(text)) {
			b = bytes[i + 1];
		}
		if (i + 2 < len(text)) {
			c = bytes[i + 2];
		}
		encoded[out++] = alphabet[a >> 2];
		encoded[out++] = alphabet[((a & 3) << 4) | (b >> 4)];
		if (i + 1 < len(text)) {
			encoded[out++] = alphabet[((b & 15) << 2) | (c >> 6)];
		} else {
			encoded[out++] = '=';
		}
		if (i + 2 < len(text)) {
			encoded[out++] = alphabet[c & 63];
		} else {
			encoded[out++] = '=';
		}
	}
	encoded[out++] = '\a';
	{
		auto (file, wrapped) = file_from_fd_borrow(1);
		if (!wrapped) {
			free(encoded);
			return false;
		}
		output = file;
	}
	ok = clipboard_write_all(&output, ranged(encoded, out, out));
	output.close();
	free(encoded);
	return ok;
#endif
}

// clipboard_write prefers a native provider and falls back to OSC 52.
bool clipboard_write(const char[..] text) {
	if (len(text) == 0) {
		return true;
	}
	if (clipboard_write_native(text)) {
		return true;
	}
	return clipboard_write_osc52(text);
}

// clipboard_read returns newly allocated text owned by the caller.
// OSC 52 is deliberately write-only because clipboard queries are not
// consistently supported and require asynchronous terminal response parsing.
(char *, size_t, bool) clipboard_read() {
	return clipboard_read_native();
}

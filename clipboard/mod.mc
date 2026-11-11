// clipboard — system text clipboard with OSC 52 write fallback.
//
// This package is editor-independent. It depends only on modc std packages.
import "fs";
import "os";
import "str";
import "tty";
import "term";

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
	ClipboardXsel = 4,
	ClipboardWin32 = 5
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
	cstr_write(g_write_path, ranged(write_path, strlen(write_path)));
	cstr_write(g_read_path, ranged(read_path, strlen(read_path)));
	g_provider = provider;
	return true;
}

// clipboard_init selects the first native provider available.
bool clipboard_init() {
	if (g_initialized) {
		return g_provider != ClipboardNone;
	}
	g_initialized = true;
#ifdef _WIN32
	g_provider = ClipboardWin32;
	return true;
#else
	if (clipboard_find_pair(ClipboardMacOS, "pbcopy", "pbpaste") ||
		clipboard_find_pair(ClipboardWayland, "wl-copy", "wl-paste") ||
		clipboard_find_pair(ClipboardXclip, "xclip", "xclip") ||
		clipboard_find_pair(ClipboardXsel, "xsel", "xsel")) {
		return true;
	}
	return false;
#endif
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
	case ClipboardWin32:
		return "win32";
	default:
		return "none";
	}
}

static size_t clipboard_args(bool writing, const char[..][..] args) {
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
	if (!clipboard_native_ready()) {
		return false;
	}
#ifdef _WIN32
	if (g_provider == ClipboardWin32) {
		return clipboard_win32_write(ptr(text), len(text));
	}
#endif
	size_t nargs = clipboard_args(true, args);
	cmd.init(ranged(g_write_path, strlen(g_write_path)), args, nargs);
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
	bool wrote = clipboard_write_all(&input, text);
	input.close();
	auto (exit_code, waited) = cmd.wait();
	cmd.close();
	return wrote && waited && exit_code == 0;
}

static (char *, size_t, bool) clipboard_read_native() {
	Cmd cmd = {0};
	const char[..] args[4] = {0};
	char chunk[4096] = {0};
	char[..] data = {0};
	if (!clipboard_native_ready()) {
		return (NULL, 0, false);
	}
#ifdef _WIN32
	if (g_provider == ClipboardWin32) {
		return clipboard_win32_read();
	}
#endif
	size_t nargs = clipboard_args(false, args);
	cmd.init(ranged(g_read_path, strlen(g_read_path)), args, nargs);
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
			str_free(&data);
			output.close();
			cmd.close();
			return (NULL, 0, false);
		}
		if (n == 0) {
			break;
		}
		if (len(data) < (size_t)ClipboardReadMax) {
			keep = n;
			if (keep > (size_t)ClipboardReadMax - len(data)) {
				keep = (size_t)ClipboardReadMax - len(data);
			}
			if (keep > 0 && !str_append(&data, ranged(chunk, keep))) {
				str_free(&data);
				output.close();
				cmd.close();
				return (NULL, 0, false);
			}
		}
	}
	output.close();
	auto (exit_code, waited) = cmd.wait();
	cmd.close();
	if (!waited || exit_code != 0 || len(data) == 0) {
		str_free(&data);
		return (NULL, 0, false);
	}
	if (!str_reserve(&data, len(data) + 1)) {
		str_free(&data);
		return (NULL, 0, false);
	}
	{
		char[..] z = ranged(ptr(data), len(data) + 1, cap(data));
		z[len(data)] = 0;
	}
	return (ptr(data), len(data), true);
}

static bool clipboard_write_osc52(const char[..] text) {
	static const char[..] alphabet =
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
	size_t i = 0;
	if (len(text) == 0) {
		return true;
	}
	if (!term_is_tty()) {
		return false;
	}
	size_t groups = (len(text) + 2) / 3;
	size_t encoded_len = groups * 4;
	size_t total = 7 + encoded_len + 1;
	pool p {
		char[..] encoded = p.alloc(char, total);
		if (ptr(encoded) == NULL) {
			return false;
		}
		memcpy(ptr(encoded), "\x1b]52;c;", 7);
		size_t out = 7;
		for (; i < len(text); i += 3) {
			uint32_t a = text[i];
			uint32_t b = 0;
			uint32_t c = 0;
			if (i + 1 < len(text)) {
				b = text[i + 1];
			}
			if (i + 2 < len(text)) {
				c = text[i + 2];
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
		encoded[out++] = 7; /* BEL terminator; modc char literals don't support \a or \xNN. */
		/* Route through term's single output buffer (matches jem.c, which emits
		 * OSC 52 via the same buffered stdout FILE* used for all other terminal
		 * output). A second, independent write path here (e.g. opening our own
		 * tty_stdout() fd and writing directly) can interleave with whatever
		 * escape sequence term/mod.mc currently has buffered but not yet
		 * flushed, tearing it apart -- the "13;15H"/"29;60H" stray-text bug. */
		term_write(ptr(encoded), out);
		term_out_flush();
		return true;
	}
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

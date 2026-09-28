// clipboard/win32.mc — native CF_UNICODETEXT clipboard (jem.c clipboard_win32.c).
#ifdef _WIN32
#include <windows.h>
#include <stddef.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>

#pragma modc c_libs(user32)

static char *clipboard_expand_newlines(const char *text, size_t len, size_t *out_len) {
	size_t extra = 0;
	size_t i = 0;
	size_t j = 0;
	char *expanded = NULL;
	for (i = 0; i < len; i++) {
		if (text[i] == '\n') {
			extra++;
		}
	}
	expanded = calloc(len + extra + 1, 1);
	if (expanded == NULL) {
		return NULL;
	}
	for (i = 0; i < len; i++) {
		if (text[i] == '\n') {
			expanded[j++] = '\r';
		}
		expanded[j++] = text[i];
	}
	expanded[j] = 0;
	*out_len = j;
	return expanded;
}

static char *clipboard_normalize_newlines(const char *text, size_t len, size_t *out_len) {
	char *normalized = NULL;
	size_t i = 0;
	size_t j = 0;
	normalized = calloc(len + 1, 1);
	if (normalized == NULL) {
		return NULL;
	}
	for (i = 0; i < len; i++) {
		if (text[i] == '\r') {
			normalized[j++] = '\n';
			if (i + 1 < len && text[i + 1] == '\n') {
				i++;
			}
		} else {
			normalized[j++] = text[i];
		}
	}
	normalized[j] = 0;
	*out_len = j;
	return normalized;
}

static bool clipboard_win32_write(const char *text, size_t len) {
	size_t expanded_len = 0;
	char *expanded = NULL;
	HANDLE handle = NULL;
	WCHAR *wide = NULL;
	int wide_len = 0;

	expanded = clipboard_expand_newlines(text != NULL ? text : "", len, &expanded_len);
	if (expanded == NULL) {
		return false;
	}
	wide_len = MultiByteToWideChar(CP_UTF8, 0, expanded, -1, NULL, 0);
	if (wide_len <= 0) {
		free(expanded);
		return false;
	}
	handle = GlobalAlloc(GMEM_MOVEABLE, wide_len * sizeof(WCHAR));
	if (handle == NULL) {
		free(expanded);
		return false;
	}
	wide = (WCHAR *)GlobalLock(handle);
	if (wide == NULL) {
		GlobalFree(handle);
		free(expanded);
		return false;
	}
	if (MultiByteToWideChar(CP_UTF8, 0, expanded, -1, wide, wide_len) <= 0) {
		GlobalUnlock(handle);
		GlobalFree(handle);
		free(expanded);
		return false;
	}
	GlobalUnlock(handle);
	free(expanded);
	if (!OpenClipboard(NULL)) {
		GlobalFree(handle);
		return false;
	}
	if (!EmptyClipboard() || SetClipboardData(CF_UNICODETEXT, handle) == NULL) {
		CloseClipboard();
		GlobalFree(handle);
		return false;
	}
	CloseClipboard();
	return true;
}

static (char *, size_t, bool) clipboard_win32_read() {
	HANDLE handle = NULL;
	WCHAR *wide = NULL;
	int utf8_len = 0;
	char *utf8 = NULL;
	char *normalized = NULL;
	size_t normalized_len = 0;

	if (!OpenClipboard(NULL)) {
		return (NULL, 0, false);
	}
	if (!IsClipboardFormatAvailable(CF_UNICODETEXT)) {
		CloseClipboard();
		return (NULL, 0, true);
	}
	handle = GetClipboardData(CF_UNICODETEXT);
	if (handle == NULL) {
		CloseClipboard();
		return (NULL, 0, false);
	}
	wide = (WCHAR *)GlobalLock(handle);
	if (wide == NULL) {
		CloseClipboard();
		return (NULL, 0, false);
	}
	utf8_len = WideCharToMultiByte(CP_UTF8, 0, wide, -1, NULL, 0, NULL, NULL);
	if (utf8_len <= 0) {
		GlobalUnlock(handle);
		CloseClipboard();
		return (NULL, 0, false);
	}
	utf8 = calloc(utf8_len, 1);
	if (utf8 == NULL) {
		GlobalUnlock(handle);
		CloseClipboard();
		return (NULL, 0, false);
	}
	if (WideCharToMultiByte(CP_UTF8, 0, wide, -1, utf8, utf8_len, NULL, NULL) <= 0) {
		free(utf8);
		GlobalUnlock(handle);
		CloseClipboard();
		return (NULL, 0, false);
	}
	GlobalUnlock(handle);
	CloseClipboard();
	normalized = clipboard_normalize_newlines(utf8, utf8_len - 1, &normalized_len);
	free(utf8);
	if (normalized == NULL) {
		return (NULL, 0, false);
	}
	return (normalized, normalized_len, true);
}
#endif

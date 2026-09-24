// regex — tiny regular expression matcher for Jem.
//
// Adapted from tiny-regex-c by kokke (Unlicense):
// https://github.com/kokke/tiny-regex-c
// Same dialect as jem.c util_regex.
//
// Dialect: . ^ $ (...) * + ? \s \S \w \W \d \D
// Byte-oriented over the view; UTF-8 literals match byte-for-byte.
// '.' matches any byte including newline. Classes are ASCII.
//
// Pattern and text are char[..] views (len bytes). No NUL required.
#include <ctype.h>
#include <stdbool.h>
#include <stddef.h>
#include <sys/types.h>

enum {
	ReMaxCaptures = 10,
	ReMaxTokens = 30
};

// Byte span into the matched text; start/length are -1 when unmatched.
struct ReCapture {
	ssize_t start;
	ssize_t length;
};

enum ReStatus {
	ReNone = 0,
	ReFound = 1,
	ReInvalid = 2
};

enum ReType {
	ReUnused,
	ReDot,
	ReBegin,
	ReEnd,
	ReGroupStart,
	ReGroupEnd,
	ReQuestion,
	ReStar,
	RePlus,
	ReChar,
	ReDigit,
	ReNotDigit,
	ReWord,
	ReNotWord,
	ReSpace,
	ReNotSpace
};

struct ReToken {
	ReType type;
	char ch;
};

static bool re_is_quantifier(ReType type) {
	return type == ReQuestion || type == ReStar || type == RePlus;
}

static bool re_can_match_char(ReType type) {
	switch (type) {
	case ReDot:
	case ReChar:
	case ReDigit:
	case ReNotDigit:
	case ReWord:
	case ReNotWord:
	case ReSpace:
	case ReNotSpace:
		return true;
	case ReUnused:
	case ReBegin:
	case ReEnd:
	case ReGroupStart:
	case ReGroupEnd:
	case ReQuestion:
	case ReStar:
	case RePlus:
		return false;
	}
	return false;
}

static bool re_can_repeat(ReType type) {
	return re_can_match_char(type) && !re_is_quantifier(type);
}

static bool re_is_digit(char c) {
	return isdigit(c) != 0;
}

static bool re_is_word(char c) {
	return c == '_' || isalnum(c) != 0;
}

static bool re_is_space(char c) {
	return isspace(c) != 0;
}

static bool re_one(ReToken token, char c) {
	switch (token.type) {
	case ReDot:
		return true;
	case ReChar:
		return token.ch == c;
	case ReDigit:
		return re_is_digit(c);
	case ReNotDigit:
		return !re_is_digit(c);
	case ReWord:
		return re_is_word(c);
	case ReNotWord:
		return !re_is_word(c);
	case ReSpace:
		return re_is_space(c);
	case ReNotSpace:
		return !re_is_space(c);
	case ReUnused:
	case ReBegin:
	case ReEnd:
	case ReGroupStart:
	case ReGroupEnd:
	case ReQuestion:
	case ReStar:
	case RePlus:
		return false;
	}
	return false;
}

static void re_caps_clear(ReCapture *groups, size_t n) {
	size_t i = 0;
	if (groups == NULL) {
		return;
	}
	for (i = 0; i < n; i++) {
		groups[i].start = -1;
		groups[i].length = -1;
	}
}

static void re_caps_copy(ReCapture *dst, ReCapture *src, size_t n) {
	size_t i = 0;
	if (dst == NULL || src == NULL) {
		return;
	}
	for (i = 0; i < n; i++) {
		dst[i] = src[i];
	}
}

static bool re_compile(const char *pattern, size_t pat_len, ReToken *compiled, size_t capacity) {
	size_t i = 0;
	size_t j = 0;
	char group_stack[ReMaxCaptures - 1] = {0};
	size_t group_depth = 0;
	char next_group = 1;

	if (pattern == NULL || compiled == NULL || capacity == 0) {
		return false;
	}
	while (i < pat_len) {
		ReType type = ReUnused;
		char ch = 0;

		if (j + 1 >= capacity) {
			return false;
		}
		switch (pattern[i]) {
		case '^':
			if (j != 0) {
				return false;
			}
			type = ReBegin;
			break;
		case '$':
			if (i + 1 != pat_len) {
				return false;
			}
			type = ReEnd;
			break;
		case '.':
			type = ReDot;
			break;
		case '(':
			if (next_group >= ReMaxCaptures || group_depth >= ReMaxCaptures - 1) {
				return false;
			}
			type = ReGroupStart;
			ch = next_group;
			group_stack[group_depth] = next_group;
			group_depth++;
			next_group++;
			break;
		case ')':
			if (group_depth == 0) {
				return false;
			}
			group_depth--;
			type = ReGroupEnd;
			ch = group_stack[group_depth];
			break;
		case '*':
			if (j == 0 || !re_can_repeat(compiled[j - 1].type)) {
				return false;
			}
			type = ReStar;
			break;
		case '+':
			if (j == 0 || !re_can_repeat(compiled[j - 1].type)) {
				return false;
			}
			type = RePlus;
			break;
		case '?':
			if (j == 0 || !re_can_repeat(compiled[j - 1].type)) {
				return false;
			}
			type = ReQuestion;
			break;
		case '[':
		case ']':
			return false;
		case '\\':
			i++;
			if (i >= pat_len) {
				return false;
			}
			switch (pattern[i]) {
			case 'd':
				type = ReDigit;
				break;
			case 'D':
				type = ReNotDigit;
				break;
			case 'w':
				type = ReWord;
				break;
			case 'W':
				type = ReNotWord;
				break;
			case 's':
				type = ReSpace;
				break;
			case 'S':
				type = ReNotSpace;
				break;
			default:
				type = ReChar;
				ch = pattern[i];
				break;
			}
			break;
		default:
			type = ReChar;
			ch = pattern[i];
			break;
		}
		compiled[j].type = type;
		compiled[j].ch = ch;
		i++;
		j++;
	}
	if (group_depth != 0) {
		return false;
	}
	compiled[j].type = ReUnused;
	compiled[j].ch = 0;
	return true;
}

static bool re_match_at(ReToken *pattern, const char *text, const char *text_end,
	ssize_t *matchlength, ReCapture *groups, size_t group_count) {
	ssize_t pre = 0;
	ReCapture saved[ReMaxCaptures] = {0};

	if (matchlength == NULL) {
		return false;
	}
	pre = *matchlength;
	re_caps_copy(saved, groups, group_count);
	for (;;) {
		if (pattern[0].type == ReUnused) {
			return true;
		}
		if (pattern[0].type == ReGroupStart) {
			if (groups != NULL && pattern[0].ch < (char)group_count) {
				groups[pattern[0].ch].start = *matchlength;
				groups[pattern[0].ch].length = -1;
			}
			pattern++;
			continue;
		}
		if (pattern[0].type == ReGroupEnd) {
			if (groups != NULL && pattern[0].ch < (char)group_count) {
				groups[pattern[0].ch].length =
					*matchlength - groups[pattern[0].ch].start;
			}
			pattern++;
			continue;
		}
		if (pattern[1].type == ReQuestion) {
			ssize_t prelen = *matchlength;
			ReCapture question_saved[ReMaxCaptures] = {0};
			re_caps_copy(question_saved, groups, group_count);
			if (re_match_at(&pattern[2], text, text_end, matchlength, groups, group_count)) {
				return true;
			}
			*matchlength = prelen;
			re_caps_copy(groups, question_saved, group_count);
			if (text < text_end && re_one(pattern[0], *text) &&
				re_match_at(&pattern[2], text + 1, text_end, matchlength, groups, group_count)) {
				(*matchlength)++;
				return true;
			}
			*matchlength = prelen;
			re_caps_copy(groups, question_saved, group_count);
			return false;
		}
		if (pattern[1].type == ReStar || pattern[1].type == RePlus) {
			bool require_one = pattern[1].type == RePlus;
			ssize_t prelen = *matchlength;
			const char *prepoint = text;
			ReCapture repeat_saved[ReMaxCaptures] = {0};
			re_caps_copy(repeat_saved, groups, group_count);
			while (text < text_end && re_one(pattern[0], text[0])) {
				text++;
				(*matchlength)++;
			}
			if (require_one && text == prepoint) {
				*matchlength = prelen;
				re_caps_copy(groups, repeat_saved, group_count);
				return false;
			}
			while (text >= prepoint + (require_one ? 1 : 0)) {
				ReCapture attempt[ReMaxCaptures] = {0};
				re_caps_copy(attempt, groups, group_count);
				if (re_match_at(&pattern[2], text, text_end, matchlength, groups, group_count)) {
					return true;
				}
				text--;
				re_caps_copy(groups, attempt, group_count);
				(*matchlength)--;
			}
			*matchlength = prelen;
			re_caps_copy(groups, repeat_saved, group_count);
			return false;
		}
		if (pattern[0].type == ReEnd && pattern[1].type == ReUnused) {
			return text >= text_end;
		}
		if (text >= text_end || !re_one(*pattern, text[0])) {
			*matchlength = pre;
			re_caps_copy(groups, saved, group_count);
			return false;
		}
		(*matchlength)++;
		pattern++;
		text++;
	}
}

static ssize_t re_search_compiled(ReToken *pattern, const char *text, const char *text_end,
	ssize_t *matchlength, ReCapture *groups, size_t group_count) {
	ssize_t idx = 0;

	*matchlength = 0;
	if (pattern[0].type == ReBegin) {
		re_caps_clear(groups, group_count);
		if (!re_match_at(&pattern[1], text, text_end, matchlength, groups, group_count)) {
			return -1;
		}
		if (groups != NULL && group_count > 0) {
			groups[0].start = 0;
			groups[0].length = *matchlength;
		}
		return 0;
	}
	while (text < text_end) {
		size_t i = 0;
		re_caps_clear(groups, group_count);
		if (re_match_at(pattern, text, text_end, matchlength, groups, group_count)) {
			if (groups != NULL && group_count > 0) {
				groups[0].start = idx;
				groups[0].length = *matchlength;
				for (i = 1; i < group_count; i++) {
					if (groups[i].start >= 0) {
						groups[i].start = groups[i].start + idx;
					}
				}
			}
			return idx;
		}
		idx++;
		text++;
	}
	return -1;
}

static (ReStatus, size_t, size_t) re_find_raw(const char *pattern, size_t pat_len,
	const char *text, size_t text_len, ReCapture *groups, size_t group_count) {
	ReToken compiled[ReMaxTokens] = {0};
	ssize_t matchlength = 0;
	ssize_t offset = -1;

	re_caps_clear(groups, group_count);
	if (pattern == NULL || text == NULL) {
		return (ReInvalid, 0, 0);
	}
	if (!re_compile(pattern, pat_len, compiled, ReMaxTokens)) {
		return (ReInvalid, 0, 0);
	}
	offset = re_search_compiled(compiled, text, text + text_len, &matchlength, groups, group_count);
	if (offset < 0) {
		return (ReNone, 0, 0);
	}
	return (ReFound, offset, matchlength);
}

// Find the first match of pattern in text.
// On ReFound, offset/length are byte indices into text.
(ReStatus, size_t, size_t) re_find(const char[..] pattern, const char[..] text) {
	return re_find_raw(ptr(pattern), len(pattern), ptr(text), len(text), NULL, 0);
}

// Like re_find, and fill groups: [0] = whole match, [1..] = (... ) captures.
// Pass an empty groups view to skip captures. At most ReMaxCaptures slots are used.
(ReStatus, size_t, size_t) re_find_groups(const char[..] pattern, const char[..] text, ReCapture[..] groups) {
	ReCapture *gp = NULL;
	size_t gn = 0;
	if (len(groups) > 0) {
		gp = ptr(groups);
		gn = len(groups);
		if (gn > ReMaxCaptures) {
			gn = ReMaxCaptures;
		}
	}
	return re_find_raw(ptr(pattern), len(pattern), ptr(text), len(text), gp, gn);
}

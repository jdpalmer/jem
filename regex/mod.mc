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
	ReMaxTokens = 64
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

static void re_caps_clear(ReCapture[..] groups) {
	size_t i = 0;
	for (; i < len(groups); i++) {
		groups[i].start = -1;
		groups[i].length = -1;
	}
}

static void re_caps_copy(ReCapture[..] dst, ReCapture[..] src) {
	size_t i = 0;
	size_t n = len(dst);
	if (n > len(src)) {
		n = len(src);
	}
	for (; i < n; i++) {
		dst[i] = src[i];
	}
}

static bool re_compile(const char[..] pattern, ReToken[..] compiled) {
	size_t i = 0;
	size_t j = 0;
	char group_stack[ReMaxCaptures - 1] = {0};
	size_t group_depth = 0;
	char next_group = 1;
	size_t pat_len = len(pattern);
	size_t capacity = len(compiled);

	if (capacity == 0) {
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

static bool re_match_at(ReToken[..] pattern, const char[..] text, ssize_t *matchlength,
	ReCapture[..] groups) {
	ReCapture saved[ReMaxCaptures] = {0};
	ReCapture[..] saved_view = saved;

	if (matchlength == NULL) {
		return false;
	}
	ssize_t pre = *matchlength;
	re_caps_copy(saved_view, groups);
	for (;;) {
		if (len(pattern) == 0 || pattern[0].type == ReUnused) {
			return true;
		}
		if (pattern[0].type == ReGroupStart) {
			if (pattern[0].ch < (char)len(groups)) {
				groups[pattern[0].ch].start = *matchlength;
				groups[pattern[0].ch].length = -1;
			}
			pattern = pattern[1..];
			continue;
		}
		if (pattern[0].type == ReGroupEnd) {
			if (pattern[0].ch < (char)len(groups)) {
				groups[pattern[0].ch].length =
					*matchlength - groups[pattern[0].ch].start;
			}
			pattern = pattern[1..];
			continue;
		}
		if (len(pattern) > 1 && pattern[1].type == ReQuestion) {
			ssize_t prelen = *matchlength;
			ReCapture question_saved[ReMaxCaptures] = {0};
			ReCapture[..] qview = question_saved;
			re_caps_copy(qview, groups);
			if (re_match_at(pattern[2..], text, matchlength, groups)) {
				return true;
			}
			*matchlength = prelen;
			re_caps_copy(groups, qview);
			if (len(text) > 0 && re_one(pattern[0], text[0]) &&
				re_match_at(pattern[2..], text[1..], matchlength, groups)) {
				(*matchlength)++;
				return true;
			}
			*matchlength = prelen;
			re_caps_copy(groups, qview);
			return false;
		}
		if (len(pattern) > 1 && (pattern[1].type == ReStar || pattern[1].type == RePlus)) {
			bool require_one = pattern[1].type == RePlus;
			ssize_t prelen = *matchlength;
			size_t consumed = 0;
			ReCapture repeat_saved[ReMaxCaptures] = {0};
			ReCapture[..] rview = repeat_saved;
			re_caps_copy(rview, groups);
			while (len(text) > 0 && re_one(pattern[0], text[0])) {
				text = text[1..];
				consumed++;
				(*matchlength)++;
			}
			if (require_one && consumed == 0) {
				*matchlength = prelen;
				re_caps_copy(groups, rview);
				return false;
			}
			size_t min_keep = require_one ? 1 : 0;
			while (consumed >= min_keep) {
				ReCapture attempt[ReMaxCaptures] = {0};
				ReCapture[..] aview = attempt;
				re_caps_copy(aview, groups);
				if (re_match_at(pattern[2..], text, matchlength, groups)) {
					return true;
				}
				if (consumed == 0) {
					break;
				}
				/* Walk back one matched char: rebuild remaining text length. */
				text = ranged(ptr(text) - 1, len(text) + 1);
				consumed--;
				re_caps_copy(groups, aview);
				(*matchlength)--;
			}
			*matchlength = prelen;
			re_caps_copy(groups, rview);
			return false;
		}
		if (pattern[0].type == ReEnd &&
			(len(pattern) == 1 || pattern[1].type == ReUnused)) {
			return len(text) == 0;
		}
		if (len(text) == 0 || !re_one(pattern[0], text[0])) {
			*matchlength = pre;
			re_caps_copy(groups, saved_view);
			return false;
		}
		(*matchlength)++;
		pattern = pattern[1..];
		text = text[1..];
	}
}

static ssize_t re_search_compiled(ReToken[..] pattern, const char[..] text,
	ssize_t *matchlength, ReCapture[..] groups) {
	ssize_t idx = 0;
	const char[..] rest = text;

	*matchlength = 0;
	if (len(pattern) > 0 && pattern[0].type == ReBegin) {
		re_caps_clear(groups);
		if (!re_match_at(pattern[1..], rest, matchlength, groups)) {
			return -1;
		}
		if (len(groups) > 0) {
			groups[0].start = 0;
			groups[0].length = *matchlength;
		}
		return 0;
	}
	while (len(rest) > 0) {
		size_t i = 0;
		re_caps_clear(groups);
		if (re_match_at(pattern, rest, matchlength, groups)) {
			if (len(groups) > 0) {
				groups[0].start = idx;
				groups[0].length = *matchlength;
				for (i = 1; i < len(groups); i++) {
					if (groups[i].start >= 0) {
						groups[i].start = groups[i].start + idx;
					}
				}
			}
			return idx;
		}
		idx++;
		rest = rest[1..];
	}
	return -1;
}

static (ReStatus, size_t, size_t) re_find_raw(const char[..] pattern, const char[..] text,
	ReCapture[..] groups) {
	ReToken compiled[ReMaxTokens] = {0};
	ssize_t matchlength = 0;

	re_caps_clear(groups);
	if (!re_compile(pattern, compiled)) {
		return (ReInvalid, 0, 0);
	}
	ssize_t offset = re_search_compiled(compiled, text, &matchlength, groups);
	if (offset < 0) {
		return (ReNone, 0, 0);
	}
	return (ReFound, offset, matchlength);
}

// Find the first match of pattern in text.
// On ReFound, offset/length are byte indices into text.
(ReStatus, size_t, size_t) re_find(const char[..] pattern, const char[..] text) {
	ReCapture[..] nogroups = ranged((ReCapture *)0, 0);
	return re_find_raw(pattern, text, nogroups);
}

// Like re_find, and fill groups: [0] = whole match, [1..] = (... ) captures.
// Pass an empty groups view to skip captures. At most ReMaxCaptures slots are used.
(ReStatus, size_t, size_t) re_find_groups(const char[..] pattern, const char[..] text, ReCapture[..] groups) {
	if (len(groups) > ReMaxCaptures) {
		groups = groups[..ReMaxCaptures];
	}
	return re_find_raw(pattern, text, groups);
}

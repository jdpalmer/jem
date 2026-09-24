// DFA tokenizer: syntax_highlight_line paints per-byte styles for one line.
#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>

static TextStyle a_normal() {
	return text_style_make(TermColorDefault, TermColorDefault, 0);
}

static TextStyle a_comment() {
	return text_style_make(TermColorBase01, TermColorDefault, 0);
}

static TextStyle a_string() {
	return text_style_make(TermColorCyan, TermColorDefault, 0);
}

static TextStyle a_number() {
	return text_style_make(TermColorMagenta, TermColorDefault, 0);
}

static TextStyle a_preproc() {
	return text_style_make(TermColorRed, TermColorDefault, 0);
}

static TextStyle a_heading() {
	return text_style_make(TermColorYellow, TermColorDefault, (uint16_t)TextStyleBold);
}

static TextStyle a_bold() {
	return text_style_make(TermColorDefault, TermColorDefault, (uint16_t)TextStyleBold);
}

static void put_paint(TextStyle[..] styles, size_t i, TextStyle val) {
	TextStyle *sp = ptr(styles);
	if (sp != NULL && i < len(styles)) {
		sp[i] = val;
	}
}

static void fill_paint(TextStyle[..] styles, size_t a, size_t b, TextStyle val) {
	size_t j = 0;
	size_t n = len(styles);
	if (b > n) {
		b = n;
	}
	{
		TextStyle *sp = ptr(styles);
		if (sp == NULL) {
			return;
		}
		for (j = a; j < b; j++) {
			sp[j] = val;
		}
	}
}

static int line_byte(char[..] line, size_t i) {
	if (i >= len(line)) {
		return 0;
	}
	{
		char *lp = ptr(line);
		if (lp == NULL) {
			return 0;
		}
		return lp[i];
	}
}

static bool is_number_cont(int c) {
	return (c >= '0' && c <= '9') ||
		(c >= 'a' && c <= 'f') ||
		(c >= 'A' && c <= 'F') ||
		c == '.' || c == '_' ||
		c == 'x' || c == 'X' ||
		c == 'b' || c == 'B' ||
		c == 'l' || c == 'L' ||
		c == 'u' || c == 'U' ||
		c == 'f' || c == 'F' ||
		c == 'e' || c == 'E' ||
		c == '+' || c == '-';
}

static bool is_ident_start(int c, uint32_t flags) {
	if ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') {
		return true;
	}
	if ((flags & (uint32_t)ModeFlagIdentLispSigil) != 0 && (c == '#' || c == '&')) {
		return true;
	}
	return false;
}

static bool is_ident_cont(int c, uint32_t flags) {
	if ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_') {
		return true;
	}
	if ((flags & (uint32_t)ModeFlagIdentDash) != 0 && c == '-') {
		return true;
	}
	if ((flags & (uint32_t)ModeFlagIdentLispExtra) != 0) {
		switch (c) {
		case '-':
		case '?':
		case '!':
		case '/':
		case '<':
		case '>':
		case '=':
		case '+':
		case '*':
		case '.':
		case '#':
			return true;
		default:
			break;
		}
	}
	if (c > 127) {
		return true;
	}
	return false;
}

static bool is_operator_char(int c) {
	switch (c) {
	case '+':
	case '-':
	case '*':
	case '/':
	case '%':
	case '=':
	case '<':
	case '>':
	case '!':
	case '&':
	case '|':
	case '^':
	case '~':
	case '?':
	case ':':
	case '.':
	case '#':
	case '$':
	case '@':
		return true;
	default:
		return false;
	}
}

static TextStyle paren_style(TermColor base, int depth) {
	TermColor cycle[4] = {TermColorBlack, TermColorCyan, TermColorRed, TermColorYellow};
	TermColor fg = base;
	if ((depth & 3) != 0) {
		fg = cycle[depth & 3];
	}
	return text_style_make(fg, TermColorDefault, (uint16_t)TextStyleBold);
}

static void paint_delimiter(SynState* syn, int ch, size_t idx, TextStyle[..] styles) {
	int di = -1;
	TermColor color = TermColorMagenta;
	uint8_t* dptr = 0;
	uint8_t old = 0;
	uint8_t meta = 0;
	int depth = 0;
	int open = 0;
	int close = 0;

	if (ch == '(' || ch == ')') {
		di = 0;
		color = TermColorMagenta;
		open = '(';
		close = ')';
		dptr = &syn.paren;
	} else if (ch == '[' || ch == ']') {
		di = 1;
		color = TermColorBlue;
		open = '[';
		close = ']';
		dptr = &syn.bracket;
	} else if (ch == '{' || ch == '}') {
		di = 2;
		color = TermColorGreen;
		open = '{';
		close = '}';
		dptr = &syn.curly;
	}
	if (di < 0 || dptr == 0) {
		return;
	}
	old = *dptr;
	meta = (uint8_t)(old & 0x80);
	if (ch == open) {
		depth = old & 0x7F;
		put_paint(styles, idx, paren_style(color, depth));
		*dptr = (uint8_t)(((depth + 1) & 0x7F) | meta);
	} else {
		depth = old & 0x7F;
		if (depth > 0) {
			depth = depth - 1;
		}
		*dptr = (uint8_t)((depth & 0x7F) | meta);
		put_paint(styles, idx, paren_style(color, depth));
	}
	(void)close;
}

static void paint_ident_range(char[..] line, size_t start, size_t end, LangMode lm, TextStyle[..] styles) {
	TextStyle a = TextStyleDefault;
	char[..] word = {0};
	if (start >= end || end > len(styles) || end > len(line)) {
		return;
	}
	word = line[start..end];
	a = ident_style_for(lm, word);
	if (text_style_eq(a, TextStyleDefault)) {
		a = a_normal();
	}
	fill_paint(styles, start, end, a);
}

static void highlight_markdown(char[..] line, TextStyle[..] styles) {
	size_t n = len(styles);
	size_t i = 0;
	bool in_bold = false;
	bool in_code = false;
	size_t bold_start = 0;
	size_t code_start = 0;
	int c = 0;
	int nc = 0;

	if (n == 0 || len(line) == 0) {
		return;
	}
	for (i = 0; i < n && (line_byte(line, i) == ' ' || line_byte(line, i) == '\t'); i++) {
	}
	if (i >= n) {
		return;
	}
	if (line_byte(line, i) == '#') {
		fill_paint(styles, 0, n, a_heading());
		return;
	}
	in_bold = false;
	in_code = false;
	bold_start = 0;
	code_start = 0;
	while (i < n) {
		c = line_byte(line, i);
		nc = line_byte(line, i + 1);
		if (!in_code && c == '*' && nc == '*') {
			if (!in_bold) {
				in_bold = true;
				bold_start = i;
			} else {
				fill_paint(styles, bold_start, i + 2, a_bold());
				in_bold = false;
			}
			i = i + 2;
			continue;
		}
		if (c == '`') {
			if (!in_code) {
				in_code = true;
				code_start = i;
			} else {
				fill_paint(styles, code_start, i + 1, a_string());
				in_code = false;
			}
			i = i + 1;
			continue;
		}
		if (in_code) {
			put_paint(styles, i, a_string());
		} else if (in_bold) {
			put_paint(styles, i, a_bold());
		}
		i = i + 1;
	}
}

static void highlight_html(char[..] line, TextStyle[..] styles, SynState* syn) {
	size_t n = len(styles);
	size_t i = 0;
	int c = 0;
	int dfa = 0;

	for (i = 0; i < n; i++) {
		c = line_byte(line, i);
		dfa = syn.dfa;
		if (dfa == (int)SynStateHTMLCmt || dfa == (int)SynStateHTMLCmtD1 || dfa == (int)SynStateHTMLCmtD2) {
			put_paint(styles, i, a_comment());
			if (c == '-') {
				if (dfa == (int)SynStateHTMLCmtD1) {
					syn.dfa = SynStateHTMLCmtD2;
				} else {
					syn.dfa = SynStateHTMLCmtD1;
				}
			} else if (c == '>' && dfa == (int)SynStateHTMLCmtD2) {
				syn.dfa = SynStateNormal;
			} else {
				syn.dfa = SynStateHTMLCmt;
			}
		} else if (c == '<' &&
			i + 3 < n &&
			line_byte(line, i + 1) == '!' &&
			line_byte(line, i + 2) == '-' &&
			line_byte(line, i + 3) == '-') {
			fill_paint(styles, i, i + 4, a_comment());
			syn.dfa = SynStateHTMLCmt;
			i = i + 3;
		} else {
			put_paint(styles, i, a_normal());
		}
	}
}

static void on_enter(int state, char[..] line, size_t i, size_t token_start, TextStyle[..] styles) {
	int c = 0;
	switch (state) {
	case SynStateNormal:
		c = line_byte(line, i);
		put_paint(styles, i, a_normal());
		(void)c;
		break;
	case SynStateNumber:
		put_paint(styles, i, a_number());
		break;
	case SynStateStringD:
	case SynStateStringDEsc:
	case SynStateStringS:
	case SynStateStringSEsc:
		put_paint(styles, i, a_string());
		break;
	case SynStateCmtLine:
	case SynStateCmtBlock:
	case SynStateCmtStar:
	case SynStateCmtBrace:
	case SynStateCmtParen:
	case SynStateCmtParen2:
		put_paint(styles, i, a_comment());
		break;
	case SynStateLuaDash:
		c = line_byte(line, i);
		if (c == '-') {
			put_paint(styles, token_start, a_comment());
			put_paint(styles, i, a_comment());
		} else {
			put_paint(styles, token_start, a_normal());
		}
		break;
	case SynStateLuaBlock:
	case SynStateLuaBlkEnd:
	case SynStateHTMLCmt:
	case SynStateHTMLCmtD1:
	case SynStateHTMLCmtD2:
		put_paint(styles, i, a_comment());
		break;
	default:
		break;
	}
}

static bool skip_enter_on_string_close(int cur, int new_state, int c) {
	if (new_state != (int)SynStateNormal) {
		return false;
	}
	if (cur == (int)SynStateStringD) {
		return c == '"';
	}
	if (cur == (int)SynStateStringS) {
		return c == '\'';
	}
	return false;
}

static void on_exit(int state, char[..] line, size_t i, size_t token_start, LangMode lm, TextStyle[..] styles) {
	if (state == (int)SynStateIdent) {
		paint_ident_range(line, token_start, i, lm, styles);
	}
}

// syntax_highlight_line paints styles[0..min(len(line),len(styles))) and
// returns the SynState after the line (for the next line's prev).
// prev may be NULL (treated as zero SynState).
SynState syntax_highlight_line(LangMode lang, SynState* prev, char[..] line, TextStyle[..] styles) {
	Spec info = {ModeSyntaxGeneral};
	SynState syn = {SynStateNormal};
	size_t n = 0;
	size_t i = 0;
	size_t token_start = 0;
	uint32_t flags = 0;
	int cur = 0;
	int c = 0;
	int lookahead = 0;
	int new_state = 0;
	bool reprocess = false;
	bool any_before = false;
	size_t k = 0;

	if (prev != 0) {
		syn = *prev;
	}
	info = syntax_spec_for(lang);
	n = len(line);
	if (len(styles) < n) {
		n = len(styles);
	}
	{
		TextStyle *sp = ptr(styles);
		if (sp != NULL) {
			for (i = 0; i < n; i++) {
				sp[i] = a_normal();
			}
		}
	}

	switch (info.kind) {
	case ModeSyntaxNone:
		syn.dfa = SynStateNormal;
		syn.paren = 0;
		syn.bracket = 0;
		syn.curly = 0;
		return syn;
	case ModeSyntaxHashCommentOnly:
		syn.dfa = SynStateNormal;
		syn.paren = 0;
		syn.bracket = 0;
		syn.curly = 0;
		for (i = 0; i < n; i++) {
			if (line_byte(line, i) == '#') {
				fill_paint(styles, i, n, a_comment());
				syn.dfa = SynStateCmtLine;
				break;
			}
		}
		return syn;
	case ModeSyntaxMarkdown:
		highlight_markdown(line, styles);
		return syn;
	case ModeSyntaxHTML:
		highlight_html(line, styles, &syn);
		syn.paren = 0;
		syn.bracket = 0;
		syn.curly = 0;
		return syn;
	default:
		break;
	}

	if (syn.dfa == SynStatePreproc) {
		fill_paint(styles, 0, n, a_preproc());
		if (n == 0 || line_byte(line, n - 1) != '\\') {
			syn.dfa = SynStateNormal;
		}
		return syn;
	}

	flags = info.flags;
	token_start = 0;
	i = 0;
	while (i < n) {
		cur = syn.dfa;
		if (cur < 0 || cur >= (int)SynStateCount) {
			syn.dfa = SynStateNormal;
			cur = SynStateNormal;
		}
		lookahead = line_byte(line, i + 1);
		on_enter(cur, line, i, token_start, styles);
		reprocess = false;
		c = line_byte(line, i);

		switch (cur) {
		case SynStateNormal:
			if ((flags & (uint32_t)ModeFlagCommentSlashLine) != 0 && c == '/' && lookahead == '/') {
				fill_paint(styles, i, n, a_comment());
				syn.dfa = SynStateCmtLine;
				i = n;
				break;
			}
			if ((flags & (uint32_t)ModeFlagCommentSlashBlock) != 0 && c == '/' && lookahead == '*') {
				put_paint(styles, i, a_comment());
				put_paint(styles, i + 1, a_comment());
				syn.dfa = SynStateCmtBlock;
				i = i + 1;
				break;
			}
			if ((flags & (uint32_t)ModeFlagPreprocHashAtBOL) != 0 && c == '#') {
				any_before = false;
				for (k = 0; k < i; k++) {
					if (line_byte(line, k) != ' ' && line_byte(line, k) != '\t') {
						any_before = true;
						break;
					}
				}
				if (!any_before) {
					fill_paint(styles, 0, n, a_preproc());
					if (n != 0 && line_byte(line, n - 1) == '\\') {
						syn.dfa = SynStatePreproc;
					} else {
						syn.dfa = SynStateNormal;
					}
					i = n;
					break;
				}
			}
			if ((flags & (uint32_t)ModeFlagCommentHash) != 0 && c == '#') {
				fill_paint(styles, i, n, a_comment());
				syn.dfa = SynStateCmtLine;
				i = n;
				break;
			}
			if ((flags & (uint32_t)ModeFlagCommentSemi) != 0 && c == ';') {
				fill_paint(styles, i, n, a_comment());
				syn.dfa = SynStateCmtLine;
				i = n;
				break;
			}
			if ((flags & (uint32_t)ModeFlagCommentLua) != 0 && c == '-') {
				token_start = i;
				syn.dfa = SynStateLuaDash;
				put_paint(styles, i, a_normal());
				break;
			}
			if ((flags & (uint32_t)ModeFlagCommentPascalBrace) != 0 && c == '{') {
				put_paint(styles, i, a_comment());
				syn.dfa = SynStateCmtBrace;
				break;
			}
			if ((flags & (uint32_t)ModeFlagCommentPascalParen) != 0 && c == '(' && lookahead == '*') {
				put_paint(styles, i, a_comment());
				put_paint(styles, i + 1, a_comment());
				syn.dfa = SynStateCmtParen;
				i = i + 1;
				break;
			}
			if (c == '"') {
				syn.dfa = SynStateStringD;
				break;
			}
			if (c == '\'') {
				syn.dfa = SynStateStringS;
				break;
			}
			if ((c >= '0' && c <= '9') || (c == '.' && lookahead >= '0' && lookahead <= '9')) {
				token_start = i;
				syn.dfa = SynStateNumber;
				break;
			}
			if (is_ident_start(c, flags)) {
				token_start = i;
				syn.dfa = SynStateIdent;
				break;
			}
			if ((flags & (uint32_t)ModeFlagAtRule) != 0 && c == '@' && lookahead >= 'a' && lookahead <= 'z') {
				put_paint(styles, i, a_preproc());
				token_start = i + 1;
				syn.dfa = SynStateIdent;
				break;
			}
			if (c == '(' || c == ')' || c == '[' || c == ']' ||
				(c == '{' && (flags & (uint32_t)ModeFlagNoCurlyRainbow) == 0) ||
				(c == '}' && (flags & (uint32_t)ModeFlagNoCurlyRainbow) == 0)) {
				paint_delimiter(&syn, c, i, styles);
				break;
			}
			if (is_operator_char(c)) {
				token_start = i;
				syn.dfa = SynStateOperator;
				break;
			}
			break;
		case SynStateIdent:
			if (!is_ident_cont(c, flags)) {
				syn.dfa = SynStateNormal;
				reprocess = true;
			}
			break;
		case SynStateOperator:
			if (!is_operator_char(c)) {
				syn.dfa = SynStateNormal;
				reprocess = true;
			}
			break;
		case SynStateNumber:
			if (!is_number_cont(c)) {
				syn.dfa = SynStateNormal;
				reprocess = true;
			}
			break;
		case SynStateStringD:
			if (c == '"') {
				syn.dfa = SynStateNormal;
			} else if (c == '\\') {
				syn.dfa = SynStateStringDEsc;
			}
			break;
		case SynStateStringDEsc:
			syn.dfa = SynStateStringD;
			break;
		case SynStateStringS:
			if (c == '\'') {
				syn.dfa = SynStateNormal;
			} else if (c == '\\') {
				syn.dfa = SynStateStringSEsc;
			}
			break;
		case SynStateStringSEsc:
			syn.dfa = SynStateStringS;
			break;
		case SynStateCmtBlock:
			if (c == '*') {
				syn.dfa = SynStateCmtStar;
			}
			break;
		case SynStateCmtStar:
			if (c == '/') {
				syn.dfa = SynStateNormal;
			} else if (c != '*') {
				syn.dfa = SynStateCmtBlock;
			}
			break;
		case SynStateCmtBrace:
			if (c == '}') {
				syn.dfa = SynStateNormal;
			}
			break;
		case SynStateCmtParen:
			if (c == '*') {
				syn.dfa = SynStateCmtParen2;
			}
			break;
		case SynStateCmtParen2:
			if (c == ')') {
				syn.dfa = SynStateNormal;
			} else if (c != '*') {
				syn.dfa = SynStateCmtParen;
			}
			break;
		case SynStateCmtLine:
			break;
		case SynStateLuaDash:
			if (c == '-') {
				if (lookahead == '[' && line_byte(line, i + 2) == '[') {
					syn.dfa = SynStateLuaBlock;
				} else {
					syn.dfa = SynStateCmtLine;
				}
			} else {
				syn.dfa = SynStateNormal;
				reprocess = true;
			}
			break;
		case SynStateLuaBlock:
			if (c == ']') {
				syn.dfa = SynStateLuaBlkEnd;
			}
			break;
		case SynStateLuaBlkEnd:
			if (c == ']') {
				syn.dfa = SynStateNormal;
			} else {
				syn.dfa = SynStateLuaBlock;
			}
			break;
		case SynStateHTMLCmt:
			if (c == '-') {
				syn.dfa = SynStateHTMLCmtD1;
			}
			break;
		case SynStateHTMLCmtD1:
			if (c == '-') {
				syn.dfa = SynStateHTMLCmtD2;
			} else {
				syn.dfa = SynStateHTMLCmt;
			}
			break;
		case SynStateHTMLCmtD2:
			if (c == '>') {
				syn.dfa = SynStateNormal;
			} else if (c != '-') {
				syn.dfa = SynStateHTMLCmt;
			}
			break;
		default:
			syn.dfa = SynStateNormal;
			break;
		}

		new_state = syn.dfa;
		if (new_state < 0 || new_state >= (int)SynStateCount) {
			syn.dfa = SynStateNormal;
			new_state = SynStateNormal;
		}
		if (new_state != cur) {
			on_exit(cur, line, i, token_start, lang, styles);
			if (!reprocess && !skip_enter_on_string_close(cur, new_state, c)) {
				on_enter(new_state, line, i, token_start, styles);
			}
		}
		if (!reprocess) {
			i = i + 1;
		}
	}

	if (syn.dfa == SynStateIdent) {
		paint_ident_range(line, token_start, n, lang, styles);
		syn.dfa = SynStateNormal;
	}
	if (syn.dfa == SynStateOperator) {
		syn.dfa = SynStateNormal;
	}

	switch (syn.dfa) {
	case SynStateCmtLine:
	case SynStateNumber:
	case SynStateStringD:
	case SynStateStringDEsc:
	case SynStateStringS:
	case SynStateStringSEsc:
	case SynStateLuaDash:
		syn.dfa = SynStateNormal;
		break;
	default:
		break;
	}
	return syn;
}

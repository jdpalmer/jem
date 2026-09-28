// syntax — DFA line highlighter (ported from jem.go/syntax).
//
// Self-contained TextStyle / SynState / LangMode matching jem.go/buffer values.
// Callers paint with syntax_highlight_line; buffer can import these types later.
#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>

// Terminal / style colors (jem.go/buffer.TermColor).
enum TermColor {
	TermColorBlack = 0,
	TermColorRed = 1,
	TermColorGreen = 2,
	TermColorYellow = 3,
	TermColorBlue = 4,
	TermColorMagenta = 5,
	TermColorCyan = 6,
	TermColorWhite = 7,
	TermColorDefault = 16,
	TermColorBase03 = 17,
	TermColorBase02 = 18,
	TermColorBase01 = 19,
	TermColorBase00 = 20,
	TermColorBase0 = 21,
	TermColorBase1 = 22,
	TermColorBase2 = 23,
	TermColorBase3 = 24
};

enum TextStyleBits {
	TextStyleFgShift = 0,
	TextStyleBgShift = 5,
	TextStyleColorMask = 0x001F,
	TextStyleBold = 0x0400,
	TextStyleUnderline = 0x0800,
	TextStyleReverse = 0x1000,
};

// Packed fg/bg/attrs (jem.go/buffer.TextStyle as uint16 bits).
struct TextStyle {
	uint16_t bits;
};

TextStyle TextStyleDefault = {
	.bits = (uint16_t)(((uint16_t)TermColorDefault <<
		(uint16_t)TextStyleFgShift) | ((uint16_t)TermColorDefault <<
		(uint16_t)TextStyleBgShift)),
};

// text_style_make packs fg, bg, and flag bits into a TextStyle.
TextStyle text_style_make(TermColor fg, TermColor bg, uint16_t flags) {
	TextStyle s = {0};
	s.bits = (uint16_t)((((uint16_t)fg) & (uint16_t)TextStyleColorMask) <<
		(uint16_t)TextStyleFgShift |
		(((uint16_t)bg) & (uint16_t)TextStyleColorMask) <<
		(uint16_t)TextStyleBgShift |
		flags);
	return s;
}

// text_style_fg returns the foreground TermColor.
TermColor text_style_fg(TextStyle style) {
	return (TermColor)((style.bits >> (uint16_t)TextStyleFgShift) &
		(uint16_t)TextStyleColorMask);
}

// text_style_bg returns the background TermColor.
TermColor text_style_bg(TextStyle style) {
	return (TermColor)((style.bits >> (uint16_t)TextStyleBgShift) &
		(uint16_t)TextStyleColorMask);
}

// text_style_eq reports equal packed bits.
bool text_style_eq(TextStyle a, TextStyle b) {
	return a.bits == b.bits;
}

// Language mode ids (jem.go/buffer.LangMode).
enum LangMode {
	LModeNone = 0,
	LModeC = 1,
	LModeJava = 2,
	LModePython = 3,
	LModeLua = 4,
	LModeLisp = 5,
	LModeMarkdown = 6,
	LModePascal = 7,
	LModeVerilog = 8,
	LModeMake = 9,
	LModeSwift = 10,
	LModeJavaScript = 11,
	LModeActionScript = 12,
	LModeTypeScript = 13,
	LModeDart = 14,
	LModeGo = 15,
	LModeCSharp = 16,
	LModeRust = 17,
	LModeR = 18,
	LModeKotlin = 19,
	LModeHTML = 20,
	LModeCSS = 21,
	LModeModC = 22
};

// DFA state indices (jem.go/syntax SynState*).
enum SynDFA {
	SynStateNormal = 0,
	SynStateIdent = 1,
	SynStateNumber = 2,
	SynStateStringD = 3,
	SynStateStringDEsc = 4,
	SynStateStringS = 5,
	SynStateStringSEsc = 6,
	SynStateCmtLine = 7,
	SynStateCmtBlock = 8,
	SynStateCmtStar = 9,
	SynStateCmtBrace = 10,
	SynStateCmtParen = 11,
	SynStateCmtParen2 = 12,
	SynStatePreproc = 13,
	SynStateLuaDash = 14,
	SynStateLuaBlock = 15,
	SynStateLuaBlkEnd = 16,
	SynStateHTMLCmt = 17,
	SynStateHTMLCmtD1 = 18,
	SynStateHTMLCmtD2 = 19,
	SynStateOperator = 20,
	SynStateCount = 21
};

// Cross-line DFA carry (jem.go/buffer.SynState).
struct SynState {
	SynDFA dfa;
	uint8_t paren;
	uint8_t bracket;
	uint8_t curly;
};

// Highlighter entry kind.
enum ModeSyntaxKind {
	ModeSyntaxGeneral = 0,
	ModeSyntaxNone = 1,
	ModeSyntaxHashCommentOnly = 2,
	ModeSyntaxMarkdown = 3,
	ModeSyntaxHTML = 4
};

// Mode flag bits used by the DFA.
enum ModeFlag {
	ModeFlagIdentDash = 1 << 0,
	ModeFlagIdentLispExtra = 1 << 1,
	ModeFlagIdentLispSigil = 1 << 2,
	ModeFlagCommentSlashLine = 1 << 3,
	ModeFlagCommentSlashBlock = 1 << 4,
	ModeFlagPreprocHashAtBOL = 1 << 5,
	ModeFlagCommentHash = 1 << 6,
	ModeFlagCommentSemi = 1 << 7,
	ModeFlagCommentLua = 1 << 8,
	ModeFlagCommentPascalBrace = 1 << 9,
	ModeFlagCommentPascalParen = 1 << 10,
	ModeFlagAtRule = 1 << 11,
	ModeFlagNoCurlyRainbow = 1 << 12,
};

// Spec is highlighter metadata for a LangMode.
struct Spec {
	ModeSyntaxKind kind;
	uint32_t flags;
};

// syntax_spec_for returns kind/flags for mode (unknown → text fallback).
Spec syntax_spec_for(LangMode mode) {
	Spec s = {ModeSyntaxGeneral};
	switch (mode) {
	case LModeNone:
		s.kind = ModeSyntaxNone;
		s.flags = (uint32_t)ModeFlagCommentSlashLine | (uint32_t)ModeFlagCommentSlashBlock;
		break;
	case LModeC:
	case LModeModC:
		s.kind = ModeSyntaxGeneral;
		s.flags = (uint32_t)ModeFlagCommentSlashLine | (uint32_t)ModeFlagCommentSlashBlock |
			(uint32_t)ModeFlagPreprocHashAtBOL;
		break;
	case LModeJava:
	case LModeJavaScript:
	case LModeActionScript:
	case LModeTypeScript:
	case LModeDart:
	case LModeGo:
	case LModeRust:
	case LModeKotlin:
	case LModeVerilog:
		s.kind = ModeSyntaxGeneral;
		s.flags = (uint32_t)ModeFlagCommentSlashLine | (uint32_t)ModeFlagCommentSlashBlock;
		break;
	case LModeCSharp:
	case LModeSwift:
		s.kind = ModeSyntaxGeneral;
		s.flags = (uint32_t)ModeFlagCommentSlashLine | (uint32_t)ModeFlagCommentSlashBlock |
			(uint32_t)ModeFlagPreprocHashAtBOL;
		break;
	case LModePython:
	case LModeR:
		s.kind = ModeSyntaxGeneral;
		s.flags = ModeFlagCommentHash;
		break;
	case LModeMake:
		s.kind = ModeSyntaxHashCommentOnly;
		s.flags = ModeFlagCommentHash;
		break;
	case LModeLua:
		s.kind = ModeSyntaxGeneral;
		s.flags = ModeFlagCommentLua;
		break;
	case LModeLisp:
		s.kind = ModeSyntaxGeneral;
		s.flags = (uint32_t)ModeFlagIdentLispExtra | (uint32_t)ModeFlagIdentLispSigil |
			(uint32_t)ModeFlagCommentSemi;
		break;
	case LModePascal:
		s.kind = ModeSyntaxGeneral;
		s.flags = (uint32_t)ModeFlagCommentPascalBrace | (uint32_t)ModeFlagCommentPascalParen |
			(uint32_t)ModeFlagNoCurlyRainbow;
		break;
	case LModeCSS:
		s.kind = ModeSyntaxGeneral;
		s.flags = (uint32_t)ModeFlagIdentDash | (uint32_t)ModeFlagCommentSlashBlock |
			(uint32_t)ModeFlagAtRule;
		break;
	case LModeHTML:
		s.kind = ModeSyntaxHTML;
		s.flags = 0;
		break;
	case LModeMarkdown:
		s.kind = ModeSyntaxMarkdown;
		s.flags = 0;
		break;
	default:
		s.kind = ModeSyntaxNone;
		s.flags = (uint32_t)ModeFlagCommentSlashLine | (uint32_t)ModeFlagCommentSlashBlock;
		break;
	}
	return s;
}

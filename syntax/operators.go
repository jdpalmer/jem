package syntax

import "github.com/jdpalmer/jem/buffer"

var operatorsByLang map[buffer.LangMode]map[string]bool

func mergeOps(lists ...[]string) map[string]bool {
	m := make(map[string]bool)
	for _, list := range lists {
		for _, s := range list {
			m[s] = true
		}
	}
	return m
}

// Shared by C, C++, Java, and similar brace languages (without ++/--).
var cFamilyOperators = []string{
	"+", "-", "*", "/", "%",
	"!", "~",
	"&", "|", "^",
	"=", "==", "!=", "<", ">", "<=", ">=",
	"&&", "||",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
	"<<", ">>", "<<=", ">>=",
	"->", ".", "?",
}

var incDecOperators = []string{"++", "--"}

var goOperators = []string{
	":=", "<-", "...",
}

var javaOperators = []string{
	"::",
}

var csOperators = []string{
	"::", "=>", "??", "?.", "??=",
}

var jsOperators = []string{
	"===", "!==", "=>", "??", "?.", "??=", "**", "**=",
}

var swiftOperators = []string{
	"??", "?.", "!", "...", "..<", "->", "::",
	"&+", "&-", "&*", "&+=",
}

var rustOperators = []string{
	"::", "->", "=>", "..", "..=", "?",
}

var pyOperators = []string{
	"**", "//", ":=", "@",
	"//=", "**=",
}

var luaOperators = []string{
	"^", "~=", "..", "#",
}

var lispOperators = []string{
	"'", ",", "@", "`", "#", ";",
}

var pasOperators = []string{
	":=", "<>", "^", "@",
}

var vlgOperators = []string{
	"===", "!==", "<=>", "<<<", ">>>",
	"<=", ">=", "->", "@", "#", "?:",
	"&=", "|=", "^=", "+=", "-=", "*=", "/=", "%=",
}

var rOperators = []string{
	"<-", "->", "<<-", "->>",
	"|>", "%>%", ":=", "::", "$",
	"%%", "%*%", "%/%", "%in%",
}

var ktOperators = []string{
	"::", "?.", "!!", "?:", "..", "->", "=>",
}

var dartOperators = []string{
	"??", "?.", "??=", "=>", "...",
}

var cssOperators = []string{
	":", ";", ">", "+", "~", "=", ",",
}

func initOperatorsByLang() {
	operatorsByLang = map[buffer.LangMode]map[string]bool{
		buffer.LModeC:            mergeOps(cFamilyOperators, incDecOperators),
		buffer.LModeJava:         mergeOps(cFamilyOperators, incDecOperators, javaOperators),
		buffer.LModeCSharp:       mergeOps(cFamilyOperators, incDecOperators, csOperators),
		buffer.LModeGo:           mergeOps(cFamilyOperators, incDecOperators, goOperators),
		buffer.LModeJavaScript:   mergeOps(cFamilyOperators, incDecOperators, jsOperators),
		buffer.LModeTypeScript:   mergeOps(cFamilyOperators, incDecOperators, jsOperators),
		buffer.LModeActionScript: mergeOps(cFamilyOperators, incDecOperators, jsOperators),
		buffer.LModeDart:         mergeOps(cFamilyOperators, incDecOperators, dartOperators),
		buffer.LModeSwift:        mergeOps(cFamilyOperators, incDecOperators, swiftOperators),
		buffer.LModeRust:         mergeOps(cFamilyOperators, rustOperators),
		buffer.LModeKotlin:       mergeOps(cFamilyOperators, ktOperators),
		buffer.LModePython:       mergeOps(cFamilyOperators, pyOperators),
		buffer.LModeLua:          mergeOps(cFamilyOperators, luaOperators),
		buffer.LModeLisp:         mergeOps(cFamilyOperators, lispOperators),
		buffer.LModePascal:       mergeOps(cFamilyOperators, pasOperators),
		buffer.LModeVerilog:      mergeOps(vlgOperators),
		buffer.LModeR:            mergeOps(cFamilyOperators, rOperators),
		buffer.LModeCSS:          mergeOps(cssOperators),
	}
}

func operatorStyleForLang(lang buffer.LangMode, op string) buffer.TextStyle {
	ops, ok := operatorsByLang[lang]
	if !ok || !ops[op] {
		return buffer.TextStyleDefault
	}
	return keywordStyle
}

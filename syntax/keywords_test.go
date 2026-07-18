package syntax

import (
	"testing"

	"github.com/jdpalmer/jem/buffer"
)

func TestIdentColorForLang(t *testing.T) {
	tests := []struct {
		name  string
		lang  buffer.LangMode
		ident string
		want  buffer.TextStyle
	}{
		{name: "java keyword", lang: buffer.LModeJava, ident: "assert", want: keywordStyle},
		{name: "java rejects c keyword", lang: buffer.LModeJava, ident: "sizeof", want: buffer.TextStyleDefault},
		{name: "java type", lang: buffer.LModeJava, ident: "boolean", want: typeStyle},
		{name: "csharp keyword", lang: buffer.LModeCSharp, ident: "async", want: keywordStyle},
		{name: "csharp rejects c type", lang: buffer.LModeCSharp, ident: "mutable", want: buffer.TextStyleDefault},
		{name: "csharp keyword delegate", lang: buffer.LModeCSharp, ident: "delegate", want: keywordStyle},
		{name: "plain text mode", lang: buffer.LModeNone, ident: "if", want: buffer.TextStyleDefault},
		{name: "markdown mode", lang: buffer.LModeMarkdown, ident: "if", want: buffer.TextStyleDefault},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ident_color_for_lang(tc.lang, tc.ident); got != tc.want {
				t.Fatalf("ident_color_for_lang(%v, %q) = %v, want %v", tc.lang, tc.ident, got, tc.want)
			}
		})
	}
}

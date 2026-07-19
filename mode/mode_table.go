package mode

import (
	"github.com/jdpalmer/jem/model"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/syntax"
)

var modeTable = []model.ModeInfo{
	{
		Mode:              buffer.LModeNone,
		DisplayName:       "Text",
		CompletionName:    "text",
		CommentOpen:       "/*",
		CommentAltOpen:    "//",
		CommentAppend:     "  /* */",
		CommentCursorBack: 3,
	},
	{
		Mode:              buffer.LModeC,
		DisplayName:       "C",
		CompletionName:    "c",
		CommentOpen:       "/*",
		CommentAltOpen:    "//",
		CommentAppend:     "  /* */",
		CommentCursorBack: 3,
	},
	{
		Mode:           buffer.LModeJava,
		DisplayName:    "Java",
		CompletionName: "java",
		CommentOpen:    "/*",
		CommentAltOpen: "//",
	},
	{
		Mode:           buffer.LModePython,
		DisplayName:    "Python",
		CompletionName: "python",
		CommentOpen:    "#",
		CommentAppend:  "  # ",
	},
	{
		Mode:           buffer.LModeLua,
		DisplayName:    "Lua",
		CompletionName: "text",
		CommentOpen:    "--",
		CommentAppend:  "  -- ",
	},
	{
		Mode:           buffer.LModeLisp,
		DisplayName:    "Lisp",
		CompletionName: "text",
		CommentOpen:    ";",
		CommentAppend:  "  ; ",
	},
	{
		Mode:              buffer.LModeMarkdown,
		DisplayName:       "Markdown",
		CompletionName:    "text",
		CommentOpen:       "<!--",
		CommentAppend:     "  <!--  -->",
		CommentCursorBack: 4,
	},
	{
		Mode:              buffer.LModePascal,
		DisplayName:       "Pascal",
		CompletionName:    "text",
		CommentOpen:       "{",
		CommentAppend:     "  {  }",
		CommentCursorBack: 2,
	},
	{
		Mode:           buffer.LModeVerilog,
		DisplayName:    "Verilog",
		CompletionName: "text",
		CommentOpen:    "/*",
		CommentAltOpen: "//",
	},
	{
		Mode:           buffer.LModeMake,
		DisplayName:    "Make",
		CompletionName: "text",
		CommentOpen:    "#",
		CommentAppend:  "  # ",
	},
	{
		Mode:           buffer.LModeSwift,
		DisplayName:    "Swift",
		CompletionName: "swift",
		CommentOpen:    "/*",
		CommentAltOpen: "//",
	},
	{
		Mode:           buffer.LModeJavaScript,
		DisplayName:    "JavaScript",
		CompletionName: "javascript",
		CommentOpen:    "/*",
		CommentAltOpen: "//",
	},
	{
		Mode:           buffer.LModeActionScript,
		DisplayName:    "ActionScript",
		CompletionName: "javascript",
		CommentOpen:    "/*",
		CommentAltOpen: "//",
	},
	{
		Mode:           buffer.LModeTypeScript,
		DisplayName:    "TypeScript",
		CompletionName: "typescript",
		CommentOpen:    "/*",
		CommentAltOpen: "//",
	},
	{
		Mode:           buffer.LModeDart,
		DisplayName:    "Dart",
		CompletionName: "dart",
		CommentOpen:    "/*",
		CommentAltOpen: "//",
	},
	{
		Mode:           buffer.LModeGo,
		DisplayName:    "Go",
		CompletionName: "go",
		CommentOpen:    "//",
		CommentAltOpen: "//",
		CommentAppend:  "  // ",
	},
	{
		Mode:           buffer.LModeCSharp,
		DisplayName:    "C#",
		CompletionName: "csharp",
		CommentOpen:    "/*",
		CommentAltOpen: "//",
	},
	{
		Mode:           buffer.LModeRust,
		DisplayName:    "Rust",
		CompletionName: "rust",
		CommentOpen:    "/*",
		CommentAltOpen: "//",
	},
	{
		Mode:           buffer.LModeR,
		DisplayName:    "R",
		CompletionName: "r",
		CommentOpen:    "#",
		CommentAppend:  "  # ",
	},
	{
		Mode:           buffer.LModeKotlin,
		DisplayName:    "Kotlin",
		CompletionName: "kotlin",
		CommentOpen:    "/*",
		CommentAltOpen: "//",
	},
	{
		Mode:              buffer.LModeHTML,
		DisplayName:       "HTML/XML",
		CompletionName:    "html",
		CommentOpen:       "<!--",
		CommentAppend:     "  <!--  -->",
		CommentCursorBack: 4,
	},
	{
		Mode:              buffer.LModeCSS,
		DisplayName:       "CSS",
		CompletionName:    "css",
		CommentOpen:       "/*",
		CommentAppend:     "  /* */",
		CommentCursorBack: 3,
	},
}

func init() {
	for i := range modeTable {
		s := syntax.For(modeTable[i].Mode)
		modeTable[i].SyntaxKind = s.Kind
		modeTable[i].SyntaxFlags = s.Flags
	}
}

func LangModeInfo(mode buffer.LangMode) *model.ModeInfo {
	for i := range modeTable {
		if modeTable[i].Mode == mode {
			return &modeTable[i]
		}
	}
	return &modeTable[0]
}

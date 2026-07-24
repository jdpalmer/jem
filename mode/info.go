package mode

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/syntax"
)

type ModeWordAttr int

const (
	ModeWordAttrNone ModeWordAttr = iota
	ModeWordAttrType
	ModeWordAttrBuiltin
)

type ModeMiscIndentKind int

const (
	ModeMiscIndentNone ModeMiscIndentKind = iota
	ModeMiscIndentMake
	ModeMiscIndentLua
	ModeMiscIndentPascal
	ModeMiscIndentVerilog
	ModeMiscIndentR
	ModeMiscIndentHTML
	ModeMiscIndentLisp
)

type ModeInfo struct {
	Mode              buffer.LangMode
	DisplayName       string
	CompletionName    string
	SyntaxKind        syntax.ModeSyntaxKind
	SyntaxFlags       uint32
	MiscIndentKind    ModeMiscIndentKind
	IndentDefault     buffer.IndentConfig
	CommentOpen       string
	CommentAltOpen    string
	CommentAppend     string
	CommentCursorBack int
	NewlineAndIndent  func(f bool, n int) bool
	IndentLine        func(f bool, n int) bool
	CloseBrace        func(f bool, n int) bool
	GotoMatch         func(f bool, n int) bool
	MakeComment       func(f bool, n int) bool
	TopOfFunction     func(f bool, n int) bool
	EndOfFunction     func(f bool, n int) bool
	MarkFunction      func(f bool, n int) bool
	Extensions        []string
	ExtensionCount    uint8
	Basenames         []string
	BasenameCount     uint8
}

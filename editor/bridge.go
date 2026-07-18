package editor

import (
	"github.com/jdpalmer/jem/app"
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/edit"
	"github.com/jdpalmer/jem/syntax"
)

func bufferSetText(bp *buffer.Buffer, begin, end buffer.Location, newText []byte, newEndOut *buffer.Location, kill bool) bool {
	if kill {
		oldText := bp.GetText(begin, end)
		if len(oldText) > 0 && !edit.KillAppend(oldText) {
			return false
		}
	}
	err := edit.SetText(bp, begin, end, newText, newEndOut)
	if err != nil {
		return false
	}
	if kill {
		edit.KillWriteClipboard()
	}
	return true
}

func syncSyntaxPalette() {
	syntax.PackagePalette = syntax.Palette{
		NormalStyle:  app.State.Theme.NormalStyle,
		CommentStyle: app.State.Theme.CommentStyle,
	}
}

var SyntaxDebug = syntax.SyntaxDebug

const (
	LModeNone         buffer.LangMode = buffer.LModeNone
	LModeC            buffer.LangMode = buffer.LModeC
	LModeJava         buffer.LangMode = buffer.LModeJava
	LModePython       buffer.LangMode = buffer.LModePython
	LModeLua          buffer.LangMode = buffer.LModeLua
	LModeLisp         buffer.LangMode = buffer.LModeLisp
	LModeMarkdown     buffer.LangMode = buffer.LModeMarkdown
	LModePascal       buffer.LangMode = buffer.LModePascal
	LModeVerilog      buffer.LangMode = buffer.LModeVerilog
	LModeMake         buffer.LangMode = buffer.LModeMake
	LModeSwift        buffer.LangMode = buffer.LModeSwift
	LModeJavaScript   buffer.LangMode = buffer.LModeJavaScript
	LModeActionScript buffer.LangMode = buffer.LModeActionScript
	LModeTypeScript   buffer.LangMode = buffer.LModeTypeScript
	LModeDart         buffer.LangMode = buffer.LModeDart
	LModeGo           buffer.LangMode = buffer.LModeGo
	LModeCSharp       buffer.LangMode = buffer.LModeCSharp
	LModeRust         buffer.LangMode = buffer.LModeRust
	LModeR            buffer.LangMode = buffer.LModeR
	LModeKotlin       buffer.LangMode = buffer.LModeKotlin
	LModeHTML         buffer.LangMode = buffer.LModeHTML
	LModeCSS          buffer.LangMode = buffer.LModeCSS
)

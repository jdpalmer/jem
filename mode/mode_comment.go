package mode

import (
	"github.com/jdpalmer/jem/buffer"
	"github.com/jdpalmer/jem/syntax"
	"github.com/jdpalmer/jem/window"
)

func modeCommentLinePrefix(info *ModeInfo) []byte {
	if info == nil {
		return nil
	}
	if info.CommentAltOpen != "" {
		return []byte(info.CommentAltOpen)
	}
	if info.CommentOpen != "" {
		flags := info.SyntaxFlags
		if (flags&(syntax.ModeFlagCommentHash|syntax.ModeFlagCommentSemi|syntax.ModeFlagCommentLua)) != 0 || len(info.CommentOpen) == 1 {
			return []byte(info.CommentOpen)
		}
	}
	return nil
}

func lineHasCommentPrefix(line *buffer.Line, prefix []byte) bool {
	if line == nil || len(prefix) == 0 {
		return false
	}
	pos := line.FirstNonblank()
	if line.Len() < pos+len(prefix) {
		return false
	}
	for k := range prefix {
		if line.Byte(pos+k) != prefix[k] {
			return false
		}
	}
	return true
}

func modeSupportsComments(info *ModeInfo) bool {
	if info == nil {
		return false
	}
	return info.CommentOpen != "" || info.CommentAppend != ""
}

func modeToggleCommentRegion(win *window.Window, buf *buffer.Buffer, info *ModeInfo, linePrefix []byte, startLine, endLine int) bool {
	prefixLen := len(linePrefix)
	allCommented := true
	for lineNum := startLine; lineNum <= endLine; lineNum++ {
		line := buf.Line(lineNum)
		if !lineHasCommentPrefix(line, linePrefix) {
			allCommented = false
			break
		}
	}
	if allCommented {
		PackageHooks.BeginCommand()
		savedCursor := win.Cursor
		savedMark := win.Mark
		for lineNum := startLine; lineNum <= endLine; lineNum++ {
			line := buf.Line(lineNum)
			if line == nil {
				continue
			}
			pos := line.FirstNonblank()
			b := buffer.MakeLocation(lineNum, pos)
			e := buffer.MakeLocation(lineNum, pos+prefixLen)
			if err := PackageHooks.SetText(buf, b, e, nil, nil); err != nil {
				win.Cursor = savedCursor
				win.Mark = savedMark
				PackageHooks.EndCommand()
				return false
			}
			if savedCursor.Line == lineNum {
				if savedCursor.Offset >= e.Offset {
					savedCursor.Offset -= prefixLen
				} else if savedCursor.Offset >= b.Offset {
					savedCursor.Offset = b.Offset
				}
			}
			if savedMark.Line == lineNum {
				if savedMark.Offset >= e.Offset {
					savedMark.Offset -= prefixLen
				} else if savedMark.Offset >= b.Offset {
					savedMark.Offset = b.Offset
				}
			}
		}
		PackageHooks.EndCommand()
		win.Cursor = savedCursor
		win.Mark = savedMark
		win.DidEdit = true
		win.DidMove = true
		return true
	}

	PackageHooks.BeginCommand()
	ok := ModeDispatch(info.MakeComment, false, 1)
	PackageHooks.EndCommand()
	return ok
}

func CmdModeToggleComment(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil {
		return false
	}
	info := LangModeInfo(buf.LangMode)
	if !modeSupportsComments(info) {
		return false
	}
	linePrefix := modeCommentLinePrefix(info)

	if win.Mark.Line != 0 && win.Mark.Line != win.Cursor.Line {
		startLine := win.Mark.Line
		endLine := win.Cursor.Line
		if startLine > endLine {
			startLine, endLine = endLine, startLine
		}
		if linePrefix != nil {
			return modeToggleCommentRegion(win, buf, info, linePrefix, startLine, endLine)
		}
		PackageHooks.BeginCommand()
		ok := ModeDispatch(info.MakeComment, false, 1)
		PackageHooks.EndCommand()
		return ok
	}

	if linePrefix != nil {
		line := buf.Line(win.Cursor.Line)
		if lineHasCommentPrefix(line, linePrefix) {
			pos := line.FirstNonblank()
			prefixLen := len(linePrefix)
			PackageHooks.BeginCommand()
			savedCursor := win.Cursor
			savedMark := win.Mark
			b := buffer.MakeLocation(win.Cursor.Line, pos)
			e := buffer.MakeLocation(win.Cursor.Line, pos+prefixLen)
			if err := PackageHooks.SetText(buf, b, e, nil, nil); err != nil {
				win.Cursor = savedCursor
				win.Mark = savedMark
				PackageHooks.EndCommand()
				return false
			}
			if savedCursor.Line == b.Line {
				if savedCursor.Offset >= e.Offset {
					savedCursor.Offset -= prefixLen
				} else if savedCursor.Offset >= b.Offset {
					savedCursor.Offset = b.Offset
				}
			}
			if savedMark.Line == b.Line {
				if savedMark.Offset >= e.Offset {
					savedMark.Offset -= prefixLen
				} else if savedMark.Offset >= b.Offset {
					savedMark.Offset = b.Offset
				}
			}
			PackageHooks.EndCommand()
			win.Cursor = savedCursor
			win.Mark = savedMark
			win.DidEdit = true
			win.DidMove = true
			return true
		}
		PackageHooks.BeginCommand()
		ok := ModeDispatch(info.MakeComment, false, 1)
		PackageHooks.EndCommand()
		return ok
	}

	PackageHooks.BeginCommand()
	ok := ModeDispatch(info.MakeComment, false, 1)
	PackageHooks.EndCommand()
	return ok
}

func CmdCommentDwim(f bool, n int) bool {
	_ = f
	_ = n
	win := window.Active.CurrentWindow
	buf := buffer.All.Current
	if win == nil || buf == nil {
		return false
	}
	info := LangModeInfo(buf.LangMode)
	if !modeSupportsComments(info) {
		return false
	}
	if win.Mark.Line != 0 && win.Mark.Line != win.Cursor.Line {
		return CmdModeToggleComment(false, 1)
	}
	linePrefix := modeCommentLinePrefix(info)
	line := buf.Line(win.Cursor.Line)
	if linePrefix != nil && line != nil && lineHasCommentPrefix(line, linePrefix) {
		return CmdModeToggleComment(false, 1)
	}
	return CmdModeMakeComment(false, 1)
}

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

func lineHasCommentPrefix(lp *buffer.Line, prefix []byte) bool {
	if lp == nil || len(prefix) == 0 {
		return false
	}
	pos := lp.FirstNonblank()
	if lp.Len() < pos+uint(len(prefix)) {
		return false
	}
	for k := range prefix {
		if lp.Byte(pos+uint(k)) != prefix[k] {
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

func modeToggleCommentRegion(wp *window.Window, bp *buffer.Buffer, info *ModeInfo, linePrefix []byte, startLine, endLine uint) bool {
	prefixLen := len(linePrefix)
	allCommented := true
	for line := startLine; line <= endLine; line++ {
		lp := bp.Line(line)
		if !lineHasCommentPrefix(lp, linePrefix) {
			allCommented = false
			break
		}
	}
	if allCommented {
		PackageHooks.BeginCommand()
		savedCursor := wp.Cursor
		savedMark := wp.Mark
		for line := startLine; line <= endLine; line++ {
			lp := bp.Line(line)
			if lp == nil {
				continue
			}
			pos := lp.FirstNonblank()
			b := buffer.MakeLocation(line, pos)
			e := buffer.MakeLocation(line, pos+uint(prefixLen))
			if err := PackageHooks.SetText(bp, b, e, nil, nil); err != nil {
				wp.Cursor = savedCursor
				wp.Mark = savedMark
				PackageHooks.EndCommand()
				return false
			}
			if savedCursor.Line == line {
				if savedCursor.Offset >= e.Offset {
					savedCursor.Offset -= uint(prefixLen)
				} else if savedCursor.Offset >= b.Offset {
					savedCursor.Offset = b.Offset
				}
			}
			if savedMark.Line == line {
				if savedMark.Offset >= e.Offset {
					savedMark.Offset -= uint(prefixLen)
				} else if savedMark.Offset >= b.Offset {
					savedMark.Offset = b.Offset
				}
			}
		}
		PackageHooks.EndCommand()
		wp.Cursor = savedCursor
		wp.Mark = savedMark
		wp.DidEdit = true
		wp.DidMove = true
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
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil {
		return false
	}
	info := LangModeInfo(bp.LangMode)
	if !modeSupportsComments(info) {
		return false
	}
	linePrefix := modeCommentLinePrefix(info)

	if wp.Mark.Line != 0 && wp.Mark.Line != wp.Cursor.Line {
		startLine := wp.Mark.Line
		endLine := wp.Cursor.Line
		if startLine > endLine {
			startLine, endLine = endLine, startLine
		}
		if linePrefix != nil {
			return modeToggleCommentRegion(wp, bp, info, linePrefix, startLine, endLine)
		}
		PackageHooks.BeginCommand()
		ok := ModeDispatch(info.MakeComment, false, 1)
		PackageHooks.EndCommand()
		return ok
	}

	if linePrefix != nil {
		lp := bp.Line(wp.Cursor.Line)
		if lineHasCommentPrefix(lp, linePrefix) {
			pos := lp.FirstNonblank()
			prefixLen := len(linePrefix)
			PackageHooks.BeginCommand()
			savedCursor := wp.Cursor
			savedMark := wp.Mark
			b := buffer.MakeLocation(wp.Cursor.Line, pos)
			e := buffer.MakeLocation(wp.Cursor.Line, pos+uint(prefixLen))
			if err := PackageHooks.SetText(bp, b, e, nil, nil); err != nil {
				wp.Cursor = savedCursor
				wp.Mark = savedMark
				PackageHooks.EndCommand()
				return false
			}
			if savedCursor.Line == b.Line {
				if savedCursor.Offset >= e.Offset {
					savedCursor.Offset -= uint(prefixLen)
				} else if savedCursor.Offset >= b.Offset {
					savedCursor.Offset = b.Offset
				}
			}
			if savedMark.Line == b.Line {
				if savedMark.Offset >= e.Offset {
					savedMark.Offset -= uint(prefixLen)
				} else if savedMark.Offset >= b.Offset {
					savedMark.Offset = b.Offset
				}
			}
			PackageHooks.EndCommand()
			wp.Cursor = savedCursor
			wp.Mark = savedMark
			wp.DidEdit = true
			wp.DidMove = true
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
	wp := window.Active.CurrentWindow
	bp := buffer.All.Current
	if wp == nil || bp == nil {
		return false
	}
	info := LangModeInfo(bp.LangMode)
	if !modeSupportsComments(info) {
		return false
	}
	if wp.Mark.Line != 0 && wp.Mark.Line != wp.Cursor.Line {
		return CmdModeToggleComment(false, 1)
	}
	linePrefix := modeCommentLinePrefix(info)
	lp := bp.Line(wp.Cursor.Line)
	if linePrefix != nil && lp != nil && lineHasCommentPrefix(lp, linePrefix) {
		return CmdModeToggleComment(false, 1)
	}
	return CmdModeMakeComment(false, 1)
}

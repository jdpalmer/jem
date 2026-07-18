package modes

import "github.com/jdpalmer/jem/app"

func modeCommentLinePrefix(info *app.ModeInfo) []byte {
	if info == nil {
		return nil
	}
	if info.CommentAltOpen != "" {
		return []byte(info.CommentAltOpen)
	}
	if info.CommentOpen != "" {
		flags := info.SyntaxFlags
		if (flags&(app.ModeFlagCommentHash|app.ModeFlagCommentSemi|app.ModeFlagCommentLua)) != 0 || len(info.CommentOpen) == 1 {
			return []byte(info.CommentOpen)
		}
	}
	return nil
}

func lineHasCommentPrefix(lp *Line, prefix []byte) bool {
	if lp == nil || len(prefix) == 0 {
		return false
	}
	pos := line_first_nonblank(lp)
	if LineLength(lp) < pos+uint(len(prefix)) {
		return false
	}
	for k := range prefix {
		if LineGetc(lp, pos+uint(k)) != prefix[k] {
			return false
		}
	}
	return true
}

func modeSupportsComments(info *app.ModeInfo) bool {
	if info == nil {
		return false
	}
	return info.CommentOpen != "" || info.CommentAppend != ""
}

func modeToggleCommentRegion(wp *Window, bp *Buffer, info *app.ModeInfo, linePrefix []byte, startLine, endLine uint) bool {
	if PackageHooks.BufferSetText == nil {
		return false
	}
	prefixLen := len(linePrefix)
	allCommented := true
	for line := startLine; line <= endLine; line++ {
		lp := BufferGetLine(bp, line)
		if !lineHasCommentPrefix(lp, linePrefix) {
			allCommented = false
			break
		}
	}
	if allCommented {
		if PackageHooks.UndoBeginCommand != nil {
			PackageHooks.UndoBeginCommand()
		}
		savedCursor := wp.Cursor
		savedMark := wp.Mark
		for line := startLine; line <= endLine; line++ {
			lp := BufferGetLine(bp, line)
			if lp == nil {
				continue
			}
			pos := line_first_nonblank(lp)
			b := MakeLocation(line, pos)
			e := MakeLocation(line, pos+uint(prefixLen))
			if !PackageHooks.BufferSetText(bp, b, e, nil, 0, nil, false) {
				wp.Cursor = savedCursor
				wp.Mark = savedMark
				if PackageHooks.UndoEndCommand != nil {
					PackageHooks.UndoEndCommand()
				}
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
		if PackageHooks.UndoEndCommand != nil {
			PackageHooks.UndoEndCommand()
		}
		wp.Cursor = savedCursor
		wp.Mark = savedMark
		wp.DidEdit = true
		wp.DidMove = true
		return true
	}

	if PackageHooks.UndoBeginCommand != nil {
		PackageHooks.UndoBeginCommand()
	}
	ok := ModeDispatch(info.MakeComment, false, 1)
	if PackageHooks.UndoEndCommand != nil {
		PackageHooks.UndoEndCommand()
	}
	return ok
}

func CmdModeToggleComment(f bool, n int) bool {
	_ = f
	_ = n
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
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
		if PackageHooks.UndoBeginCommand != nil {
			PackageHooks.UndoBeginCommand()
		}
		ok := ModeDispatch(info.MakeComment, false, 1)
		if PackageHooks.UndoEndCommand != nil {
			PackageHooks.UndoEndCommand()
		}
		return ok
	}

	if linePrefix != nil {
		lp := BufferGetLine(bp, wp.Cursor.Line)
		if lineHasCommentPrefix(lp, linePrefix) {
			pos := line_first_nonblank(lp)
			prefixLen := len(linePrefix)
			if PackageHooks.UndoBeginCommand != nil {
				PackageHooks.UndoBeginCommand()
			}
			savedCursor := wp.Cursor
			savedMark := wp.Mark
			b := MakeLocation(wp.Cursor.Line, pos)
			e := MakeLocation(wp.Cursor.Line, pos+uint(prefixLen))
			if PackageHooks.BufferSetText == nil || !PackageHooks.BufferSetText(bp, b, e, nil, 0, nil, false) {
				wp.Cursor = savedCursor
				wp.Mark = savedMark
				if PackageHooks.UndoEndCommand != nil {
					PackageHooks.UndoEndCommand()
				}
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
			if PackageHooks.UndoEndCommand != nil {
				PackageHooks.UndoEndCommand()
			}
			wp.Cursor = savedCursor
			wp.Mark = savedMark
			wp.DidEdit = true
			wp.DidMove = true
			return true
		}
		if PackageHooks.UndoBeginCommand != nil {
			PackageHooks.UndoBeginCommand()
		}
		ok := ModeDispatch(info.MakeComment, false, 1)
		if PackageHooks.UndoEndCommand != nil {
			PackageHooks.UndoEndCommand()
		}
		return ok
	}

	if PackageHooks.UndoBeginCommand != nil {
		PackageHooks.UndoBeginCommand()
	}
	ok := ModeDispatch(info.MakeComment, false, 1)
	if PackageHooks.UndoEndCommand != nil {
		PackageHooks.UndoEndCommand()
	}
	return ok
}

func CmdCommentDwim(f bool, n int) bool {
	_ = f
	_ = n
	wp := app.State.CurrentWindow
	bp := app.State.CurrentBuffer
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
	lp := BufferGetLine(bp, wp.Cursor.Line)
	if linePrefix != nil && lp != nil && lineHasCommentPrefix(lp, linePrefix) {
		return CmdModeToggleComment(false, 1)
	}
	return CmdModeMakeComment(false, 1)
}

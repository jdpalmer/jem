package editor

import sess "github.com/jdpalmer/jem/session"

const MarkCapacity = sess.MarkCapacity

type Mark = sess.Mark
type MarkState = sess.MarkState

var marksState = &sess.MarksState

func markPopOnce() bool {
	if !sess.MarkPopOnce() {
		mbWrite("[no saved mark]")
		return false
	}
	return true
}

// CmdMarkPush saves the current location on the mark stack.
func CmdMarkPush(f bool, n int) bool {
	_ = f
	_ = n
	sess.MarkPushCurrent()
	mbWrite("[mark pushed]")
	return true
}

// CmdMarkPop restores the most recently pushed mark.
func CmdMarkPop(f bool, n int) bool {
	_ = f
	_ = n
	return markPopOnce()
}
